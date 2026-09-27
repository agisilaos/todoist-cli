package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func captureTestContext(t *testing.T, handler http.HandlerFunc) (*Context, *bytes.Buffer) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	out := &bytes.Buffer{}
	return &Context{Stdout: out, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader(""), Mode: output.ModeHuman,
		Token: "fixture", Config: config.Config{TimeoutSeconds: 1},
		Client: api.NewClient(ts.URL, "fixture", time.Second, authorization.Resolve(nil, "env", true))}, out
}

func TestCaptureReceiptUsesReturnedStateAcrossCreationPaths(t *testing.T) {
	for _, command := range []string{"quick", "strict", "task", "inbox"} {
		for priority := 1; priority <= 4; priority++ {
			t.Run(fmt.Sprintf("%s/p%d", command, priority), func(t *testing.T) {
				posts := 0
				ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
					switch r.Method + " " + r.URL.Path {
					case "GET /projects":
						io.WriteString(w, `{"results":[{"id":"100","name":"Inbox","inbox_project":true},{"id":"200","name":"Home"}]}`)
					case "GET /sections":
						io.WriteString(w, `{"results":[{"id":"201","name":"Planning"}]}`)
					case "POST /tasks", "POST /tasks/quick":
						posts++
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						if r.URL.Path == "/tasks/quick" {
							if !strings.Contains(fmt.Sprint(body["text"]), fmt.Sprintf("p%d", priority)) {
								t.Error(body)
							}
						} else if body["priority"] != float64(5-priority) {
							t.Error(body)
						}
						fmt.Fprintf(w, `{"id":"full-task-123456789","content":"Returned content","project_id":"200","section_id":"201","priority":%d,"labels":["work"],"due":{"date":"2026-09-28T09:00:00Z","timezone":"Europe/Berlin","string":"every day at 11","is_recurring":true}}`, 5-priority)
					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						http.Error(w, "unexpected", 500)
					}
				})
				var err error
				token := fmt.Sprintf("p%d", priority)
				switch command {
				case "quick":
					err = quickAddCommand(ctx, []string{"Input #Home tomorrow @work " + token})
				case "strict":
					err = quickAddCommand(ctx, []string{"Input", "--strict", "--project", "Home", "--priority", token})
				case "task":
					err = taskAdd(ctx, []string{"--content", "Input", "--priority", token})
				case "inbox":
					err = inboxAdd(ctx, []string{"--content", "Input", "--priority", fmt.Sprint(5 - priority)})
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{"Created task\n", "Content: Returned content\n", "Project: Home\n", "Section: Planning\n", "Due: 2026-09-28T09:00:00Z\n", "Timezone: Europe/Berlin\n", "Recurrence: Yes (every day at 11)\n", fmt.Sprintf("Priority: %d\n", priority), `Labels: "work"`, "View: todoist task view id:full-task-123456789\n", "task update --help", "task move --help"} {
					if !strings.Contains(out.String(), want) {
						t.Errorf("missing %q in %s", want, out)
					}
				}
				if posts != 1 || strings.Contains(out.String(), "Content: Input") {
					t.Fatalf("unexpected mutations/state: %d %s", posts, out)
				}
			})
		}
	}
}

func TestCaptureReceiptAbsentFieldsAndDates(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		want           []string
		absent         string
	}{
		{"none", `{"id":"1","content":"Milk","priority":1,"due":null,"labels":[]}`, []string{"Due: No due date\n", "Recurrence: None\n", "Labels: None\n", "Priority: 4\n"}, "Timezone:"},
		{"missing", `{"id":"1"}`, []string{"Content: Not returned\n", "Project: Not returned\n", "Due: Not returned\n", "Recurrence: Not returned\n", "Labels: Not returned\n", "Priority: Not returned\n"}, "No due date"},
		{"null-labels", `{"id":"1","labels":null}`, []string{"Labels: Not returned\n"}, "Labels: None"},
		{"date", `{"id":"1","due":{"date":"2026-09-28","is_recurring":false}}`, []string{"Due: 2026-09-28\n", "Recurrence: None\n"}, "Timezone:"},
		{"local-time", `{"id":"1","due":{"date":"2026-09-28T09:30:00"}}`, []string{"Due: 2026-09-28T09:30:00\n", "Timezone: Not returned (time shown as returned)\n"}, "Europe/"},
		{"offset", `{"id":"1","due":{"datetime":"2026-09-28T09:30:00+02:00"}}`, []string{"Due: 2026-09-28T09:30:00+02:00\n"}, "Timezone:"},
		{"expression-only", `{"id":"1","due":{"string":"tomorrow"}}`, []string{"Due: Not returned\n", "Recurrence: Not returned\n"}, "Due: tomorrow"},
		{"no-id", `{}`, []string{"Verify in Todoist before retrying."}, "Created task"},
		{"lookup-failed", `{"id":"1","project_id":"200","section_id":"201"}`, []string{"Project: 200 (name unavailable)", "Section: 201 (name unavailable)"}, "Project: Inbox"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unavailable", 403) })
			var task api.Task
			if err := json.Unmarshal([]byte(tc.response), &task); err != nil {
				t.Fatal(err)
			}
			if err := writeCaptureReceipt(ctx, task); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q: %s", want, out)
				}
			}
			if strings.Contains(out.String(), tc.absent) {
				t.Errorf("unexpected %q: %s", tc.absent, out)
			}
		})
	}
}

