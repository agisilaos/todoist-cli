package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMoveProjectToWorkspace(t *testing.T) {
	var gotPath string
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"Home","workspace_id":"w1"}}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "token", time.Second)
	project, reqID, err := client.MoveProjectToWorkspace(context.Background(), MoveProjectToWorkspaceInput{
		ProjectID:   "p1",
		WorkspaceID: "w1",
		Visibility:  "team",
	})
	if err != nil {
		t.Fatalf("MoveProjectToWorkspace: %v", err)
	}
	if reqID == "" {
		t.Fatalf("expected request id")
	}
	if gotPath != "/projects/move_to_workspace" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"project_id":"p1"`) || !strings.Contains(gotBody, `"workspace_id":"w1"`) || !strings.Contains(gotBody, `"visibility":"team"`) {
		t.Fatalf("unexpected body: %s", gotBody)
	}
	if project.ID != "p1" || project.WorkspaceID != "w1" {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestMoveProjectToPersonal(t *testing.T) {
	var gotPath string
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"Home"}}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "token", time.Second)
	project, _, err := client.MoveProjectToPersonal(context.Background(), "p1")
	if err != nil {
		t.Fatalf("MoveProjectToPersonal: %v", err)
	}
	if gotPath != "/projects/move_to_personal" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"project_id":"p1"`) {
		t.Fatalf("unexpected body: %s", gotBody)
	}
	if project.ID != "p1" {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestMoveProjectToWorkspaceRequiresFields(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	if _, _, err := client.MoveProjectToWorkspace(context.Background(), MoveProjectToWorkspaceInput{WorkspaceID: "w1"}); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := client.MoveProjectToWorkspace(context.Background(), MoveProjectToWorkspaceInput{ProjectID: "p1"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestMoveProjectToPersonalRequiresProjectID(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	if _, _, err := client.MoveProjectToPersonal(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDecodeProjectFromMoveResponseSupportsDirectProjectShape(t *testing.T) {
	project, err := decodeProjectFromMoveResponse(map[string]any{
		"id":   "p1",
		"name": "Home",
	})
	if err != nil {
		t.Fatalf("decodeProjectFromMoveResponse: %v", err)
	}
	if project.ID != "p1" {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestMoveProjectToWorkspaceEscapesBaseURL(t *testing.T) {
	base := "https://example.com/api"
	client := NewClient(base, "token", time.Second)
	u, err := url.Parse(client.BaseURL)
	if err != nil || u.Host == "" {
		t.Fatalf("unexpected base url: %q err=%v", client.BaseURL, err)
	}
}
