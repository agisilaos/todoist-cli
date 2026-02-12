package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestContractPlannerAcceptsGlobalFlagsAfterCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"planner", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"planner_cmd"`) {
		t.Fatalf("unexpected planner output: %q", stdout.String())
	}
}

func TestContractAddSupportsInterspersedFlags(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"add", "Buy milk", "--project", "Home", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"text": "Buy milk #Home"`) {
		t.Fatalf("unexpected add dry-run payload: %q", got)
	}
	if strings.Contains(got, "--project") {
		t.Fatalf("project flag leaked into content: %q", got)
	}
}

func TestContractTaskDeleteSupportsInterspersedFlags(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "delete", "--yes", "--id", "123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "123"`) {
		t.Fatalf("unexpected task delete dry-run output: %q", stdout.String())
	}
}

func TestContractTaskDeleteAliasRm(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "rm", "--yes", "--id", "123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "task delete"`) {
		t.Fatalf("unexpected task rm dry-run output: %q", stdout.String())
	}
}

func TestContractProjectDeleteAliasRm(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"project", "rm", "--id", "p1", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "project delete"`) {
		t.Fatalf("unexpected project rm dry-run output: %q", stdout.String())
	}
}

func TestContractSchemaWritesCleanStdout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"schema", "--name", "task_item_ndjson"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "[") {
		t.Fatalf("expected JSON array output, got %q", stdout.String())
	}
}

func TestContractQuietJSONErrorsAreSingleLine(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "delete", "--id", "123", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single-line stderr json, got %q", stderr.String())
	}
	if !strings.HasPrefix(lines[0], "{") {
		t.Fatalf("expected json object line, got %q", lines[0])
	}
}

func TestContractNDJSONWritersForAllLists(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: io.Discard, Mode: output.ModeNDJSON}
	if err := writeTaskList(ctx, []api.Task{{ID: "t1", Content: "Task", ProjectID: "p1", Priority: 1}}, "", false); err != nil {
		t.Fatalf("task ndjson: %v", err)
	}
	if !strings.Contains(ctx.Stdout.(*bytes.Buffer).String(), `"id":"t1"`) {
		t.Fatalf("unexpected task ndjson output: %q", ctx.Stdout.(*bytes.Buffer).String())
	}

	ctx.Stdout = &bytes.Buffer{}
	if err := writeProjectList(ctx, []api.Project{{ID: "p1", Name: "Proj"}}, ""); err != nil {
		t.Fatalf("project ndjson: %v", err)
	}
	if !strings.Contains(ctx.Stdout.(*bytes.Buffer).String(), `"id":"p1"`) {
		t.Fatalf("unexpected project ndjson output: %q", ctx.Stdout.(*bytes.Buffer).String())
	}

	ctx.Stdout = &bytes.Buffer{}
	if err := writeSectionList(ctx, []api.Section{{ID: "s1", Name: "Sec", ProjectID: "p1"}}, ""); err != nil {
		t.Fatalf("section ndjson: %v", err)
	}
	if !strings.Contains(ctx.Stdout.(*bytes.Buffer).String(), `"id":"s1"`) {
		t.Fatalf("unexpected section ndjson output: %q", ctx.Stdout.(*bytes.Buffer).String())
	}

	ctx.Stdout = &bytes.Buffer{}
	if err := writeLabelList(ctx, []api.Label{{ID: "l1", Name: "urgent"}}, ""); err != nil {
		t.Fatalf("label ndjson: %v", err)
	}
	if !strings.Contains(ctx.Stdout.(*bytes.Buffer).String(), `"id":"l1"`) {
		t.Fatalf("unexpected label ndjson output: %q", ctx.Stdout.(*bytes.Buffer).String())
	}

	ctx.Stdout = &bytes.Buffer{}
	if err := writeCommentList(ctx, []api.Comment{{ID: "c1", Content: "hello"}}, ""); err != nil {
		t.Fatalf("comment ndjson: %v", err)
	}
	if !strings.Contains(ctx.Stdout.(*bytes.Buffer).String(), `"id":"c1"`) {
		t.Fatalf("unexpected comment ndjson output: %q", ctx.Stdout.(*bytes.Buffer).String())
	}
}
