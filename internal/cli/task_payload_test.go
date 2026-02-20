package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestBuildTaskMovePayloadUsesExplicitIDs(t *testing.T) {
	ctx := &Context{}
	body, err := buildTaskMovePayload(ctx, "p123", "", "s123", "", "")
	if err != nil {
		t.Fatalf("buildTaskMovePayload: %v", err)
	}
	if body["project_id"] != "p123" {
		t.Fatalf("expected explicit project_id, got %#v", body)
	}
	if body["section_id"] != "s123" {
		t.Fatalf("expected explicit section_id, got %#v", body)
	}
}

func TestBuildTaskCreatePayloadResolvesProjectError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	_, err := buildTaskCreatePayload(ctx, taskMutationInput{Content: "x", ProjectRef: "Home"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
