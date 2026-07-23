package services

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFetchTitleSuccess(t *testing.T) {
	svc := NewURLMetadataService()
	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader("<html><head><title>  Example\n Title  </title></head></html>"))
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
	})

	mockResolver := NewMockDNSResolver()
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	title, err := svc.FetchTitle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("FetchTitle returned error: %v", err)
	}

	if title != "Example Title" {
		t.Fatalf("expected sanitized title, got %q", title)
	}
}

func TestFetchTitleErrors(t *testing.T) {
	svc := NewURLMetadataService()

	if _, err := svc.FetchTitle(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty URL")
	}

	if _, err := svc.FetchTitle(context.Background(), ":://invalid"); err == nil {
		t.Fatal("expected parse error for invalid URL")
	}

	if _, err := svc.FetchTitle(context.Background(), "ftp://example.com"); !errors.Is(err, ErrUnsupportedScheme) {
		t.Fatalf("expected ErrUnsupportedScheme, got %v", err)
	}
}

func TestFetchTitleHTTPFailures(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(*http.Request) (*http.Response, error)
		wantErr    string
		wantTarget error
	}{
		{
			name: "NonOKStatus",
			handler: func(*http.Request) (*http.Response, error) {
				body := io.NopCloser(strings.NewReader("error"))
				return &http.Response{StatusCode: http.StatusBadGateway, Body: body, Header: make(http.Header)}, nil
			},
			wantErr: "unexpected status code",
		},
		{
			name: "NoTitle",
			handler: func(*http.Request) (*http.Response, error) {
				body := io.NopCloser(strings.NewReader("<html><head></head><body>No title</body></html>"))
				header := make(http.Header)
				header.Set("Content-Type", "text/html; charset=utf-8")
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
			},
			wantTarget: ErrTitleNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewURLMetadataService()
			svc.client = newTestClient(t, tc.handler)

			mockResolver := NewMockDNSResolver()
			mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
			svc.resolver = mockResolver

			_, err := svc.FetchTitle(context.Background(), "https://example.com")
			if err == nil {
				t.Fatal("expected error but got nil")
			}

			if tc.wantTarget != nil {
				if !errors.Is(err, tc.wantTarget) {
					t.Fatalf("expected error %v, got %v", tc.wantTarget, err)
				}
				return
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestFetchTitleSkipsEmptyTitles(t *testing.T) {
	svc := NewURLMetadataService()

	sampleHTML := `<!DOCTYPE html><html><head><svg><title> </title></svg><title>Real Video Title - YouTube</title></head><body></body></html>`

	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
		headers := make(http.Header)
		headers.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(sampleHTML)),
		}, nil
	})

	mockResolver := NewMockDNSResolver()
	mockResolver.SetLookupResult("www.youtube.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	title, err := svc.FetchTitle(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("FetchTitle returned error: %v", err)
	}

	if title != "Real Video Title - YouTube" {
		t.Fatalf("expected real title, got %q", title)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(t *testing.T, fn roundTripFunc) *http.Client {
	t.Helper()
	return &http.Client{Transport: fn}
}

type MockDNSResolver struct {
	lookupResults map[string][]net.IP
	lookupErrors  map[string]error
}

func NewMockDNSResolver() *MockDNSResolver {
	return &MockDNSResolver{
		lookupResults: make(map[string][]net.IP),
		lookupErrors:  make(map[string]error),
	}
}

func (m *MockDNSResolver) LookupIP(hostname string) ([]net.IP, error) {
	if err, exists := m.lookupErrors[hostname]; exists {
		return nil, err
	}
	if ips, exists := m.lookupResults[hostname]; exists {
		return ips, nil
	}

	if ip := net.ParseIP(hostname); ip != nil {
		return []net.IP{ip}, nil
	}

	return []net.IP{net.ParseIP("8.8.8.8")}, nil
}

func (m *MockDNSResolver) SetLookupResult(hostname string, ips []net.IP) {
	m.lookupResults[hostname] = ips
}

func (m *MockDNSResolver) SetLookupError(hostname string, err error) {
	m.lookupErrors[hostname] = err
}

func TestSSRFProtection(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorType   error
	}{
		{
			name:        "localhost blocked",
			url:         "http://localhost:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "127.0.0.1 blocked",
			url:         "http://127.0.0.1:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "private IP 10.0.0.1 blocked",
			url:         "http://10.0.0.1:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "private IP 192.168.1.1 blocked",
			url:         "http://192.168.1.1:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "private IP 172.16.0.1 blocked",
			url:         "http://172.16.0.1:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "link-local 169.254.0.1 blocked",
			url:         "http://169.254.0.1:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "multicast 224.0.0.1 blocked",
			url:         "http://224.0.0.1:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "valid public domain allowed",
			url:         "https://example.com",
			expectError: false,
		},
		{
			name:        "IPv6 localhost blocked",
			url:         "http://[::1]:8080",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
		{
			name:        "empty host blocked",
			url:         "http://",
			expectError: true,
			errorType:   ErrSSRFBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewURLMetadataService()

			mockResolver := NewMockDNSResolver()
			svc.resolver = mockResolver

			if tt.url == "https://example.com" {
				mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
			}

			if tt.expectError && strings.Contains(tt.url, "://") {
				if u, err := url.Parse(tt.url); err == nil {
					hostname := u.Hostname()
					if hostname != "" {
						if ip := net.ParseIP(hostname); ip != nil {
							mockResolver.SetLookupResult(hostname, []net.IP{ip})
						}
					}
				}
			}

			if !tt.expectError && tt.url == "https://example.com" {
				svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
					body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
					header := make(http.Header)
					header.Set("Content-Type", "text/html; charset=utf-8")
					return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
				})
			}

			_, err := svc.FetchTitle(context.Background(), tt.url)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for URL %q, got nil", tt.url)
				}
				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Fatalf("expected error type %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for URL %q: %v", tt.url, err)
				}
			}
		})
	}
}

