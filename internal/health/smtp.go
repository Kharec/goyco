package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

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
	address := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	result := Result{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC(),
		Details: map[string]any{
			"host": address,
		},
	}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
		result.Latency = time.Since(start)
		return result
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.Host)
	if err != nil {
		result.Status = StatusDegraded
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
