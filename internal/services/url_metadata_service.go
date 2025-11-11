package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrTitleNotFound     = errors.New("page title not found")
	ErrSSRFBlocked       = errors.New("request blocked for security reasons")
	ErrTooManyRedirects  = errors.New("too many redirects")
)

const (
	maxTitleBodyBytes     = 512 * 1024
	defaultUserAgent      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	maxRedirects          = 3
	requestTimeout        = 10 * time.Second
	dialTimeout           = 5 * time.Second
	tlsHandshakeTimeout   = 5 * time.Second
	responseHeaderTimeout = 5 * time.Second
	maxContentLength      = 10 * 1024 * 1024
)

type TitleFetcher interface {
	FetchTitle(ctx context.Context, rawURL string) (string, error)
}

type DNSResolver interface {
	LookupIP(hostname string) ([]net.IP, error)
}

type DefaultDNSResolver struct{}

func (d DefaultDNSResolver) LookupIP(hostname string) ([]net.IP, error) {
	return net.LookupIP(hostname)
}

type DNSCache struct {
	mu   sync.RWMutex
	data map[string][]net.IP
}

func NewDNSCache() *DNSCache {
	return &DNSCache{
		data: make(map[string][]net.IP),
	}
}

func (c *DNSCache) Get(hostname string) ([]net.IP, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ips, exists := c.data[hostname]
	return ips, exists
}

func (c *DNSCache) Set(hostname string, ips []net.IP) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[hostname] = ips
}

type CachedDNSResolver struct {
	resolver DNSResolver
	cache    *DNSCache
}

func NewCachedDNSResolver(resolver DNSResolver) *CachedDNSResolver {
	return &CachedDNSResolver{
		resolver: resolver,
		cache:    NewDNSCache(),
	}
}

func (c *CachedDNSResolver) LookupIP(hostname string) ([]net.IP, error) {
	if ips, exists := c.cache.Get(hostname); exists {
		return ips, nil
	}

	ips, err := c.resolver.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	c.cache.Set(hostname, ips)
	return ips, nil
}

type CustomDialer struct {
	cache    *DNSCache
	fallback *net.Dialer
}

func NewCustomDialer(cache *DNSCache) *CustomDialer {
	return &CustomDialer{
		cache: cache,
		fallback: &net.Dialer{
			Timeout: dialTimeout,
		},
	}
}

func (d *CustomDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return d.fallback.DialContext(ctx, network, address)
	}

	if ips, exists := d.cache.Get(host); exists {
		for _, ip := range ips {
			ipAddr := net.JoinHostPort(ip.String(), port)
			if conn, err := d.fallback.DialContext(ctx, network, ipAddr); err == nil {
				return conn, nil
			}
		}
	}

	return d.fallback.DialContext(ctx, network, address)
}

type URLMetadataService struct {
	client        *http.Client
	resolver      DNSResolver
	dnsCache      *DNSCache
	approvedHosts map[string]bool
	mu            sync.RWMutex
}

func NewURLMetadataService() *URLMetadataService {
	dnsCache := NewDNSCache()
	cachedResolver := NewCachedDNSResolver(DefaultDNSResolver{})
	customDialer := NewCustomDialer(dnsCache)

	svc := &URLMetadataService{
		resolver:      cachedResolver,
		dnsCache:      dnsCache,
		approvedHosts: make(map[string]bool),
	}

	transport := &http.Transport{
		DialContext:           customDialer.DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		DisableKeepAlives:     false,
	}

	svc.client = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}

			hostname := req.URL.Hostname()
			svc.mu.RLock()
			approved := svc.approvedHosts[hostname]
			svc.mu.RUnlock()

			if approved {
				return nil
			}

			if err := svc.validateURLForSSRF(req.URL); err != nil {
				return err
			}

			svc.mu.Lock()
			svc.approvedHosts[hostname] = true
			svc.mu.Unlock()

			return nil
		},
	}
	return svc
}

