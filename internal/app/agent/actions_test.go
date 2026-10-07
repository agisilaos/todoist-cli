package agent

import (
	"errors"
	"net/http"
	"testing"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
)

func TestBuildActionRequestTaskAdd(t *testing.T) {
	req, err := BuildActionRequest(coreagent.Action{Type: "task_add", Content: "Do"}, ActionDeps{
		TaskSelectors: actionSelectors{},
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

type actionSelectors struct{ err error }

func (s actionSelectors) ResolveProjectSelector(id, ref string) (string, error) { return id, s.err }
func (s actionSelectors) ResolveSectionSelector(id, ref, project string) (string, error) {
	return id, s.err
}
func (s actionSelectors) ResolveAssigneeSelector(id, ref, project, task string) (string, error) {
	return id, s.err
}

func TestBuildActionRequestTaskMutationResolverFailures(t *testing.T) {
	want := errors.New("selector lookup failed")
	for _, kind := range []string{"task_add", "task_update", "task_move"} {
		t.Run(kind, func(t *testing.T) {
			_, err := BuildActionRequest(coreagent.Action{Type: kind, TaskID: "t1", Content: "Do", ProjectID: "p1"}, ActionDeps{TaskSelectors: actionSelectors{err: want}})
			if !errors.Is(err, want) {
				t.Fatalf("resolver failure lost: %v", err)
			}
		})
	}
}
