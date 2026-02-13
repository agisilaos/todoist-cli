package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestWriteWorkspaceListJSON(t *testing.T) {
	ctx := &Context{
		Stdout:    &bytes.Buffer{},
		Mode:      output.ModeJSON,
		RequestID: "req-1",
	}
	workspaces := []api.Workspace{
		{ID: "w1", Name: "Team", Role: "ADMIN", Plan: "pro"},
	}
	if err := writeWorkspaceList(ctx, workspaces); err != nil {
		t.Fatalf("writeWorkspaceList: %v", err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(got, `"id": "w1"`) || !strings.Contains(got, `"name": "Team"`) {
		t.Fatalf("unexpected workspace json output: %q", got)
	}
}

func TestWriteProjectCollaboratorsNDJSON(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Mode:   output.ModeNDJSON,
	}
	collaborators := []api.Collaborator{
		{ID: "u1", Name: "Ada", Email: "ada@example.com"},
	}
	if err := writeProjectCollaborators(ctx, collaborators, ""); err != nil {
		t.Fatalf("writeProjectCollaborators: %v", err)
	}
	got := strings.TrimSpace(ctx.Stdout.(*bytes.Buffer).String())
	if !strings.Contains(got, `"id":"u1"`) || !strings.Contains(got, `"email":"ada@example.com"`) {
		t.Fatalf("unexpected collaborator ndjson output: %q", got)
	}
}

func TestWorkspaceHelpIsAvailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"help", "workspace"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "todoist workspace list") {
		t.Fatalf("unexpected workspace help output: %q", stdout.String())
	}
}
