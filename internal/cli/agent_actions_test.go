package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

type capturedRequest struct {
	method string
	path   string
	body   map[string]any
}

func TestApplyActionExecutionMatrix(t *testing.T) {
	tests := []struct {
		name       string
		action     Action
		wantMethod string
		wantPath   string
		wantBody   map[string]any
	}{
		{name: "task_add", action: Action{Type: "task_add", Content: "Do", ProjectID: "p1", SectionID: "s1", Priority: 4}, wantMethod: http.MethodPost, wantPath: "/tasks", wantBody: map[string]any{"content": "Do", "project_id": "p1", "section_id": "s1", "priority": 4}},
		{name: "task_update", action: Action{Type: "task_update", TaskID: "t1", Content: "Rename"}, wantMethod: http.MethodPost, wantPath: "/tasks/t1", wantBody: map[string]any{"content": "Rename"}},
		{name: "task_move", action: Action{Type: "task_move", TaskID: "t1", ProjectID: "p2", SectionID: "s2"}, wantMethod: http.MethodPost, wantPath: "/tasks/t1/move", wantBody: map[string]any{"project_id": "p2", "section_id": "s2"}},
		{name: "task_complete", action: Action{Type: "task_complete", TaskID: "t1"}, wantMethod: http.MethodPost, wantPath: "/tasks/t1/close"},
		{name: "task_reopen", action: Action{Type: "task_reopen", TaskID: "t1"}, wantMethod: http.MethodPost, wantPath: "/tasks/t1/reopen"},
		{name: "task_delete", action: Action{Type: "task_delete", TaskID: "t1"}, wantMethod: http.MethodDelete, wantPath: "/tasks/t1"},
		{name: "project_add", action: Action{Type: "project_add", Name: "New", Color: "red"}, wantMethod: http.MethodPost, wantPath: "/projects", wantBody: map[string]any{"name": "New", "color": "red"}},
		{name: "project_update", action: Action{Type: "project_update", ProjectID: "p1", Name: "Upd"}, wantMethod: http.MethodPost, wantPath: "/projects/p1", wantBody: map[string]any{"name": "Upd"}},
		{name: "project_archive", action: Action{Type: "project_archive", ProjectID: "p1"}, wantMethod: http.MethodPost, wantPath: "/projects/p1/archive"},
		{name: "project_unarchive", action: Action{Type: "project_unarchive", ProjectID: "p1"}, wantMethod: http.MethodPost, wantPath: "/projects/p1/unarchive"},
		{name: "project_delete", action: Action{Type: "project_delete", ProjectID: "p1"}, wantMethod: http.MethodDelete, wantPath: "/projects/p1"},
		{name: "section_add", action: Action{Type: "section_add", Name: "Backlog", ProjectID: "p1", Order: 7}, wantMethod: http.MethodPost, wantPath: "/sections", wantBody: map[string]any{"name": "Backlog", "project_id": "p1", "order": 7}},
		{name: "section_update", action: Action{Type: "section_update", SectionID: "s1", Name: "Current"}, wantMethod: http.MethodPost, wantPath: "/sections/s1", wantBody: map[string]any{"name": "Current"}},
		{name: "section_delete", action: Action{Type: "section_delete", SectionID: "s1"}, wantMethod: http.MethodDelete, wantPath: "/sections/s1"},
		{name: "label_add", action: Action{Type: "label_add", Name: "urgent", Favorite: boolPtr(true)}, wantMethod: http.MethodPost, wantPath: "/labels", wantBody: map[string]any{"name": "urgent", "is_favorite": true}},
		{name: "label_update", action: Action{Type: "label_update", LabelID: "l1", Name: "focus"}, wantMethod: http.MethodPost, wantPath: "/labels/l1", wantBody: map[string]any{"name": "focus"}},
		{name: "label_delete", action: Action{Type: "label_delete", LabelID: "l1"}, wantMethod: http.MethodDelete, wantPath: "/labels/l1"},
		{name: "comment_add", action: Action{Type: "comment_add", Content: "hello", ProjectID: "p1"}, wantMethod: http.MethodPost, wantPath: "/comments", wantBody: map[string]any{"content": "hello", "project_id": "p1"}},
		{name: "comment_update", action: Action{Type: "comment_update", CommentID: "c1", Content: "edited"}, wantMethod: http.MethodPost, wantPath: "/comments/c1", wantBody: map[string]any{"content": "edited"}},
		{name: "comment_delete", action: Action{Type: "comment_delete", CommentID: "c1"}, wantMethod: http.MethodDelete, wantPath: "/comments/c1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []capturedRequest
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := capturedRequest{method: r.Method, path: r.URL.Path}
				if r.Method == http.MethodPost {
					data, _ := io.ReadAll(r.Body)
					if len(strings.TrimSpace(string(data))) > 0 {
						_ = json.Unmarshal(data, &call.body)
					}
				}
				calls = append(calls, call)
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			ctx := &Context{
				Stdout: &bytes.Buffer{},
				Token:  "token",
				Client: api.NewClient(ts.URL, "token", time.Second),
				Config: config.Config{TimeoutSeconds: 2},
			}
			if err := applyAction(ctx, tt.action); err != nil {
				t.Fatalf("applyAction: %v", err)
			}
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d (%#v)", len(calls), calls)
			}
			if calls[0].method != tt.wantMethod {
				t.Fatalf("method: got %s want %s", calls[0].method, tt.wantMethod)
			}
			if calls[0].path != tt.wantPath {
				t.Fatalf("path: got %s want %s", calls[0].path, tt.wantPath)
			}
			if !assertBodySubset(calls[0].body, tt.wantBody) {
				t.Fatalf("body mismatch: got %#v want subset %#v", calls[0].body, tt.wantBody)
			}
		})
	}
}

func TestApplyActionValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		wantErr string
	}{
		{name: "task_update_missing_id", action: Action{Type: "task_update"}, wantErr: "task_update requires task_id"},
		{name: "task_move_missing_id", action: Action{Type: "task_move"}, wantErr: "task_move requires task_id"},
		{name: "project_add_missing_name", action: Action{Type: "project_add"}, wantErr: "project_add requires name"},
		{name: "project_update_missing_id", action: Action{Type: "project_update"}, wantErr: "project_update requires project_id"},
		{name: "section_add_missing_project", action: Action{Type: "section_add", Name: "N"}, wantErr: "section_add requires project or project_id"},
		{name: "section_update_missing_fields", action: Action{Type: "section_update"}, wantErr: "section_update requires section_id and name"},
		{name: "label_update_missing_id", action: Action{Type: "label_update"}, wantErr: "label_update requires label_id"},
		{name: "comment_add_missing_content", action: Action{Type: "comment_add"}, wantErr: "comment_add requires content"},
		{name: "comment_update_missing_fields", action: Action{Type: "comment_update"}, wantErr: "comment_update requires comment_id and content"},
		{name: "unsupported_type", action: Action{Type: "unknown_action"}, wantErr: "unsupported action type"},
	}

	ctx := &Context{Token: "token", Config: config.Config{TimeoutSeconds: 2}, Client: api.NewClient("https://example.com", "token", time.Second)}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := applyAction(ctx, tt.action)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

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

func assertBodySubset(got map[string]any, want map[string]any) bool {
	if len(want) == 0 {
		return len(got) == 0 || got == nil
	}
	if got == nil {
		return false
	}
	for k, v := range want {
		gv, ok := got[k]
		if !ok {
			return false
		}
		switch expected := v.(type) {
		case int:
			if !reflect.DeepEqual(gv, float64(expected)) {
				return false
			}
		default:
			if !reflect.DeepEqual(gv, expected) {
				return false
			}
		}
	}
	return true
}

func boolPtr(v bool) *bool {
	return &v
}