func TestRedirectLimiting(t *testing.T) {
	svc := NewURLMetadataService()

	mockResolver := NewMockDNSResolver()
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	svc.client = &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}
			return nil
		},
		Transport: newTestClient(t, func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusMovedPermanently,
				Header:     http.Header{"Location": []string{"https://example.com/redirect"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}).Transport,
	}

	_, err := svc.FetchTitle(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error for too many redirects")
	}
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("expected ErrTooManyRedirects, got %v", err)
	}
}

func TestValidateURLForSSRF(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "valid public URL",
			url:         "https://example.com",
			expectError: false,
		},
		{
			name:        "localhost blocked",
			url:         "http://localhost",
			expectError: true,
		},
		{
			name:        "127.0.0.1 blocked",
			url:         "http://127.0.0.1",
			expectError: true,
		},
		{
			name:        "private IP blocked",
			url:         "http://192.168.1.1",
			expectError: true,
		},
		{
			name:        "empty host blocked",
			url:         "http://",
			expectError: true,
		},
		{
			name:        "nil URL blocked",
			url:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u *url.URL
			var err error

			if tt.url != "" {
				u, err = url.Parse(tt.url)
				if err != nil {
					t.Fatalf("failed to parse URL %q: %v", tt.url, err)
				}
			}

			svc := NewURLMetadataService()
			mockResolver := NewMockDNSResolver()
			svc.resolver = mockResolver

			if tt.url == "https://example.com" {
				mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
			}

			if u != nil && u.Hostname() != "" {
				if ip := net.ParseIP(u.Hostname()); ip != nil {
					mockResolver.SetLookupResult(u.Hostname(), []net.IP{ip})
				}
			}

			err = svc.validateURLForSSRF(u)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for URL %q, got nil", tt.url)
				}
				if !errors.Is(err, ErrSSRFBlocked) {
					t.Fatalf("expected ErrSSRFBlocked, got %v", err)
				}
			} else {
				if err != nil && !strings.Contains(err.Error(), "fetch url") {
					t.Fatalf("unexpected error for URL %q: %v", tt.url, err)
				}
			}
		})
	}
}

func TestIsPrivateOrReservedIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"10.0.0.1", "10.0.0.1", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"172.16.0.1", "172.16.0.1", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"192.168.1.1", "192.168.1.1", true},
		{"192.168.255.255", "192.168.255.255", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"169.254.0.1", "169.254.0.1", true},
		{"224.0.0.1", "224.0.0.1", true},
		{"240.0.0.1", "240.0.0.1", true},
		{"0.0.0.0", "0.0.0.0", true},
		{"0.1.2.3", "0.1.2.3", true},
		{"CGNAT 100.64.0.1", "100.64.0.1", true},
		{"CGNAT 100.127.255.255", "100.127.255.255", true},
		{"IETF 192.0.0.1", "192.0.0.1", true},
		{"TEST-NET-1 192.0.2.1", "192.0.2.1", true},
		{"benchmarking 198.18.0.1", "198.18.0.1", true},
		{"benchmarking 198.19.255.255", "198.19.255.255", true},
		{"TEST-NET-2 198.51.100.1", "198.51.100.1", true},
		{"TEST-NET-3 203.0.113.1", "203.0.113.1", true},
		{"broadcast 255.255.255.255", "255.255.255.255", true},
		{"IPv4-mapped private ::ffff:10.0.0.1", "::ffff:10.0.0.1", true},
		{"IPv4-mapped loopback ::ffff:127.0.0.1", "::ffff:127.0.0.1", true},

		{"IPv6 loopback ::1", "::1", true},
		{"IPv6 unspecified ::", "::", true},
		{"IPv6 ULA fc00::1", "fc00::1", true},
		{"IPv6 ULA fd00::1", "fd00::1", true},
		{"IPv6 link-local fe80::1", "fe80::1", true},
		{"IPv6 multicast ff00::1", "ff00::1", true},
		{"IPv6 interface-local multicast ff01::1", "ff01::1", true},
		{"NAT64 64:ff9b::7f00:1", "64:ff9b::7f00:1", true},
		{"discard-only 100::1", "100::1", true},
		{"documentation 2001:db8::1", "2001:db8::1", true},

		{"8.8.8.8", "8.8.8.8", false},
		{"1.1.1.1", "1.1.1.1", false},
		{"74.125.224.72", "74.125.224.72", false},
		{"100.128.0.1 just past CGNAT", "100.128.0.1", false},
		{"IPv6 public 2001:4860::1", "2001:4860::1", false},
		{"IPv6 public 2607:f8b0::1", "2607:f8b0::1", false},
		{"IPv4-mapped public ::ffff:8.8.8.8", "::ffff:8.8.8.8", false},

		{"nil IP", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ip net.IP
			if tt.ip != "" {
				ip = net.ParseIP(tt.ip)
			}

			result := isPrivateOrReservedIP(ip)
			if result != tt.expected {
				t.Fatalf("expected %v for IP %q, got %v", tt.expected, tt.ip, result)
			}
		})
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		hostname string
		expected bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"0:0:0:0:0:0:0:1", true},
		{"0:0:0:0:0:0:0:0", true},
		{"example.com", false},
		{"192.168.1.1", false},
		{"8.8.8.8", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := isLocalhost(tt.hostname)
			if result != tt.expected {
				t.Fatalf("expected %v for hostname %q, got %v", tt.expected, tt.hostname, result)
			}
		})
	}
}

