package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
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
