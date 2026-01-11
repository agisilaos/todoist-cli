package cli

import (
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestParseGlobalFlagsConflicts(t *testing.T) {
	opts, _, err := parseGlobalFlags([]string{"--json", "--plain"}, nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := output.DetectMode(opts.JSON, opts.Plain, true); err == nil {
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
