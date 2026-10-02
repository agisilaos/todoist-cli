package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"

	"io"
)

func TestParseGlobalFlagsConflicts(t *testing.T) {
	opts, _, err := parseGlobalFlags([]string{"--json", "--plain"}, nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := output.DetectMode(opts.JSON, opts.Plain, false, false, true); err == nil {
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
	opts, rest, err := parseGlobalFlags([]string{"planner", "--json", "--quiet-json", "--profile", "work", "--progress-jsonl=events.jsonl", "--accessible"}, nil)
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
	if opts.ProgressJSONL != "events.jsonl" {
		t.Fatalf("expected progress-jsonl path, got %q", opts.ProgressJSONL)
	}
	if !opts.Accessible {
		t.Fatalf("expected accessible true")
	}
	if len(rest) != 1 || rest[0] != "planner" {
		t.Fatalf("unexpected rest args: %#v", rest)
	}
}

func TestParseGlobalBooleanHelpValues(t *testing.T) {
	for _, prefix := range []string{"--help=", "-h="} {
		for _, value := range []string{"1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False"} {
			want := value == "1" || strings.EqualFold(value, "t") || strings.EqualFold(value, "true")
			opts, rest, err := parseGlobalFlags([]string{"task", "view", prefix + value}, nil)
			if err != nil || opts.Help != want || !reflect.DeepEqual(rest, []string{"task", "view"}) {
				t.Errorf("help %s: %+v %v %v", prefix+value, opts, rest, err)
			}
		}
	}
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"--help=true", "-h=false"}, false},
		{[]string{"--help=false", "-h=true"}, true},
		{[]string{"--help", "-h=false"}, false},
	} {
		opts, _, err := parseGlobalFlags(tc.args, nil)
		if err != nil || opts.Help != tc.want {
			t.Errorf("help order %v: %+v %v", tc.args, opts, err)
		}
	}
	for _, args := range [][]string{
		{"task", "add", "--content", "--help=true"},
		{"task", "add", "--content", "-h=true"},
		{"task", "add", "--content=--help=true"},
		{"task", "list", "--id", "--help=true"},
	} {
		opts, rest, err := parseGlobalFlags(args, nil)
		if err != nil || opts.Help || !reflect.DeepEqual(rest, args) {
			t.Errorf("help-shaped value consumed: %v => %+v %v %v", args, opts, rest, err)
		}
	}
	opts, rest, err := parseGlobalFlags([]string{"task", "view", "--", "--help=true"}, nil)
	if err != nil || opts.Help || !reflect.DeepEqual(rest, []string{"task", "view", "--help=true"}) {
		t.Errorf("help after terminator consumed: %+v %v %v", opts, rest, err)
	}
}

func TestTopLevelPlannerCommandIsDispatched(t *testing.T) {
	var out bytes.Buffer
	code := executeTest([]string{"planner", "--json"}, &out, io.Discard)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	if !strings.Contains(out.String(), "\"planner_cmd\"") {
		t.Fatalf("unexpected planner output: %q", out.String())
	}
}
