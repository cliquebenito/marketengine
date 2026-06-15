package httpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ipv4Transport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func init() {

	d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	ipv4Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		switch network {
		case "tcp", "tcp6":
			network = "tcp4"
		}
		return d.DialContext(ctx, network, addr)
	}
}

type Client struct {
	baseURL     string
	headers     map[string]string
	http        *http.Client
	throttleMin time.Duration

	mu          sync.Mutex
	lastRequest time.Time
}

type Option func(*Client)

func New(opts ...Option) *Client {
	c := &Client{
		headers: make(map[string]string),
		http:    &http.Client{Timeout: 30 * time.Second, Transport: ipv4Transport},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

func WithHeader(k, v string) Option {
	return func(c *Client) { c.headers[k] = v }
}

func WithThrottle(min time.Duration) Option {
	return func(c *Client) { c.throttleMin = min }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	c.waitThrottle(ctx)

	url := path
	if !strings.HasPrefix(path, "http") {
		url = c.baseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, Truncate(body, 200))
	}
	return body, nil
}

func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	body, err := c.Get(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func (c *Client) waitThrottle(ctx context.Context) {
	if c.throttleMin <= 0 {
		return
	}
	c.mu.Lock()
	wait := c.throttleMin - time.Since(c.lastRequest)
	c.mu.Unlock()
	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
		case <-t.C:
		}
	}
	c.mu.Lock()
	c.lastRequest = time.Now()
	c.mu.Unlock()
}

func SHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func Truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}