func TestCaptureReceiptFullAccessibleSafeText(t *testing.T) {
	content := strings.Repeat("Long 日本語 & 'quoted' text ", 12) + "\nForged line\t\x1b[31m\u202e"
	var previous string
	for _, accessible := range []bool{false, true} {
		ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected lookup") })
		ctx.Accessible, ctx.Global.NoColor = accessible, true
		ctx.Config.TableWidth = 20
		if err := writeCaptureReceipt(ctx, api.Task{ID: "abc'$(echo bad)", Content: content, Labels: []string{"x,y", "a\nb"}}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), strings.Repeat("Long 日本語 & 'quoted' text ", 12)) || !strings.Contains(out.String(), `\nForged line\t\x1b[31m\u202e`) {
			t.Fatal(out.String())
		}
		if !strings.Contains(out.String(), `Labels: "x,y", "a\nb"`) || strings.ContainsAny(out.String(), "\x1b\u202e") {
			t.Fatal(out.String())
		}
		if !strings.Contains(out.String(), `View: todoist task view 'id:abc'"'"'$(echo bad)'`) {
			t.Fatal(out.String())
		}
		if accessible && previous != out.String() {
			t.Fatal("accessible receipt lost information")
		}
		previous = out.String()
	}
}

func TestCaptureReceiptPreservesOutputContracts(t *testing.T) {
	var task api.Task
	if err := json.Unmarshal([]byte(`{"id":"1","content":"Milk","priority":3,"labels":[],"due":{"date":"2026-09-28","timezone":"Europe/Berlin","is_recurring":true}}`), &task); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON, output.ModePlain, output.ModeHuman} {
		t.Run(string(mode), func(t *testing.T) {
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"results":[]}`) })
			ctx.Mode, ctx.Global.Quiet = mode, true
			if err := writeTaskList(ctx, []api.Task{task}, "", false); err != nil {
				t.Fatal(err)
			}
			want := out.String()
			out.Reset()
			if err := writeCaptureReceipt(ctx, task); err != nil {
				t.Fatal(err)
			}
			if out.String() != want {
				t.Fatalf("contract changed: %q != %q", out, want)
			}
			if mode != output.ModeHuman {
				ctx.Global.Quiet = false
				out.Reset()
				if err := writeCaptureReceipt(ctx, task); err != nil {
					t.Fatal(err)
				}
				if out.String() != want {
					t.Fatalf("nonquiet contract changed: %q", out)
				}
			}
		})
	}
}

func TestCapturePreviewAndFailuresDoNotClaimSuccess(t *testing.T) {
	for _, command := range []string{"quick", "strict", "task", "inbox"} {
		t.Run(command, func(t *testing.T) {
			requests := 0
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) { requests++; http.Error(w, "denied", 403) })
			ctx.Global.DryRun = true
			var err error
			switch command {
			case "quick":
				err = quickAddCommand(ctx, []string{"Milk tomorrow p2"})
			case "strict":
				err = quickAddCommand(ctx, []string{"Milk", "--strict", "--due", "tomorrow", "--priority", "p2"})
			case "task":
				err = taskAdd(ctx, []string{"--content", "Milk", "--due", "tomorrow", "--priority", "p2"})
			case "inbox":
				err = inboxAdd(ctx, []string{"--content", "Milk", "--due", "tomorrow", "--priority", "3"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if requests != 0 || !strings.Contains(out.String(), "no task created") || !strings.Contains(out.String(), "Milk") || strings.Contains(out.String(), "Created task") {
				t.Fatal(out.String())
			}
			if command != "quick" && (!strings.Contains(out.String(), "Priority: 2") || !strings.Contains(out.String(), "interpret the due expression")) {
				t.Fatal(out.String())
			}
			ctx.Global.DryRun = false
			out.Reset()
			if err := quickAddCommand(ctx, []string{"Milk"}); err == nil || out.Len() != 0 {
				t.Fatalf("failed mutation claimed success: %v %s", err, out)
			}
			if err := taskAdd(ctx, []string{"Milk"}); err == nil || out.Len() != 0 {
				t.Fatalf("failed mutation claimed success: %v %s", err, out)
			}
		})
	}
}

func TestCapturePreviewPreservesMachineAndQuietContracts(t *testing.T) {
	for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON, output.ModePlain, output.ModeHuman} {
		ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
		ctx.Mode, ctx.Global.Quiet = mode, true
		for _, action := range []string{"task add", "inbox add"} {
			payload := map[string]any{"content": "Milk", "priority": 3}
			out.Reset()
			if err := writeDryRun(ctx, action, payload); err != nil {
				t.Fatal(err)
			}
			want := out.String()
			out.Reset()
			if err := writeCapturePreview(ctx, action, payload); err != nil {
				t.Fatal(err)
			}
			if out.String() != want {
				t.Fatalf("contract changed: %q != %q", out, want)
			}
		}
	}
}
