//go:build darwin || linux

package cli

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func taskAmbiguityTTY(t *testing.T, input string) *os.File {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("PTY unavailable: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })
	name, err := taskAmbiguityPTYName(master.Fd())
	if err != nil {
		t.Fatalf("prepare PTY: %v", err)
	}
	slave, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open PTY slave: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	if !isTTYReader(slave) {
		t.Fatal("PTY slave must exercise the interactive selection path")
	}
	if _, err := master.Write([]byte(input)); err != nil {
		t.Fatalf("write PTY input: %v", err)
	}
	return slave
}

func TestTaskAmbiguityTTYDefaultsToHumanOutput(t *testing.T) {
	tty := taskAmbiguityTTY(t, "\n")
	if !output.IsTTY(tty) {
		t.Fatal("actual PTY must be recognized as a terminal")
	}
	mode, err := output.DetectMode(false, false, false, false, output.IsTTY(tty))
	if err != nil || mode != output.ModeHuman {
		t.Fatalf("actual terminal should default to human output: %s, %v", mode, err)
	}
}

func TestTaskAmbiguityInteractiveChoiceRequiresExplicitNumber(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		selected    bool
		invalid     bool
	}{
		{"select second", "2\n", true, false},
		{"Enter cancels", "\n", false, false},
		{"invalid number", "3\n", false, true},
		{"invalid text", "second\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, requests := newTaskAmbiguityContext(t, duplicateTaskResponse, false)
			ctx.Stdin = taskAmbiguityTTY(t, tc.input)
			err := taskComplete(ctx, []string{"Review"})
			wantCalls := []string{"GET /tasks", "GET /projects", "GET /sections"}
			if tc.selected {
				if err != nil || ctx.Stdout.(*bytes.Buffer).String() != "Completion accepted\nTask before completion: Review\nID: second\n" {
					t.Fatalf("explicit second choice did not complete the intended task: %v, %q", err, ctx.Stdout)
				}
				wantCalls = append(wantCalls, "POST /tasks/second/close")
			} else {
				if toExitCode(err) != exitUsage || ctx.Stdout.(*bytes.Buffer).Len() != 0 {
					t.Fatalf("cancelled/invalid choice proceeded: %v, %q", err, ctx.Stdout)
				}
				if tc.invalid {
					if !strings.Contains(err.Error(), "invalid selection for ambiguous match") {
						t.Fatalf("invalid choice behavior changed: %v", err)
					}
				} else {
					var ambiguous *AmbiguousMatchError
					if !errors.As(err, &ambiguous) {
						t.Fatalf("cancellation must retain the ambiguity error: %v", err)
					}
				}
			}
			if got := requests.snapshot(); !reflect.DeepEqual(got, wantCalls) {
				t.Fatalf("unexpected reads or mutation targets: %v, want %v", got, wantCalls)
			}
			got := ctx.Stderr.(*bytes.Buffer).String()
			for _, want := range []string{"1) Review (id:first)", "2) Review (id:second)", "Project: Work", "Section: Launch", "Due 2026-09-30", "Project: Home", "No due date", "Choose number (or press Enter to cancel): "} {
				if !strings.Contains(got, want) {
					t.Errorf("selection lacks %q: %s", want, got)
				}
			}
		})
	}
}

func TestTaskAmbiguityNoInputAvoidsEnrichmentEvenWithTTY(t *testing.T) {
	ctx, requests := newTaskAmbiguityContext(t, duplicateTaskResponse, false)
	ctx.Stdin = taskAmbiguityTTY(t, "2\n")
	ctx.Global.NoInput = true
	err := taskComplete(ctx, []string{"Review"})
	var ambiguous *AmbiguousMatchError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("--no-input selected despite TTY: %v", err)
	}
	if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks"}) {
		t.Fatalf("--no-input enriched or mutated tasks: %v", got)
	}
	if ctx.Stdout.(*bytes.Buffer).Len() != 0 || ctx.Stderr.(*bytes.Buffer).Len() != 0 {
		t.Fatalf("--no-input prompted or reported success: stdout=%q stderr=%q", ctx.Stdout, ctx.Stderr)
	}
}

