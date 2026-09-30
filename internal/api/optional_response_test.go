package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/authorization"
)

type optionalResponseBody struct {
	io.Reader
	closed bool
	bytes  int
}

func (b *optionalResponseBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.bytes += n
	return n, err
}

func (b *optionalResponseBody) Close() error { b.closed = true; return nil }

type brokenOptionalReader struct{}

func (brokenOptionalReader) Read(p []byte) (int, error) {
	return copy(p, `{"project_id":"untrusted"}`), io.ErrUnexpectedEOF
}

type canceledOptionalReader struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (r canceledOptionalReader) Read([]byte) (int, error) {
	r.cancel()
	return 0, r.ctx.Err()
}

func TestClientOptionalResponseCanceledAfterAcceptance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := &optionalResponseBody{Reader: canceledOptionalReader{ctx, cancel}}
	client := NewClient("https://example.test", "fixture", time.Second, authorization.Resolve(nil, "env", true))
	calls := 0
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: body, Header: http.Header{}}, nil
	})
	data, id, err := client.PostWithOptionalResponse(ctx, "/tasks/task/move", nil)
	if err != nil || ctx.Err() != context.Canceled || id == "" || len(data) != 0 || calls != 1 || !body.closed {
		t.Fatalf("accepted cancellation lost success: err=%v context=%v data=%q calls=%d closed=%v", err, ctx.Err(), data, calls, body.closed)
	}
}

func TestClientOptionalResponseAcceptanceBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		body io.Reader
		want string
	}{
		{"valid", strings.NewReader(`{"id":"task"}`), `{"id":"task"}`},
		{"empty", strings.NewReader(""), ""},
		{"malformed remains advisory", strings.NewReader("{broken"), "{broken"},
		{"incomplete", brokenOptionalReader{}, ""},
		{"oversized", strings.NewReader(strings.Repeat("x", maxOptionalResponseSize+100)), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &optionalResponseBody{Reader: tc.body}
			calls := 0
			client := NewClient("https://example.test", "fixture", time.Second, authorization.Resolve(nil, "env", true))
			client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Header.Get("X-Request-Id") == "" || r.Method != "POST" || r.URL.Path != "/tasks/task/move" {
					t.Errorf("request contract changed: %v", r)
				}
				return &http.Response{StatusCode: 200, Body: body, Header: http.Header{}}, nil
			})
			data, id, err := client.PostWithOptionalResponse(context.Background(), "/tasks/task/move", map[string]any{"project_id": "project"})
			if err != nil || id == "" || string(data) != tc.want || calls != 1 || !body.closed || body.bytes > maxOptionalResponseSize+1 {
				t.Fatalf("accepted response: data=%q id=%q err=%v calls=%d closed=%v bytes=%d", data, id, err, calls, body.closed, body.bytes)
			}
		})
	}
}

func TestClientOptionalResponsePreservesRetriesAndRejections(t *testing.T) {
	origWait := waitForRetry
	waitForRetry = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { waitForRetry = origWait })
	for _, tc := range []struct {
		name     string
		statuses []int
		wantErr  bool
	}{
		{"rejected", []int{403}, true},
		{"existing retry", []int{503, 200}, false},
		{"exhausted", []int{503, 503, 503}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient("https://example.test", "fixture", time.Second, authorization.Resolve(nil, "env", true))
			calls, requestID, payload := 0, "", ""
			client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				data, _ := io.ReadAll(r.Body)
				if calls == 0 {
					requestID, payload = r.Header.Get("X-Request-Id"), string(data)
				} else if requestID != r.Header.Get("X-Request-Id") || payload != string(data) {
					t.Error("retry changed identity or payload")
				}
				status := tc.statuses[calls]
				calls++
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"id":"task"}`)), Header: http.Header{}}, nil
			})
			data, id, err := client.PostWithOptionalResponse(context.Background(), "/tasks/task/move", map[string]any{"project_id": "project"})
			if (err != nil) != tc.wantErr || calls != len(tc.statuses) || id != requestID || id == "" {
				t.Fatalf("request contract: err=%v calls=%d id=%q", err, calls, id)
			}
			if tc.wantErr {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Status != tc.statuses[len(tc.statuses)-1] || len(data) != 0 {
					t.Fatalf("rejection lost identity: %v, %s", err, data)
				}
			}
		})
	}
}

func TestClientOptionalResponsePreservesAuthorizationAndStrictPost(t *testing.T) {
	report := authorization.Resolve([]byte(`{"version":1,"mode":"read-only","origin":"oauth-device","requested_scopes":["data:read"],"effective_scopes":["data:read"],"scope_evidence":"oauth-request"}`), "credentials", true)
	client := NewClient("https://example.test", "fixture", time.Second, report)
	calls := 0
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected") })
	if _, _, err := client.PostWithOptionalResponse(context.Background(), "/tasks/task/move", nil); err == nil || calls != 0 {
		t.Fatalf("authorization bypassed: %v calls=%d", err, calls)
	}
	client = NewClient("https://example.test", "fixture", time.Second, authorization.Resolve(nil, "env", true))
	for _, strict := range []bool{false, true} {
		body := &optionalResponseBody{Reader: brokenOptionalReader{}}
		client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: body, Header: http.Header{}}, nil
		})
		var out any
		if strict {
			out = &map[string]any{}
		}
		_, err := client.Post(context.Background(), "/tasks/task/move", nil, nil, out, true)
		if !body.closed || (err != nil) != strict || !strict && body.bytes != 0 {
			t.Fatalf("existing Post changed: strict=%v err=%v bytes=%d", strict, err, body.bytes)
		}
	}
}
