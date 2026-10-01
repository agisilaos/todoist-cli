package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

const detailTaskJSON = `{"id":"task-long-exact-id","content":"Prepare launch checklist","description":"First paragraph.\n\n- Confirm owner\n    literal code  spacing","project_id":"project-A","section_id":"section-B","parent_id":null,"labels":["work","a,b"],"priority":3,"checked":false,"due":{"date":"2026-09-29","datetime":"2026-09-29T16:30:00+02:00","timezone":"Europe/Berlin","string":"every Tuesday at 16:30","is_recurring":true},"added_at":"2026-09-20T09:15:00Z","updated_at":"2026-09-28T13:45:00Z","completed_at":null,"note_count":2}`

func detailTask(t *testing.T, value string) api.Task {
	t.Helper()
	var task api.Task
	if err := json.Unmarshal([]byte(value), &task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestTaskDetailDefaultAndFull(t *testing.T) {
	var defaultOutput string
	for _, full := range []bool{false, true} {
		ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("mutation: %s", r.Method)
			}
			switch r.URL.Path {
			case "/tasks/task-long-exact-id":
				io.WriteString(w, detailTaskJSON)
			case "/projects":
				io.WriteString(w, `{"results":[{"id":"project-A","name":"Work"}]}`)
			case "/sections":
				if r.URL.Query().Get("project_id") != "project-A" {
					t.Errorf("unscoped sections: %s", r.URL)
				}
				io.WriteString(w, `{"results":[{"id":"section-B","project_id":"project-A","name":"Launch"}]}`)
			default:
				t.Errorf("unexpected request: %s", r.URL)
				http.NotFound(w, r)
			}
		})
		ctx.Config.TableWidth = 120
		args := []string{"id:task-long-exact-id"}
		if full {
			args = append(args, "--full")
		}
		if err := taskView(ctx, args); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"P2  Prepare launch checklist\n", "Project: Work\n", "Section: Launch\n", "State: Active\n", "Due: 2026-09-29T16:30:00+02:00\n", "Timezone: Europe/Berlin\n", "Recurrence: Yes (every Tuesday at 16:30)\n", `Labels: "work", "a,b"`, "ID: task-long-exact-id\n", "Description:\n  First paragraph.\n  \n  - Confirm owner\n      literal code  spacing\n"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("missing %q in %s", want, out)
			}
		}
		if ctx.Stderr.(*bytes.Buffer).Len() != 0 {
			t.Errorf("unexpected stderr: %s", ctx.Stderr)
		}
		if full {
			if !strings.HasPrefix(out.String(), defaultOutput+"\n") {
				t.Errorf("full changed default layout: %s", out)
			}
			for _, want := range []string{"Project ID: project-A\n", "Section ID: section-B\n", "Parent ID: None\n", "Added: 2026-09-20T09:15:00Z\n", "Updated: 2026-09-28T13:45:00Z\n", "Completed at: None\n", "Comments: 2 (deprecated API value; not a reliable comment count)\n"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing full field %q: %s", want, out)
				}
			}
		} else {
			defaultOutput = out.String()
			if strings.Contains(defaultOutput, "Project ID:") || strings.Contains(defaultOutput, "Comments:") {
				t.Errorf("expanded fields in default: %s", out)
			}
		}
	}
}

