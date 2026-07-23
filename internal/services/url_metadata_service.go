package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
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

	dnsPinTTL         = 5 * time.Minute
	maxDNSPinnedHosts = 1024
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

type dnsCacheEntry struct {
	ips       []net.IP
	expiresAt time.Time
}

type DNSCache struct {
	mu   sync.RWMutex
	data map[string]dnsCacheEntry
}

func NewDNSCache() *DNSCache {
	return &DNSCache{
		data: make(map[string]dnsCacheEntry),
	}
}

func (c *DNSCache) Get(hostname string) ([]net.IP, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.data[hostname]
	if !exists || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.ips, true
}

func (c *DNSCache) Set(hostname string, ips []net.IP) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.data) >= maxDNSPinnedHosts {
		now := time.Now()
		for host, entry := range c.data {
			if now.After(entry.expiresAt) {
				delete(c.data, host)
			}
		}
		for host := range c.data {
			if len(c.data) < maxDNSPinnedHosts {
				break
			}
			delete(c.data, host)
		}
	}

	c.data[hostname] = dnsCacheEntry{
		ips:       ips,
		expiresAt: time.Now().Add(dnsPinTTL),
	}
}

type CustomDialer struct {
	cache  *DNSCache
	dialer *net.Dialer
}

func NewCustomDialer(cache *DNSCache) *CustomDialer {
	return &CustomDialer{
		cache: cache,
		dialer: &net.Dialer{
			Timeout: dialTimeout,
		},
	}
}

func (d *CustomDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrSSRFBlocked
	}

	ips, exists := d.cache.Get(host)
	if !exists {
		return nil, ErrSSRFBlocked
	}

	var lastErr error
	for _, ip := range ips {
		conn, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = ErrSSRFBlocked
	}
	return nil, lastErr
}

type URLMetadataService struct {
	client   *http.Client
	resolver DNSResolver
	dnsCache *DNSCache
}

func NewURLMetadataService() *URLMetadataService {
	svc := &URLMetadataService{
		resolver: DefaultDNSResolver{},
		dnsCache: NewDNSCache(),
	}

	customDialer := NewCustomDialer(svc.dnsCache)

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
			return svc.validateURLForSSRF(req.URL)
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

	if err := s.validateURLForSSRF(parsed); err != nil {
		return "", err
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

	hostname := u.Hostname()
	if _, pinned := s.dnsCache.Get(hostname); pinned {
		return nil
	}

	ips, err := s.resolver.LookupIP(hostname)
	if err != nil || len(ips) == 0 {
		return ErrSSRFBlocked
	}
	if slices.ContainsFunc(ips, isPrivateOrReservedIP) {
		return ErrSSRFBlocked
	}

	s.dnsCache.Set(hostname, ips)
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

	return slices.Contains(localhostNames, hostname)
}

var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func isPrivateOrReservedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()

	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return true
	}

	for _, prefix := range reservedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
