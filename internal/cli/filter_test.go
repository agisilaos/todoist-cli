package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestWriteFilterListNDJSON(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Mode: output.ModeNDJSON}
	filters := []api.Filter{{ID: "f1", Name: "Today", Query: "today"}}
	if err := writeFilterList(ctx, filters); err != nil {
		t.Fatalf("writeFilterList: %v", err)
	}
	got := strings.TrimSpace(ctx.Stdout.(*bytes.Buffer).String())
	if !strings.Contains(got, `"id":"f1"`) || !strings.Contains(got, `"query":"today"`) {
		t.Fatalf("unexpected ndjson output: %q", got)
	}
}

func TestFilterHelpIsAvailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"help", "filter"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "todoist filter list") {
		t.Fatalf("unexpected filter help output: %q", stdout.String())
	}
}
