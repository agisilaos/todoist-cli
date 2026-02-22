package cli

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"

	"io"
)

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestResolveTaskRefLegacyNumericIDMessage(t *testing.T) {
	client := api.NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: testRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		payload := `{"error":"The ID provided was deprecated and cannot be used with this version of the API","error_tag":"V1_ID_CANNOT_BE_USED","http_code":400}`
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	ctx := &Context{
		Client: client,
		Config: config.Config{TimeoutSeconds: 10},
	}

	_, err := resolveTaskRef(ctx, "id:123")
	if err == nil {
		t.Fatalf("expected error")
	}
	var codeErr *CodeError
	if !errors.As(err, &codeErr) {
		t.Fatalf("expected CodeError, got %T", err)
	}
	if codeErr.Code != exitUsage {
		t.Fatalf("expected exitUsage, got %d", codeErr.Code)
	}
	if !strings.Contains(err.Error(), "legacy numeric task IDs") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestResolveTaskRefExplicitAlphaNumericID(t *testing.T) {
	client := api.NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: testRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/tasks/abc123XYZ" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		payload := `{"id":"abc123XYZ","content":"Call mom"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	ctx := &Context{
		Client: client,
		Config: config.Config{TimeoutSeconds: 10},
	}

	task, err := resolveTaskRef(ctx, "id:abc123XYZ")
	if err != nil {
		t.Fatalf("resolveTaskRef: %v", err)
	}
	if task.ID != "abc123XYZ" {
		t.Fatalf("unexpected task id: %q", task.ID)
	}
	if task.Content != "Call mom" {
		t.Fatalf("unexpected task content: %q", task.Content)
	}
}
