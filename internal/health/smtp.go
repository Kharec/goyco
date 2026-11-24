package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

const defaultSMTPCheckTimeout = 30 * time.Second

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPChecker struct {
	config SMTPConfig
}

func NewSMTPChecker(config SMTPConfig) *SMTPChecker {
	return &SMTPChecker{
		config: config,
	}
}

func (c *SMTPChecker) Name() string {
	return "smtp"
}

func (c *SMTPChecker) Check(ctx context.Context) Result {
	start := time.Now()
	address := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))

	result := Result{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC(),
		Details: map[string]any{
			"host": address,
		},
	}

	deadline := start.Add(defaultSMTPCheckTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}

	dialer := net.Dialer{Deadline: deadline}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
		result.Latency = time.Since(start)
		return result
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to set SMTP connection deadline: %v", err)
		result.Latency = time.Since(start)
		return result
	}

	stopCancellation := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stopCancellation()

	client, err := smtp.NewClient(conn, c.config.Host)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to create SMTP client: %v", err)
		result.Latency = time.Since(start)
		return result
	}
	defer client.Close()

	err = client.Hello("goyco-health-check")
	if err != nil {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("EHLO failed: %v", err)
		result.Latency = time.Since(start)
		return result
	}

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: c.config.Host,
		}
		err = client.StartTLS(tlsConfig)
		if err != nil {
			result.Details["starttls"] = "failed"
			result.Details["starttls_error"] = err.Error()
		} else {
			result.Details["starttls"] = "enabled"
		}
	} else {
		result.Details["starttls"] = "not supported"
	}

	if c.config.Username != "" && c.config.Password != "" {
		auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)
		err = client.Auth(auth)
		if err != nil {
			result.Details["auth"] = "failed"
			result.Details["auth_error"] = err.Error()
		} else {
			result.Details["auth"] = "success"
		}
	} else {
		result.Details["auth"] = "not configured"
	}

	client.Quit()

	result.Latency = time.Since(start)
	result.Details["handshake"] = "completed"

	return result
}