func (s *URLMetadataService) FetchTitle(ctx context.Context, rawURL string) (string, error) {
	if rawURL == "" {
		return "", errors.New("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrUnsupportedScheme
	}

	hostname := parsed.Hostname()
	s.mu.RLock()
	approved := s.approvedHosts[hostname]
	s.mu.RUnlock()

	if !approved {
		if err := s.validateURLForSSRF(parsed); err != nil {
			return "", err
		}

		s.mu.Lock()
		s.approvedHosts[hostname] = true
		s.mu.Unlock()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	request.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return "", ErrTitleNotFound
	}

	contentLength := resp.ContentLength
	if contentLength > maxContentLength {
		return "", ErrTitleNotFound
	}

	limited := io.LimitReader(resp.Body, maxTitleBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	title := s.ExtractTitleFromHTML(string(body))
	if title != "" {
		return title, nil
	}

	return "", ErrTitleNotFound
}

func (s *URLMetadataService) ExtractTitleFromHTML(html string) string {
	return s.ExtractFromTitleTag(html)
}

func (s *URLMetadataService) ExtractFromTitleTag(htmlContent string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return ""
			}
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if strings.EqualFold(token.Data, "title") {
				textTokenType := tokenizer.Next()
				if textTokenType == html.TextToken {
					rawTitle := tokenizer.Token().Data
					cleaned := s.optimizedTitleClean(rawTitle)
					if cleaned != "" {
						return cleaned
					}
				}
			}
		}
	}
}

func (s *URLMetadataService) optimizedTitleClean(title string) string {
	if title == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(title))

	inWhitespace := false
	started := false

	for _, r := range title {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if started && !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
			started = true
		}
	}

	cleaned := result.String()

	if len(cleaned) > 0 && cleaned[len(cleaned)-1] == ' ' {
		cleaned = cleaned[:len(cleaned)-1]
	}

	return cleaned
}

func (s *URLMetadataService) validateURLForSSRF(u *url.URL) error {
	switch {
	case u == nil,
		u.Scheme != "http" && u.Scheme != "https",
		u.Host == "",
		u.Hostname() == "",
		isLocalhost(u.Hostname()):
		return ErrSSRFBlocked
	}

	ips, err := s.resolver.LookupIP(u.Hostname())
	if err != nil {
		return ErrSSRFBlocked
	}
	for _, ip := range ips {
		if isPrivateOrReservedIP(ip) {
			return ErrSSRFBlocked
		}
	}
	return nil
}

func isLocalhost(hostname string) bool {
	hostname = strings.ToLower(hostname)

	localhostNames := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"0:0:0:0:0:0:0:1",
		"0:0:0:0:0:0:0:0",
	}

	for _, name := range localhostNames {
		if hostname == name {
			return true
		}
	}

	return false
}

func isPrivateOrReservedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return isPrivateIPv6(ip)
	}

	privateRanges := []struct {
		start, end net.IP
	}{
		{net.IPv4(10, 0, 0, 0), net.IPv4(10, 255, 255, 255)},
		{net.IPv4(172, 16, 0, 0), net.IPv4(172, 31, 255, 255)},
		{net.IPv4(192, 168, 0, 0), net.IPv4(192, 168, 255, 255)},
		{net.IPv4(127, 0, 0, 0), net.IPv4(127, 255, 255, 255)},
		{net.IPv4(169, 254, 0, 0), net.IPv4(169, 254, 255, 255)},
		{net.IPv4(224, 0, 0, 0), net.IPv4(239, 255, 255, 255)},
		{net.IPv4(240, 0, 0, 0), net.IPv4(255, 255, 255, 255)},
	}

	for _, r := range privateRanges {
		if ipInRange(ipv4, r.start, r.end) {
			return true
		}
	}

	return false
}

func isPrivateIPv6(ip net.IP) bool {
	privateRanges := []struct {
		prefix []byte
		length int
	}{
		{[]byte{0xfc, 0x00}, 7},
		{[]byte{0xfe, 0x80}, 10},
		{[]byte{0xff, 0x00}, 8},
		{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}, 128},
	}

	for _, r := range privateRanges {
		if ipv6InRange(ip, r.prefix, r.length) {
			return true
		}
	}

	return false
}

func ipInRange(ip, start, end net.IP) bool {
	ipInt := ipToInt(ip)
	startInt := ipToInt(start)
	endInt := ipToInt(end)
	return ipInt >= startInt && ipInt <= endInt
}

func ipToInt(ip net.IP) uint32 {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	return uint32(ipv4[0])<<24 + uint32(ipv4[1])<<16 + uint32(ipv4[2])<<8 + uint32(ipv4[3])
}

func ipv6InRange(ip net.IP, prefix []byte, length int) bool {
	ipBytes := ip.To16()
	if ipBytes == nil {
		return false
	}

	bytesToCompare := length / 8
	bitsToCompare := length % 8

	for i := 0; i < bytesToCompare && i < len(prefix) && i < len(ipBytes); i++ {
		if ipBytes[i] != prefix[i] {
			return false
		}
	}

	if bitsToCompare > 0 && bytesToCompare < len(prefix) && bytesToCompare < len(ipBytes) {
		mask := byte(0xff) << (8 - bitsToCompare)
		if (ipBytes[bytesToCompare] & mask) != (prefix[bytesToCompare] & mask) {
			return false
		}
	}

	return true
}
