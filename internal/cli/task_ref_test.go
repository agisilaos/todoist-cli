package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
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
