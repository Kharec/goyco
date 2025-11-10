package testutils

import (
	"fmt"
	"time"
)

type AsyncEmailSender interface {
	Send(to, subject, body string) error
	SendAsync(to, subject, body string) <-chan error
	SetTimeout(timeout time.Duration)
}

type TestSMTPClient struct {
	sender AsyncEmailSender
	server *TestEmailServer
}

func NewTestSMTPClient(factory func(port int) AsyncEmailSender) (*TestSMTPClient, error) {
	server, err := NewTestEmailServer()
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	sender := factory(server.GetPort())
	if sender == nil {
		server.Close()
		return nil, fmt.Errorf("smtp sender factory returned nil")
	}

	return &TestSMTPClient{
		sender: sender,
		server: server,
	}, nil
}

func (c *TestSMTPClient) Sender() AsyncEmailSender {
	return c.sender
}

func (c *TestSMTPClient) Server() *TestEmailServer {
	return c.server
}

func (c *TestSMTPClient) Close() error {
	if c == nil || c.server == nil {
		return nil
	}

	return c.server.Close()
}

func (c *TestSMTPClient) SendTestEmail(to, subject, body string) error {
	if c == nil || c.sender == nil {
		return fmt.Errorf("smtp sender not configured")
	}

	return c.sender.Send(to, subject, body)
}

func (c *TestSMTPClient) SendTestEmailAsync(to, subject, body string) <-chan error {
	if c == nil || c.sender == nil {
		result := make(chan error, 1)
		result <- fmt.Errorf("smtp sender not configured")
		close(result)
		return result
	}

	return c.sender.SendAsync(to, subject, body)
}

func (c *TestSMTPClient) WaitForEmail(timeout time.Duration) bool {
	if c == nil || c.server == nil {
		return false
	}

	return c.server.WaitForEmails(1, timeout)
}

func (c *TestSMTPClient) GetReceivedEmails() []TestEmail {
	if c == nil || c.server == nil {
		return nil
	}

	return c.server.GetEmails()
}

func (c *TestSMTPClient) ClearReceivedEmails() {
	if c == nil || c.server == nil {
		return
	}

	c.server.ClearEmails()
}

func (c *TestSMTPClient) SetTimeout(timeout time.Duration) {
	if c == nil || c.sender == nil {
		return
	}

	c.sender.SetTimeout(timeout)
}