func TestExtractFromTitleTag(t *testing.T) {
	svc := NewURLMetadataService()

	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "simple title",
			html:     `<html><head><title>Test Title</title></head></html>`,
			expected: "Test Title",
		},
		{
			name:     "title with whitespace",
			html:     `<html><head><title>  Test   Title  </title></head></html>`,
			expected: "Test Title",
		},
		{
			name:     "title with newlines",
			html:     `<html><head><title>Test` + "\n" + `Title</title></head></html>`,
			expected: "Test Title",
		},
		{
			name:     "empty title",
			html:     `<html><head><title></title></head></html>`,
			expected: "",
		},
		{
			name:     "whitespace only title",
			html:     `<html><head><title>   </title></head></html>`,
			expected: "",
		},
		{
			name:     "no title tag",
			html:     `<html><head></head></html>`,
			expected: "",
		},
		{
			name:     "title in svg (first title found)",
			html:     `<html><head><svg><title>SVG Title</title></svg><title>Real Title</title></head></html>`,
			expected: "SVG Title",
		},
		{
			name:     "multiple title tags (first non-empty)",
			html:     `<html><head><title>First Title</title><title>Second Title</title></head></html>`,
			expected: "First Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ExtractFromTitleTag(tt.html)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractTitleFromHTML(t *testing.T) {
	svc := NewURLMetadataService()

	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "title tag extracted",
			html:     `<html><head><title>Title Tag</title><meta property="og:title" content="OG Title"></head></html>`,
			expected: "Title Tag",
		},
		{
			name:     "no title tag returns empty",
			html:     `<html><head><meta property="og:title" content="OG Title"></head></html>`,
			expected: "",
		},
		{
			name:     "empty title tag returns empty",
			html:     `<html><head><title></title><meta property="og:title" content="OG Title"></head></html>`,
			expected: "",
		},
		{
			name:     "whitespace title tag returns empty",
			html:     `<html><head><title>   </title><meta property="og:title" content="OG Title"></head></html>`,
			expected: "",
		},
		{
			name:     "no title found",
			html:     `<html><head></head></html>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ExtractTitleFromHTML(tt.html)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestOptimizedTitleClean(t *testing.T) {
	svc := NewURLMetadataService()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple title",
			input:    "Simple Title",
			expected: "Simple Title",
		},
		{
			name:     "leading and trailing whitespace",
			input:    "  Title  ",
			expected: "Title",
		},
		{
			name:     "multiple spaces",
			input:    "Title   with    spaces",
			expected: "Title with spaces",
		},
		{
			name:     "tabs and newlines",
			input:    "Title\twith\nnewlines\r\nand\ttabs",
			expected: "Title with newlines and tabs",
		},
		{
			name:     "mixed whitespace",
			input:    "  \t  Title   \n  with  \r\n  mixed  \t  whitespace  ",
			expected: "Title with mixed whitespace",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \t\n\r   ",
			expected: "",
		},
		{
			name:     "single character",
			input:    "A",
			expected: "A",
		},
		{
			name:     "single character with whitespace",
			input:    "  A  ",
			expected: "A",
		},
		{
			name:     "unicode characters",
			input:    "  Title  with  émojis  🎉  ",
			expected: "Title with émojis 🎉",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.optimizedTitleClean(tt.input)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestContentTypeValidation(t *testing.T) {
	svc := NewURLMetadataService()

	tests := []struct {
		name        string
		contentType string
		expectError bool
	}{
		{
			name:        "valid HTML content type",
			contentType: "text/html; charset=utf-8",
			expectError: false,
		},
		{
			name:        "HTML without charset",
			contentType: "text/html",
			expectError: false,
		},
		{
			name:        "HTML with different charset",
			contentType: "text/html; charset=iso-8859-1",
			expectError: false,
		},
		{
			name:        "XHTML content type",
			contentType: "application/xhtml+xml",
			expectError: true,
		},
		{
			name:        "invalid content type - JSON",
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "invalid content type - plain text",
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "invalid content type - XML",
			contentType: "application/xml",
			expectError: true,
		},
		{
			name:        "empty content type",
			contentType: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
				body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
				header := make(http.Header)
				header.Set("Content-Type", tt.contentType)
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
			})

			mockResolver := NewMockDNSResolver()
			mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
			svc.resolver = mockResolver

			_, err := svc.FetchTitle(context.Background(), "https://example.com")

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !errors.Is(err, ErrTitleNotFound) {
					t.Fatalf("expected ErrTitleNotFound, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestContentLengthLimit(t *testing.T) {
	svc := NewURLMetadataService()

	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          body,
			Header:        header,
			ContentLength: 15000000,
		}, nil
	})

	mockResolver := NewMockDNSResolver()
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	_, err := svc.FetchTitle(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error for content length exceeding limit")
	}
	if !errors.Is(err, ErrTitleNotFound) {
		t.Fatalf("expected ErrTitleNotFound, got %v", err)
	}
}

func TestHTTPHeaders(t *testing.T) {
	svc := NewURLMetadataService()

	var capturedRequest *http.Request
	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
		capturedRequest = r
		body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
	})

	mockResolver := NewMockDNSResolver()
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	_, err := svc.FetchTitle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedUserAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	if capturedRequest.Header.Get("User-Agent") != expectedUserAgent {
		t.Fatalf("expected User-Agent %q, got %q", expectedUserAgent, capturedRequest.Header.Get("User-Agent"))
	}

	expectedAccept := "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	if capturedRequest.Header.Get("Accept") != expectedAccept {
		t.Fatalf("expected Accept %q, got %q", expectedAccept, capturedRequest.Header.Get("Accept"))
	}

	expectedAcceptLanguage := "en-US,en;q=0.5"
	if capturedRequest.Header.Get("Accept-Language") != expectedAcceptLanguage {
		t.Fatalf("expected Accept-Language %q, got %q", expectedAcceptLanguage, capturedRequest.Header.Get("Accept-Language"))
	}
}

func TestDNSCaching(t *testing.T) {
	svc := NewURLMetadataService()

	lookupCount := 0
	mockResolver := &CountingMockDNSResolver{
		MockDNSResolver: MockDNSResolver{
			lookupResults: make(map[string][]net.IP),
			lookupErrors:  make(map[string]error),
		},
		lookupCount: &lookupCount,
	}
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
	})

	_, err := svc.FetchTitle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookupCount != 1 {
		t.Fatalf("expected 1 DNS lookup, got %d", lookupCount)
	}

	_, err = svc.FetchTitle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookupCount != 1 {
		t.Fatalf("expected 1 DNS lookup (cached), got %d", lookupCount)
	}
}

type CountingMockDNSResolver struct {
	MockDNSResolver
	lookupCount *int
}

func (c *CountingMockDNSResolver) LookupIP(hostname string) ([]net.IP, error) {
	*c.lookupCount++
	return c.MockDNSResolver.LookupIP(hostname)
}

func TestCustomDialerRequiresPinnedIP(t *testing.T) {
	cache := NewDNSCache()
	dialer := NewCustomDialer(cache)

	_, err := dialer.DialContext(context.Background(), "tcp", "unvalidated.example.com:443")
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("expected ErrSSRFBlocked for unpinned host, got %v", err)
	}

	_, err = dialer.DialContext(context.Background(), "tcp", "missing-port.example.com")
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("expected ErrSSRFBlocked for invalid address, got %v", err)
	}
}

func TestDNSCacheExpiry(t *testing.T) {
	cache := NewDNSCache()
	cache.Set("example.com", []net.IP{net.ParseIP("8.8.8.8")})

	if _, exists := cache.Get("example.com"); !exists {
		t.Fatal("expected fresh entry to be returned")
	}

	cache.mu.Lock()
	entry := cache.data["example.com"]
	entry.expiresAt = time.Now().Add(-time.Second)
	cache.data["example.com"] = entry
	cache.mu.Unlock()

	if _, exists := cache.Get("example.com"); exists {
		t.Fatal("expected expired entry to be treated as a miss")
	}
}

func TestExpiredPinTriggersRevalidation(t *testing.T) {
	svc := NewURLMetadataService()

	lookupCount := 0
	mockResolver := &CountingMockDNSResolver{
		MockDNSResolver: MockDNSResolver{
			lookupResults: make(map[string][]net.IP),
			lookupErrors:  make(map[string]error),
		},
		lookupCount: &lookupCount,
	}
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
	})

	if _, err := svc.FetchTitle(context.Background(), "https://example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookupCount != 1 {
		t.Fatalf("expected 1 DNS lookup, got %d", lookupCount)
	}

	svc.dnsCache.mu.Lock()
	entry := svc.dnsCache.data["example.com"]
	entry.expiresAt = time.Now().Add(-time.Second)
	svc.dnsCache.data["example.com"] = entry
	svc.dnsCache.mu.Unlock()

	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("127.0.0.1")})

	if _, err := svc.FetchTitle(context.Background(), "https://example.com"); !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("expected ErrSSRFBlocked after DNS rebind to private IP, got %v", err)
	}
	if lookupCount != 2 {
		t.Fatalf("expected expired pin to trigger a second DNS lookup, got %d", lookupCount)
	}
}

func TestFetchTitleWithDifferentStatusCodes(t *testing.T) {
	svc := NewURLMetadataService()

	tests := []struct {
		name       string
		statusCode int
		expectErr  bool
	}{
		{"OK status", http.StatusOK, false},
		{"Created status", http.StatusCreated, false},
		{"Accepted status", http.StatusAccepted, false},
		{"No Content status", http.StatusNoContent, false},
		{"Bad Request", http.StatusBadRequest, true},
		{"Unauthorized", http.StatusUnauthorized, true},
		{"Forbidden", http.StatusForbidden, true},
		{"Not Found", http.StatusNotFound, true},
		{"Internal Server Error", http.StatusInternalServerError, true},
		{"Bad Gateway", http.StatusBadGateway, true},
		{"Service Unavailable", http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {
				body := io.NopCloser(strings.NewReader("<html><head><title>Test Title</title></head></html>"))
				header := make(http.Header)
				header.Set("Content-Type", "text/html; charset=utf-8")
				return &http.Response{StatusCode: tt.statusCode, Body: body, Header: header}, nil
			})

			mockResolver := NewMockDNSResolver()
			mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
			svc.resolver = mockResolver

			_, err := svc.FetchTitle(context.Background(), "https://example.com")

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), "unexpected status code") {
					t.Fatalf("expected 'unexpected status code' error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestFetchTitleWithBodyReadError(t *testing.T) {
	svc := NewURLMetadataService()

	svc.client = newTestClient(t, func(r *http.Request) (*http.Response, error) {

		errorReader := &errorReader{}
		body := io.NopCloser(errorReader)
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: header}, nil
	})

	mockResolver := NewMockDNSResolver()
	mockResolver.SetLookupResult("example.com", []net.IP{net.ParseIP("8.8.8.8")})
	svc.resolver = mockResolver

	_, err := svc.FetchTitle(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "read body") {
		t.Fatalf("expected 'read body' error, got %v", err)
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
