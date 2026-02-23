package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"

	"io"
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

func TestContractTaskDeleteStripsIDPrefix(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "delete", "--yes", "--id", "id:123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "123"`) {
		t.Fatalf("unexpected task delete dry-run output: %q", stdout.String())
	}
}

func TestContractTaskDeleteAcceptsTaskURLID(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "delete", "--yes", "--id", "https://app.todoist.com/app/task/call-mom-abc123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "abc123"`) {
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

func TestContractProjectDeleteAcceptsProjectURLID(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"project", "rm", "--id", "https://app.todoist.com/app/project/home-2203306141", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "2203306141"`) {
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

func TestContractUnknownCommandJSONError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"--json", "--quiet-json", "nope-command"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	line := strings.TrimSpace(stderr.String())
	if !strings.HasPrefix(line, "{") || !strings.Contains(line, `"unknown command: nope-command"`) {
		t.Fatalf("unexpected stderr payload: %q", line)
	}
}

func TestContractSubcommandHelpWithTrailingGlobalHelpFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"auth", "login", "--oauth", "--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "todoist auth login --oauth") {
		t.Fatalf("expected auth help output, got %q", got)
	}
	if !strings.Contains(got, "--oauth-listen") {
		t.Fatalf("expected login-specific oauth flags in help, got %q", got)
	}
}

func TestContractRootHelpWithoutCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "Agentic Todoist CLI") {
		t.Fatalf("expected root help output, got %q", got)
	}
	if !strings.Contains(got, "--fuzzy") || !strings.Contains(got, "--no-fuzzy") || !strings.Contains(got, "--accessible") {
		t.Fatalf("expected global accessibility/fuzzy flags in help, got %q", got)
	}
	if !strings.Contains(got, "Note for AI/LLM agents:") || !strings.Contains(got, "--quiet-json") {
		t.Fatalf("expected agent guidance note in root help, got %q", got)
	}
}

func TestContractNestedHelpAuthLogin(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"auth", "help", "login"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "Flags:") || !strings.Contains(got, "--oauth-token-url") {
		t.Fatalf("expected detailed auth login help, got %q", got)
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

func TestContractTaskCompleteBulkRequiresYes(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "complete", "--filter", "today", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "requires --yes") {
		t.Fatalf("expected requires --yes error, got %q", stderr.String())
	}
}

func TestContractTaskCompleteStripsIDPrefix(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "complete", "--id", "id:123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "123"`) {
		t.Fatalf("unexpected task complete dry-run output: %q", stdout.String())
	}
}

func TestContractTaskHelpMentionsStrictOnAddCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"help", "task"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `--strict belongs to top-level "todoist add", not "todoist task add".`) {
		t.Fatalf("expected strict guidance in task help, got %q", stdout.String())
	}
}

func TestContractTaskMoveBulkRejectsIDCombination(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "move", "--id", "123", "--filter", "today", "--project", "Home", "--yes", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "cannot be combined") {
		t.Fatalf("expected cannot be combined error, got %q", stderr.String())
	}
}

func TestContractFilterAddDryRunJSON(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"filter", "add", "--name", "Today", "--query", "today", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"action": "filter add"`) || !strings.Contains(got, `"query": "today"`) {
		t.Fatalf("unexpected filter add dry-run output: %q", got)
	}
}

func TestContractFilterUpdateRequiresFields(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"filter", "update", "Today", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "no fields to update") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
}

func TestContractCommentAddDryRunJSON(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"comment", "add", "--task", "123", "--content", "Need QA sign-off", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"action": "comment add"`) || !strings.Contains(got, `"task_id": "123"`) {
		t.Fatalf("unexpected comment add dry-run output: %q", got)
	}
}

func TestContractCommentUpdateRequiresFields(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"comment", "update", "--id", "c1", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "--id and --content are required") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
}

func TestContractLabelAddDryRunJSON(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"label", "add", "--name", "urgent", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "label add"`) || !strings.Contains(stdout.String(), `"name": "urgent"`) {
		t.Fatalf("unexpected label add dry-run output: %q", stdout.String())
	}
}

func TestContractLabelUpdateRequiresFields(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"label", "update", "--id", "l1", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "no fields to update") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
}

func TestContractSectionAddDryRunJSON(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"section", "add", "--name", "Backlog", "--project", "id:123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "section add"`) || !strings.Contains(stdout.String(), `"project_id": "123"`) {
		t.Fatalf("unexpected section add dry-run output: %q", stdout.String())
	}
}

func TestContractSectionUpdateRequiresFields(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"section", "update", "--id", "s1", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "--id and --name are required") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
}

func TestContractInboxAddDryRunDoesNotRequireInboxLookup(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"inbox", "add", "--content", "offline smoke", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "inbox add"`) || !strings.Contains(stdout.String(), `"content": "offline smoke"`) {
		t.Fatalf("unexpected inbox add dry-run output: %q", stdout.String())
	}
}

func TestContractTaskAddAssigneeIDRefDryRun(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "add", "--content", "Write docs", "--assignee", "id:123", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"assignee_id": "123"`) {
		t.Fatalf("unexpected assignee payload: %q", stdout.String())
	}
}

func TestContractTaskAddNaturalShorthandDryRun(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "add", "--content", "Buy milk #id:123 @errands p2 due:tomorrow", "--natural", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"content": "Buy milk"`) || !strings.Contains(got, `"project_id": "123"`) || !strings.Contains(got, `"labels": [`) || !strings.Contains(got, `"priority": 3`) || !strings.Contains(got, `"due_string": "tomorrow"`) {
		t.Fatalf("unexpected natural shorthand payload: %q", got)
	}
}

func TestContractTaskUpdateNaturalShorthandDryRun(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "update", "--id", "123", "--content", "Call mom p1 due:today", "--natural", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"content": "Call mom"`) || !strings.Contains(got, `"priority": 4`) || !strings.Contains(got, `"due_string": "today"`) {
		t.Fatalf("unexpected natural update payload: %q", got)
	}
}

func TestContractTaskAddAssigneeNameRequiresProject(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"task", "add", "--content", "Write docs", "--assignee", "Ada Lovelace", "--dry-run", "--json", "--quiet-json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "--project is required") {
		t.Fatalf("unexpected error: %q", stderr.String())
	}
}
