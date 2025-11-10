package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
)

const (
	StandardUserAgent      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	StandardAcceptEncoding = "gzip"
)

type RequestBuilder struct {
	method         string
	url            string
	body           io.Reader
	headers        map[string]string
	withAuth       bool
	authToken      string
	withJSON       bool
	jsonData       any
	withStdHeaders bool
	withIP         bool
	ipAddress      string
}

func NewRequestBuilder(method, url string) *RequestBuilder {
	return &RequestBuilder{
		method:         method,
		url:            url,
		headers:        make(map[string]string),
		withStdHeaders: true,
	}
}

func (rb *RequestBuilder) WithBody(body io.Reader) *RequestBuilder {
	rb.body = body
	return rb
}

func (rb *RequestBuilder) WithJSONBody(data any) *RequestBuilder {
	rb.withJSON = true
	rb.jsonData = data
	return rb
}

func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	maps.Copy(rb.headers, headers)
	return rb
}

func (rb *RequestBuilder) WithAuth(token string) *RequestBuilder {
	rb.withAuth = true
	rb.authToken = token
	return rb
}

func (rb *RequestBuilder) WithIP(ipAddress string) *RequestBuilder {
	rb.withIP = true
	rb.ipAddress = ipAddress
	return rb
}

func (rb *RequestBuilder) WithoutStandardHeaders() *RequestBuilder {
	rb.withStdHeaders = false
	return rb
}

func (rb *RequestBuilder) Build() (*http.Request, error) {
	var body io.Reader = rb.body
	if rb.withJSON && rb.jsonData != nil {
		jsonBytes, err := json.Marshal(rb.jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		body = bytes.NewReader(jsonBytes)
	}
	request, err := http.NewRequest(rb.method, rb.url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if rb.withStdHeaders {
		request.Header.Set("User-Agent", StandardUserAgent)
		request.Header.Set("Accept-Encoding", StandardAcceptEncoding)
	}
	if rb.withJSON {
		request.Header.Set("Content-Type", "application/json")
	}
	if rb.withAuth && rb.authToken != "" {
		request.Header.Set("Authorization", "Bearer "+rb.authToken)
	}
	if rb.withIP && rb.ipAddress != "" {
		request.Header.Set("X-Forwarded-For", rb.ipAddress)
	}
	for key, value := range rb.headers {
		request.Header.Set(key, value)
	}
	return request, nil
}

func (rb *RequestBuilder) BuildOrFatal(t TestingT) *http.Request {
	req, err := rb.Build()
	if err != nil {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		t.Fatalf("RequestBuilder.Build failed: %v", err)
	}
	return req
}

type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}
