package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type echoResponse struct {
	Query  string `json:"query"`
	Method string `json:"method"`
}

func TestClientGet(t *testing.T) {
	client := NewClient("https://example.com", "token", 2*time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		resp := echoResponse{Query: r.URL.Query().Encode(), Method: r.Method}
		payload, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	query := url.Values{}
	query.Set("foo", "bar")
	var out echoResponse
	if _, err := client.Get(context.Background(), "/test", query, &out); err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Method != http.MethodGet {
		t.Fatalf("expected GET, got %s", out.Method)
	}
	if out.Query != "foo=bar" {
		t.Fatalf("unexpected query: %s", out.Query)
	}
}

func TestClientPostRequestID(t *testing.T) {
	client := NewClient("https://example.com", "token", 2*time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Request-Id") == "" {
			t.Fatalf("missing request id")
		}
		resp := echoResponse{Query: r.URL.Query().Encode(), Method: r.Method}
		payload, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	var out echoResponse
	if _, err := client.Post(context.Background(), "/test", nil, map[string]any{"ok": true}, &out, true); err != nil {
		t.Fatalf("post: %v", err)
	}
	if out.Method != http.MethodPost {
		t.Fatalf("expected POST, got %s", out.Method)
	}
}

func TestClientQuickAdd(t *testing.T) {
	client := NewClient("https://example.com", "token", 2*time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://example.com/tasks/quick" {
			t.Fatalf("unexpected url: %s", r.URL.String())
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["text"] != "Buy milk" {
			t.Fatalf("unexpected body: %#v", payload)
		}
		respPayload, _ := json.Marshal(map[string]any{
			"id":      "123",
			"content": "Buy milk",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respPayload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	task, reqID, err := client.QuickAdd(context.Background(), "Buy milk")
	if err != nil {
		t.Fatalf("quick add: %v", err)
	}
	if reqID == "" {
		t.Fatalf("expected request id")
	}
	if task.ID != "123" || task.Content != "Buy milk" {
		t.Fatalf("unexpected task: %#v", task)
	}
}

func TestClientRetriesGetOnTransientStatus(t *testing.T) {
	client := NewClient("https://example.com", "token", 2*time.Second)
	origWait := waitForRetry
	waitForRetry = func(ctx context.Context, delay time.Duration) error { return nil }
	t.Cleanup(func() { waitForRetry = origWait })

	var calls int32
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(bytes.NewReader([]byte("temporarily unavailable"))),
				Header:     http.Header{},
			}, nil
		}
		payload, _ := json.Marshal(echoResponse{Method: r.Method})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	var out echoResponse
	if _, err := client.Get(context.Background(), "/test", nil, &out); err != nil {
		t.Fatalf("get with retry: %v", err)
	}
	if out.Method != http.MethodGet {
		t.Fatalf("expected GET, got %s", out.Method)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestClientDoesNotRetryUnsafePostWithoutRequestID(t *testing.T) {
	client := NewClient("https://example.com", "token", 2*time.Second)
	origWait := waitForRetry
	waitForRetry = func(ctx context.Context, delay time.Duration) error { return nil }
	t.Cleanup(func() { waitForRetry = origWait })

	var calls int32
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(bytes.NewReader([]byte("temporarily unavailable"))),
			Header:     http.Header{},
		}, nil
	})}

	var out echoResponse
	_, err := client.Post(context.Background(), "/test", nil, map[string]any{"ok": true}, &out, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.RequestID != "" {
		t.Fatalf("unexpected request id: %q", apiErr.RequestID)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 attempt, got %d", calls)
	}
}

func TestClientRetriesPostWithRequestIDOn429(t *testing.T) {
	client := NewClient("https://example.com", "token", 2*time.Second)
	origWait := waitForRetry
	waitForRetry = func(ctx context.Context, delay time.Duration) error { return nil }
	t.Cleanup(func() { waitForRetry = origWait })

	var calls int32
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewReader([]byte("rate limited"))),
				Header:     http.Header{"Retry-After": []string{"0"}},
			}, nil
		}
		payload, _ := json.Marshal(echoResponse{Method: r.Method})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	var out echoResponse
	if _, err := client.Post(context.Background(), "/test", nil, map[string]any{"ok": true}, &out, true); err != nil {
		t.Fatalf("post with retry: %v", err)
	}
	if out.Method != http.MethodPost {
		t.Fatalf("expected POST, got %s", out.Method)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestRetryPolicyShouldRetryStatus(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		includeRequestID bool
		status           int
		want             bool
	}{
		{name: "get retries 503", method: http.MethodGet, status: http.StatusServiceUnavailable, want: true},
		{name: "get retries 500", method: http.MethodGet, status: http.StatusInternalServerError, want: true},
		{name: "get does not retry 400", method: http.MethodGet, status: http.StatusBadRequest, want: false},
		{name: "post retries with request id on 429", method: http.MethodPost, includeRequestID: true, status: http.StatusTooManyRequests, want: true},
		{name: "post does not retry without request id", method: http.MethodPost, includeRequestID: false, status: http.StatusTooManyRequests, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetryStatus(tt.method, tt.includeRequestID, tt.status)
			if got != tt.want {
				t.Fatalf("shouldRetryStatus()=%v want %v", got, tt.want)
			}
		})
	}
}

func TestRetryPolicyShouldRetryTransport(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		includeRequestID bool
		err              error
		want             bool
	}{
		{name: "get retries transport error", method: http.MethodGet, err: errors.New("boom"), want: true},
		{name: "post retries transport with request id", method: http.MethodPost, includeRequestID: true, err: errors.New("boom"), want: true},
		{name: "post no retry transport without request id", method: http.MethodPost, err: errors.New("boom"), want: false},
		{name: "no error does not retry", method: http.MethodGet, err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetryTransport(tt.method, tt.includeRequestID, tt.err)
			if got != tt.want {
				t.Fatalf("shouldRetryTransport()=%v want %v", got, tt.want)
			}
		})
	}
}

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		name       string
		attempt    int
		retryAfter string
		want       time.Duration
	}{
		{name: "retry-after seconds", retryAfter: "2", want: 2 * time.Second},
		{name: "retry-after capped to 3s", retryAfter: "99", want: 3 * time.Second},
		{name: "attempt0 backoff", attempt: 0, want: 200 * time.Millisecond},
		{name: "attempt1 backoff", attempt: 1, want: 400 * time.Millisecond},
		{name: "attempt2 backoff", attempt: 2, want: 800 * time.Millisecond},
		{name: "backoff capped", attempt: 10, want: 1200 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryDelay(tt.attempt, tt.retryAfter)
			if got != tt.want {
				t.Fatalf("retryDelay()=%v want %v", got, tt.want)
			}
		})
	}
}
