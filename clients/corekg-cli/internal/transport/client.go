package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
	UserAgent  string
}

func New(baseURL string) (*Client, error) {
	return NewWithTimeout(baseURL, 30*time.Second)
}

func NewWithTimeout(baseURL string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("server URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("server URL must include a host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("server URL must not include user information")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("server URL must not include a query or fragment")
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(basePath, "/v3") {
		basePath += "/v3"
	}
	parsed.Path = basePath + "/"
	parsed.RawQuery = ""
	parsed.Fragment = ""

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return &Client{BaseURL: parsed, HTTPClient: httpClient, UserAgent: "corekg-cli"}, nil
}

func (c *Client) Endpoint(action string) (*url.URL, error) {
	if c == nil || c.BaseURL == nil {
		return nil, fmt.Errorf("HTTP client is not initialized")
	}
	action = strings.TrimLeft(strings.TrimSpace(action), "/")
	if action == "" || strings.Contains(action, "..") {
		return nil, fmt.Errorf("invalid API action %q", action)
	}
	endpoint := *c.BaseURL
	endpoint.Path = strings.TrimRight(c.BaseURL.Path, "/") + "/" + action
	return &endpoint, nil
}

func (c *Client) Do(ctx context.Context, apiKey, action string, body []byte) (*http.Response, error) {
	return c.DoReader(ctx, apiKey, action, "application/json", bytes.NewReader(body))
}

func (c *Client) DoReader(ctx context.Context, apiKey, action, contentType string, body io.Reader) (*http.Response, error) {
	endpoint, err := c.Endpoint(action)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", action, err)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("User-Agent", c.UserAgent)
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if c.HTTPClient == nil {
		return nil, fmt.Errorf("HTTP client is not initialized")
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", action, err)
	}
	return response, nil
}

func ReadBody(response *http.Response, limit int64) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("HTTP response body is empty")
	}
	defer response.Body.Close()
	if limit <= 0 {
		return nil, fmt.Errorf("HTTP response body limit must be positive")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read HTTP response: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("HTTP response body exceeds %d bytes", limit)
	}
	return body, nil
}
