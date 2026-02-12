package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestParseGlobalFlagsConflicts(t *testing.T) {
	opts, _, err := parseGlobalFlags([]string{"--json", "--plain"}, nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := output.DetectMode(opts.JSON, opts.Plain, false, true); err == nil {
		t.Fatalf("expected error for --json and --plain")
	}
	_, _, err = parseGlobalFlags([]string{"-q", "-v"}, nil)
	if err == nil {
		t.Fatalf("expected error for --quiet and --verbose")
	}
}

func TestParseGlobalFlagsValues(t *testing.T) {
	opts, rest, err := parseGlobalFlags([]string{"--timeout", "5", "task", "list"}, nil)
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if opts.TimeoutSec != 5 {
		t.Fatalf("expected timeout 5, got %d", opts.TimeoutSec)
	}
	if len(rest) != 2 || rest[0] != "task" {
		t.Fatalf("unexpected rest args: %#v", rest)
	}
}

func TestParseGlobalFlagsInterspersed(t *testing.T) {
	opts, rest, err := parseGlobalFlags([]string{"planner", "--json", "--quiet-json", "--profile", "work"}, nil)
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if !opts.JSON {
		t.Fatalf("expected json true")
	}
	if !opts.QuietJSON {
		t.Fatalf("expected quiet-json true")
	}
	if opts.Profile != "work" {
		t.Fatalf("expected profile work, got %q", opts.Profile)
	}
	if len(rest) != 1 || rest[0] != "planner" {
		t.Fatalf("unexpected rest args: %#v", rest)
	}
}

func TestTopLevelPlannerCommandIsDispatched(t *testing.T) {
	var out bytes.Buffer
	code := Execute([]string{"planner", "--json"}, &out, io.Discard)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	if !strings.Contains(out.String(), "\"planner_cmd\"") {
		t.Fatalf("unexpected planner output: %q", out.String())
	}
}
