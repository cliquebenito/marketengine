package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGet_BaseURLAndHeaders(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-API-KEY")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithHeader("X-API-KEY", "k1"))
	body, err := c.Get(context.Background(), "/v1/foo")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotPath != "/v1/foo" {
		t.Errorf("path: want /v1/foo, got %q", gotPath)
	}
	if gotKey != "k1" {
		t.Errorf("api key: want k1, got %q", gotKey)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body: %s", body)
	}
}

func TestGet_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	_, err := c.Get(context.Background(), "/x")
	if err == nil || !strings.Contains(err.Error(), "status 429") {
		t.Fatalf("want status 429 error, got %v", err)
	}
}

func TestThrottle_EnforcesMinInterval(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithThrottle(50*time.Millisecond))
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), "/p"); err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	if elapsed < 100*time.Millisecond {
		t.Errorf("throttle not enforced: 3 calls took %v", elapsed)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("hits: want 3, got %d", got)
	}
}

func TestGetJSON_Decodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value": 42}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	var out struct {
		Value int `json:"value"`
	}
	if err := c.GetJSON(context.Background(), "/x", &out); err != nil {
		t.Fatalf("getjson: %v", err)
	}
	if out.Value != 42 {
		t.Errorf("value: want 42, got %d", out.Value)
	}
}

func TestTruncate(t *testing.T) {
	b := []byte(strings.Repeat("a", 250))
	got := Truncate(b, 100)
	if len(got) != 103 || !strings.HasSuffix(got, "...") {
		t.Errorf("truncate: got len=%d (%s)", len(got), got)
	}
	short := Truncate([]byte("ok"), 100)
	if short != "ok" {
		t.Errorf("short: %q", short)
	}
}

func TestSHA256_Stable(t *testing.T) {
	a := SHA256("hello")
	b := SHA256("hello")
	if a != b || len(a) != 64 {
		t.Errorf("sha256 not stable or wrong len: %s vs %s", a, b)
	}
}
