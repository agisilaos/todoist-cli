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
