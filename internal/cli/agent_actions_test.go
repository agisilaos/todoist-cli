package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestApplyActionCommentAddAcceptsProjectID(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	action := Action{Type: "comment_add", Content: "hello", ProjectID: "p123"}
	if err := applyAction(ctx, action); err != nil {
		t.Fatalf("applyAction: %v", err)
	}
	if gotPath != "/comments" {
		t.Fatalf("expected /comments, got %q", gotPath)
	}
	if gotBody["project_id"] != "p123" {
		t.Fatalf("expected project_id p123, got %#v", gotBody)
	}
}

func TestApplyActionTaskAddReturnsResolverError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"server error"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	err := applyAction(ctx, Action{Type: "task_add", Content: "x", Project: "Home"})
	if err == nil {
		t.Fatalf("expected resolver error")
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
}

func TestValidateActionFieldsCommentAddAllowsProjectAlias(t *testing.T) {
	err := validateActionFields(Action{Type: "comment_add", Content: "hello", Project: "Home"})
	if err != nil {
		t.Fatalf("validateActionFields: %v", err)
	}
}

func TestValidateActionFieldsTaskMoveAllowsProjectID(t *testing.T) {
	err := validateActionFields(Action{Type: "task_move", TaskID: "t1", ProjectID: "p1"})
	if err != nil {
		t.Fatalf("validateActionFields: %v", err)
	}
}