func TestTaskDetailMissingFactsAndDates(t *testing.T) {
	for _, tc := range []struct {
		name, task string
		want       []string
		absent     string
	}{
		{"missing", `{}`, []string{"State: Not returned", "Due: Not returned", "Recurrence: Not returned", "Labels: Not returned", "Description:\n  Not returned", "Comments: Not returned", "Section: Not returned", "Completed at: Not returned"}, "State: Active"},
		{"null", `{"checked":null,"note_count":null,"description":null,"due":null,"section_id":null,"parent_id":null,"completed_at":null}`, []string{"State: Not returned", "Comments: Not returned", "Due: No due date", "Recurrence: None", "Section: None", "Parent ID: None", "Completed at: None"}, "State: Active"},
		{"empty", `{"checked":false,"note_count":0,"description":"","labels":[],"due":{}}`, []string{"State: Active", "Comments: 0", "Description:\n  None", "Labels: None", "Due: Not returned", "Recurrence: Not returned"}, "No due date"},
		{"completed", `{"checked":true,"completed_at":"2026-09-29T17:00:00+02:00"}`, []string{"State: Completed", "Completed at: 2026-09-29T17:00:00+02:00"}, "State: Active"},
		{"active with completion time", `{"checked":false,"completed_at":"2026-09-29T17:00:00+02:00"}`, []string{"State: Active", "Completed at: 2026-09-29T17:00:00+02:00"}, "State: Completed"},
		{"unknown with completion time", `{"completed_at":"2026-09-29T17:00:00Z"}`, []string{"State: Not returned", "Completed at: 2026-09-29T17:00:00Z"}, "State: Completed"},
		{"date differs", `{"due":{"date":"2026-09-29","datetime":"2026-09-30T00:30:00+02:00","timezone":"Europe/Berlin","is_recurring":false,"string":"next Tuesday"}}`, []string{"Due date: 2026-09-29", "Due time: 2026-09-30T00:30:00+02:00", "Timezone: Europe/Berlin", "Recurrence: None", "Due expression: next Tuesday"}, "Today"},
		{"floating and fixed timestamps", `{"due":{"date":"2026-10-01T09:00:00","datetime":"2026-10-01T09:00:00Z"}}`, []string{"Due date: 2026-10-01T09:00:00\n", "Due time: 2026-10-01T09:00:00Z\n", "Timezone: Not returned (time shown as returned)"}, "Due: "},
		{"same calendar date", `{"due":{"date":"2026-10-01","datetime":"2026-10-01T09:00:00Z"}}`, []string{"Due: 2026-10-01T09:00:00Z"}, "Due date:"},
		{"floating time", `{"due":{"datetime":"2026-09-29T16:30:00"}}`, []string{"Due: 2026-09-29T16:30:00", "Timezone: Not returned (time shown as returned)"}, "Europe/Berlin"},
		{"date only", `{"due":{"date":"2026-09-29","is_recurring":false}}`, []string{"Due: 2026-09-29", "Recurrence: None"}, "Timezone:"},
		{"expression only", `{"due":{"string":"tomorrow"}}`, []string{"Due: Not returned", "Due expression: tomorrow", "Recurrence: Not returned"}, "Due: tomorrow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			ctx := &Context{Stdout: &out, Mode: output.ModeHuman}
			if err := writeTaskDetail(ctx, detailTask(t, tc.task), true); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q: %s", want, &out)
				}
			}
			if strings.Contains(out.String(), tc.absent) {
				t.Errorf("invented %q: %s", tc.absent, &out)
			}
		})
	}
	for priority := 0; priority <= 5; priority++ {
		var out bytes.Buffer
		ctx := &Context{Stdout: &out}
		writeTaskDetail(ctx, detailTask(t, fmt.Sprintf(`{"content":"Task","priority":%d}`, priority)), false)
		if priority >= 1 && priority <= 4 {
			if !strings.HasPrefix(out.String(), fmt.Sprintf("P%d  Task\n", 5-priority)) {
				t.Errorf("priority %d: %s", priority, &out)
			}
		} else if !strings.Contains(out.String(), "Priority: Not returned") {
			t.Errorf("invalid priority asserted: %s", &out)
		}
	}
}