func TestTaskAmbiguityFuzzyChoiceUsesSelectedIDRatherThanHighestRank(t *testing.T) {
	response := `{"results":[{"id":"second","content":"Annual review","project_id":"home"},{"id":"first","content":"Review","project_id":"work"}],"next_cursor":""}`
	ctx, requests := newTaskAmbiguityContext(t, response, false)
	ctx.Stdin = taskAmbiguityTTY(t, "2\n")
	ctx.Fuzzy = true
	if err := taskComplete(ctx, []string{"rev"}); err != nil {
		t.Fatalf("explicit fuzzy choice failed: %v", err)
	}
	got := ctx.Stderr.(*bytes.Buffer).String()
	if !strings.Contains(got, "1) Review (id:first)") || !strings.Contains(got, "2) Annual review (id:second)") {
		t.Fatalf("fuzzy choice order changed: %s", got)
	}
	if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks", "GET /projects", "POST /tasks/second/close"}) {
		t.Fatalf("ranking overrode the explicit mutation target: %v", got)
	}
	if got := ctx.Stdout.(*bytes.Buffer).String(); got != "Completion accepted\nTask before completion: Annual review\nID: second\n" {
		t.Fatalf("reported the wrong mutation target: %q", got)
	}
}

func TestTaskAmbiguityShowsUsefulContextWithoutFetchingParents(t *testing.T) {
	response := `{"results":[{"id":"first","content":"Review","project_id":"work","parent_id":"parent","labels":["legal, client","urgent"],"due":{"date":"2026-09-30","datetime":"2026-09-30T10:00:00+02:00","timezone":"Europe/Berlin","is_recurring":true,"string":"every weekday"}},{"id":"second","content":"Review","project_id":"home","parent_id":"missing-parent"},{"id":"parent","content":"Website redesign","project_id":"work","due":null}],"next_cursor":""}`
	ctx, requests := newTaskAmbiguityContext(t, response, false)
	ctx.Stdin = taskAmbiguityTTY(t, "2\n")
	task, err := resolveTaskRef(ctx, "Review")
	if err != nil || task.ID != "second" {
		t.Fatalf("context changed selected identity: %#v, %v", task, err)
	}
	got := ctx.Stderr.(*bytes.Buffer).String()
	for _, want := range []string{"2026-09-30T10:00:00+02:00 (Europe/Berlin)", "Repeats: every weekday", "Parent: Website redesign (id:parent)", `Labels: "legal, client", "urgent"`, "Parent: missing-parent (name unavailable)", "Due unavailable"} {
		if !strings.Contains(got, want) {
			t.Errorf("selection lacks %q: %s", want, got)
		}
	}
	if strings.Contains(got, "3) Website redesign") {
		t.Fatalf("nonmatching parent was added to the choices: %s", got)
	}
	if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks", "GET /projects"}) {
		t.Fatalf("context fetched unused sections or parent tasks: %v", got)
	}
}

func TestTaskAmbiguityMetadataFailureRetainsChoiceAndIDs(t *testing.T) {
	ctx, requests := newTaskAmbiguityContext(t, duplicateTaskResponse, true)
	ctx.Stdin = taskAmbiguityTTY(t, "2\n")
	task, err := resolveTaskRef(ctx, "Review")
	if err != nil || task.ID != "second" {
		t.Fatalf("optional enrichment prevented the intended choice: %#v, %v", task, err)
	}
	got := ctx.Stderr.(*bytes.Buffer).String()
	for _, want := range []string{"Project: work (name unavailable)", "Section: launch (name unavailable)", "Project: home (name unavailable)", "1) Review (id:first)", "2) Review (id:second)"} {
		if !strings.Contains(got, want) {
			t.Errorf("failed enrichment lacks fallback %q: %s", want, got)
		}
	}
	if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks", "GET /projects", "GET /sections"}) {
		t.Fatalf("failed enrichment fetched or mutated unexpected resources: %v", got)
	}
}

func TestTaskAmbiguityContextCannotForgeSelectionRows(t *testing.T) {
	ctx, _ := newTaskAmbiguityContext(t, duplicateTaskResponse, false)
	ctx.Stdin = taskAmbiguityTTY(t, "2\n")
	tasks := []api.Task{
		{ID: "first", Content: "Review\n2) Forged\x1b[2J", ProjectID: "work", ParentID: "parent", Labels: []string{"urgent\nChoose 1"}},
		{ID: "second", Content: "Review", ProjectID: "home", DueReturned: true},
		{ID: "parent", Content: "Parent\x1b[2J"},
	}
	chosen, ok, err := promptAmbiguousTaskChoice(ctx, "Review", taskCandidates(tasks[:2]), tasks)
	if err != nil || !ok || chosen != "second" {
		t.Fatalf("unsafe text changed selection: %q, %v, %v", chosen, ok, err)
	}
	got := ctx.Stderr.(*bytes.Buffer).String()
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\n2) Forged") || strings.Contains(got, "\nChoose 1") {
		t.Fatalf("remote text forged terminal content: %q", got)
	}
	for _, want := range []string{`Review\n2) Forged\x1b[2J`, `Parent\x1b[2J`, `urgent\nChoose 1`} {
		if !strings.Contains(got, want) {
			t.Errorf("selection lost escaped identifying text %q: %q", want, got)
		}
	}
}
