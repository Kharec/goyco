package health

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSMTPChecker_Check_NoAuth(t *testing.T) {
	config := SMTPConfig{
		Host: "smtp.example.com",
		Port: 25,
	}

	checker := NewSMTPChecker(config)

	if checker.config.Username != "" {
		t.Error("expected empty username")
	}
	if checker.config.Password != "" {
		t.Error("expected empty password")
	}
}

func TestSMTPChecker_Check_InvalidPort(t *testing.T) {
	config := SMTPConfig{
		Host: "localhost",
		Port: 99999,
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	if result.Status != StatusDegraded {
		t.Errorf("expected degraded status for invalid port, got %s", result.Status)
	}
}

func TestSMTPChecker_Check_ConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	config := SMTPConfig{
		Host: "127.0.0.1",
		Port: port,
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	if result.Status != StatusDegraded {
		t.Errorf("expected degraded status for connection refused, got %s", result.Status)
	}
}

func TestSMTPChecker_Check_WithMockServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	serverDone := make(chan bool)
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		conn.Write([]byte("220 test.example.com ESMTP ready\r\n"))

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("250-test.example.com\r\n250-STARTTLS\r\n250 AUTH PLAIN\r\n"))
		}

		n, _ = conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("220 2.0.0 Ready to start TLS\r\n"))
		}

		n, _ = conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("235 2.7.0 Authentication successful\r\n"))
		}

		n, _ = conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("221 2.0.0 Bye\r\n"))
		}
	}()

	config := SMTPConfig{
		Host:     "127.0.0.1",
		Port:     port,
		Username: "test@example.com",
		Password: "password",
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Error("server timeout")
	}

	if result.Status != StatusHealthy && result.Status != StatusDegraded {
		t.Errorf("unexpected status: %s", result.Status)
	}

	if result.Details["host"] == nil {
		t.Errorf("expected host in details, got %v", result.Details["host"])
	}
}

func TestSMTPChecker_Check_EHLOFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	serverDone := make(chan bool)
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		conn.Write([]byte("220 test.example.com ESMTP ready\r\n"))

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("500 Syntax error, command unrecognized\r\n"))
		}
	}()

	config := SMTPConfig{
		Host: "127.0.0.1",
		Port: port,
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Error("server timeout")
	}

	if result.Status != StatusDegraded {
		t.Errorf("expected degraded status for EHLO failure, got %s", result.Status)
	}

	if !contains(result.Message, "EHLO") {
		t.Errorf("expected EHLO error in message, got: %s", result.Message)
	}
}

func TestSMTPChecker_Check_STARTTLSNotSupported(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	serverDone := make(chan bool)
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		conn.Write([]byte("220 test.example.com ESMTP ready\r\n"))

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("250-test.example.com\r\n250 AUTH PLAIN\r\n"))
		}

		n, _ = conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("221 2.0.0 Bye\r\n"))
		}
	}()

	config := SMTPConfig{
		Host: "127.0.0.1",
		Port: port,
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Error("server timeout")
	}

	if result.Details["starttls"] != "not supported" && result.Details["starttls"] != "failed" {
		t.Errorf("expected starttls not supported or failed, got: %v", result.Details["starttls"])
	}
}

func TestSMTPChecker_Check_AuthNotConfigured(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	serverDone := make(chan bool)
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		conn.Write([]byte("220 test.example.com ESMTP ready\r\n"))

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("250-test.example.com\r\n250-STARTTLS\r\n250 AUTH PLAIN\r\n"))
		}

		n, _ = conn.Read(buf)
		if n > 0 {
			conn.Write([]byte("221 2.0.0 Bye\r\n"))
		}
	}()

	config := SMTPConfig{
		Host: "127.0.0.1",
		Port: port,
	}

	checker := NewSMTPChecker(config)
	ctx := context.Background()

	result := checker.Check(ctx)

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Error("server timeout")
	}

	if result.Details["auth"] != "not configured" {
		t.Errorf("expected auth not configured, got: %v", result.Details["auth"])
	}
}

func TestSMTPConfig_GetAddress(t *testing.T) {
	config := SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	address := getSMTPAddress(config)
	expected := "smtp.example.com:587"
	if address != expected {
		t.Errorf("expected address %s, got %s", expected, address)
	}
}

func getSMTPAddress(config SMTPConfig) string {
	return config.Host + ":" + itoa(config.Port)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