func TestTaskDetailTextPreservesWhitespaceAndFullContent(t *testing.T) {
	text := "  - Item  with spaces\n\n    code  x\nTail\n"
	var b strings.Builder
	writeDetailText(&b, text, "  ", "  ", 120, true)
	if want := "    - Item  with spaces\n  \n      code  x\n  Tail\n  \n"; b.String() != want {
		t.Fatalf("literal text changed: %q", b.String())
	}
	url := "https://example.invalid/" + strings.Repeat("long-path", 20)
	text = "  Markdown  spaces " + strings.Repeat("長い題名e\u0301 ", 30) + url + " \x1b[2J"
	for _, width := range []int{1, 40, 80, 120} {
		lines := wrapDetailLine(captureText(text), width, true)
		// Single word separators can become soft line breaks. Every text
		// character remains; literal spacing is checked separately above/below.
		if got := strings.Join(strings.Fields(strings.Join(lines, "")), ""); got != strings.Join(strings.Fields(captureText(text)), "") {
			t.Fatalf("lost text at width %d: %q", width, got)
		}
		for _, line := range lines {
			if overviewTextWidth(line) > width && !strings.Contains(line, url) && width != 1 {
				t.Errorf("overflow at width %d: %q", width, line)
			}
			if strings.Contains(line, "\x1b") {
				t.Errorf("terminal control passed through: %q", line)
			}
		}
	}
	if got := strings.Join(wrapDetailLine("  Markdown  spaces and words", 18, true), "\n"); got != "  Markdown  spaces\nand words" {
		t.Fatalf("soft wrap changed authored spacing/indent: %q", got)
	}
	if got := strings.Join(wrapDetailLine("Prepare launch checklist and confirm the rollback owner with the platform team", 36, true), "\n"); got != "Prepare launch checklist and confirm\nthe rollback owner with the platform\nteam" {
		t.Fatalf("separator shifted hanging indent: %q", got)
	}
	for _, text := range []string{
		" https://example.invalid/very-long-path",
		" abcdefghijklmnopqrst",
		"abc        defghijklmnopqrst",
	} {
		for _, width := range []int{1, 5, 10} {
			if got := strings.Join(wrapDetailLine(text, width, true), ""); got != text {
				t.Errorf("lost authored indentation at width %d: got %q, want %q", width, got, text)
			}
		}
	}
	for _, token := range []string{"task-exact-id-" + strings.Repeat("123", 30), "2026-09-29T16:30:00.123456789+02:00"} {
		b.Reset()
		writeDetailText(&b, token, "ID: ", "  ", 10, false)
		if !strings.Contains(b.String(), token) {
			t.Fatalf("split exact metadata: %q", b.String())
		}
	}
	for _, token := range []string{"2026-09-29T16:30:00.123456789+02:00", "2026-09-29T16:30:00.123456789", "HTTPS://example.invalid/very-long-path", "task-exact-id-123456789"} {
		lines := wrapDetailLine("Inspect ("+token+").", 10, true, "task-exact-id-123456789")
		if !strings.Contains(strings.Join(lines, "\n"), token) {
			t.Errorf("split identifier/time/URL inside text: %q", lines)
		}
	}
}

func TestTaskDetailEnrichmentFailures(t *testing.T) {
	for _, tc := range []struct {
		name, task, projects, sections, project, section string
		projectStatus, sectionStatus                     int
		projectRequests, sectionRequests                 int
	}{
		{"project failure", detailTaskJSON, "", `{"results":[{"id":"section-B","project_id":"project-A","name":"Launch"}]}`, "project-A (name unavailable; lookup failed)", "Launch", 403, 200, 1, 1},
		{"section failure", detailTaskJSON, `{"results":[{"id":"project-A","name":"Work"}]}`, "", "Work", "section-B (name unavailable; lookup failed)", 200, 403, 1, 1},
		{"names missing", detailTaskJSON, `{"results":[]}`, `{"results":[]}`, "project-A (name unavailable)", "section-B (name unavailable)", 200, 200, 1, 1},
		{"no section", `{"project_id":"project-A","section_id":null}`, `{"results":[{"id":"project-A","name":"Work"}]}`, "", "Work", "None", 200, 200, 1, 0},
		{"no project", `{"section_id":"section-B"}`, "", "", "Not returned", "section-B (name unavailable)", 200, 200, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			counts := map[string]int{}
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				counts[r.URL.Path]++
				mu.Unlock()
				switch r.URL.Path {
				case "/projects":
					w.WriteHeader(tc.projectStatus)
					io.WriteString(w, tc.projects)
				case "/sections":
					if r.URL.Query().Get("project_id") != "project-A" {
						t.Errorf("wrong section scope: %s", r.URL)
					}
					w.WriteHeader(tc.sectionStatus)
					io.WriteString(w, tc.sections)
				default:
					t.Errorf("unexpected request: %s", r.URL)
				}
			})
			ctx.Config.TableWidth = 120
			if err := writeTaskDetail(ctx, detailTask(t, tc.task), false); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Project: " + tc.project + "\n", "Section: " + tc.section + "\n"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q: %s", want, out)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if counts["/projects"] != tc.projectRequests || counts["/sections"] != tc.sectionRequests {
				t.Errorf("unnecessary requests: %v", counts)
			}
			if ctx.Stderr.(*bytes.Buffer).Len() != 0 {
				t.Errorf("enrichment failed command: %s", ctx.Stderr)
			}
		})
	}
}

