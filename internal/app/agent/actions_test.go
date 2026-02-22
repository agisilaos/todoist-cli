package agent

import (
	"net/http"
	"testing"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

func TestBuildActionRequestTaskAdd(t *testing.T) {
	req, err := BuildActionRequest(coreagent.Action{Type: "task_add", Content: "Do"}, ActionDeps{
		BuildTaskCreatePayload: func(in apptasks.MutationInput) (map[string]any, error) {
			return map[string]any{"content": in.Content}, nil
		},
	})
	if err != nil {
		t.Fatalf("BuildActionRequest: %v", err)
	}
	if req.Method != http.MethodPost || req.Path != "/tasks" || req.Body["content"] != "Do" {
		t.Fatalf("unexpected request: %#v", req)
	}
}

func TestBuildActionRequestSectionAddRequiresProject(t *testing.T) {
	_, err := BuildActionRequest(coreagent.Action{Type: "section_add", Name: "Backlog"}, ActionDeps{
		ResolveProjectSelector: func(explicitID, reference string) (string, error) { return "", nil },
	})
	if err == nil || err.Error() != "section_add requires project or project_id" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildActionRequestCommentUpdate(t *testing.T) {
	req, err := BuildActionRequest(coreagent.Action{Type: "comment_update", CommentID: "c1", Content: "edited"}, ActionDeps{})
	if err != nil {
		t.Fatalf("BuildActionRequest: %v", err)
	}
	if req.Path != "/comments/c1" || req.Body["content"] != "edited" {
		t.Fatalf("unexpected request: %#v", req)
	}
}
