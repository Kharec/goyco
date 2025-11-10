package services

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type EmailSender interface {
	Send(to, subject, body string) error
}

type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	timeout  time.Duration
}

func NewSMTPSender(cfgHost string, cfgPort int, cfgUsername, cfgPassword, cfgFrom string) *SMTPSender {
	return &SMTPSender{
		host:     cfgHost,
		port:     cfgPort,
		username: cfgUsername,
		password: cfgPassword,
		from:     cfgFrom,
		timeout:  30 * time.Second,
	}
}

func NewSMTPSenderWithTimeout(cfgHost string, cfgPort int, cfgUsername, cfgPassword, cfgFrom string, timeout time.Duration) *SMTPSender {
	return &SMTPSender{
		host:     cfgHost,
		port:     cfgPort,
		username: cfgUsername,
		password: cfgPassword,
		from:     cfgFrom,
		timeout:  timeout,
	}
}

func (s *SMTPSender) Send(to, subject, body string) error {
	if s == nil {
		return fmt.Errorf("smtp sender is not configured")
	}

	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("recipient address is required")
	}

	address := fmt.Sprintf("%s:%d", s.host, s.port)

	headers := []string{
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=\"UTF-8\"",
	}

	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n"

	var auth smtp.Auth
	if strings.TrimSpace(s.username) != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(address, auth, s.from, []string{to}, []byte(message))
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(s.timeout):
		return fmt.Errorf("email sending timeout after %v", s.timeout)
	}
}

func (s *SMTPSender) SendAsync(to, subject, body string) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- s.Send(to, subject, body)
	}()
	return result
}

func (s *SMTPSender) SetTimeout(timeout time.Duration) {
	if s == nil {
		return
	}

	s.timeout = timeout
}