func TestTaskDetailEnrichmentPaginationAndCache(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	ctx, _ := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		if r.URL.Query().Get("cursor") == "" {
			io.WriteString(w, `{"results":[],"next_cursor":"second"}`)
			return
		}
		switch r.URL.Path {
		case "/projects":
			io.WriteString(w, `{"results":[{"id":"project-A","name":"Work"}]}`)
		case "/sections":
			if r.URL.Query().Get("project_id") != "project-A" {
				t.Errorf("wrong section scope: %s", r.URL)
			}
			io.WriteString(w, `{"results":[{"id":"section-B","project_id":"project-A","name":"Launch"}]}`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
		}
	})
	for i := 0; i < 2; i++ {
		project, section := detailDestinations(ctx, detailTask(t, detailTaskJSON))
		if project != "Work" || section != "Launch" {
			t.Fatalf("lost paginated names: %q / %q", project, section)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if counts["/projects"] != 2 || counts["/sections"] != 2 {
		t.Fatalf("lookup cache not reused: %v", counts)
	}
}

func TestTaskDetailJSONAndPlainPreserveLegacyOutput(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "synthetic")
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/task-long-exact-id" || r.Method != http.MethodGet {
			t.Errorf("machine enrichment or mutation: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, detailTaskJSON)
	}))
	defer server.Close()
	for _, full := range []bool{false, true} {
		for _, mode := range []string{"", "--plain", "--json"} {
			args := []string{"--base-url", server.URL, "task", "view", "id:task-long-exact-id", "--no-input"}
			if mode != "" {
				args = append(args, mode)
			}
			if full {
				args = append(args, "--full")
			}
			var out, stderr bytes.Buffer
			if code := Execute(args, &out, &stderr); code != 0 || stderr.Len() != 0 {
				t.Fatalf("%v: code=%d stderr=%s", args, code, &stderr)
			}
			if mode == "--json" {
				var task api.Task
				if err := json.Unmarshal(out.Bytes(), &task); err != nil || task.Priority != 3 || task.ProjectID != "project-A" {
					t.Fatalf("machine task changed: %s / %v", &out, err)
				}
				if strings.Contains(out.String(), "Returned") || strings.Contains(out.String(), "timezone") || strings.Contains(out.String(), "is_recurring") {
					t.Fatalf("response-only facts leaked: %s", &out)
				}
				continue
			}
			want := "ID: task-long-exact-id\nContent: Prepare launch checklist\nDescription: First paragraph.\n\n- Confirm owner\n    literal code  spacing\nProject: project-A\nSection: section-B\nLabels: work, a,b\nDue: 2026-09-29T16:30:00+02:00\n"
			if full {
				want += "Priority: 3\nCompleted: false\nAdded: 2026-09-20T09:15:00Z\nUpdated: 2026-09-28T13:45:00Z\nCompletedAt: \nNoteCount: 2\n"
			}
			if out.String() != want {
				t.Fatalf("legacy %q full=%t changed: %q", mode, full, out.String())
			}
		}
	}
}
