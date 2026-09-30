package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

const duplicateTaskResponse = `{"results":[{"id":"first","content":"Review","project_id":"work","section_id":"launch","due":{"date":"2026-09-30"}},{"id":"second","content":"Review","project_id":"home","due":null}],"next_cursor":""}`

type taskAmbiguityRequests struct {
	mu    sync.Mutex
	calls []string
}

func (requests *taskAmbiguityRequests) snapshot() []string {
	requests.mu.Lock()
	defer requests.mu.Unlock()
	return append([]string(nil), requests.calls...)
}

func newTaskAmbiguityContext(t *testing.T, taskResponse string, metadataUnavailable bool) (*Context, *taskAmbiguityRequests) {
	t.Helper()
	requests := &taskAmbiguityRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.mu.Lock()
		requests.calls = append(requests.calls, r.Method+" "+r.URL.Path)
		requests.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if metadataUnavailable && (r.URL.Path == "/projects" || r.URL.Path == "/sections") {
			http.Error(w, "metadata unavailable", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/tasks":
			_, _ = w.Write([]byte(taskResponse))
		case "/tasks/second":
			_, _ = w.Write([]byte(`{"id":"second","content":"Review","project_id":"home","section_id":"launch","due":null}`))
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"work","name":"Work"},{"id":"home","name":"Home"}],"next_cursor":""}`))
		case "/sections":
			_, _ = w.Write([]byte(`{"results":[{"id":"launch","project_id":"work","name":"Launch"}],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return &Context{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("2\n"),
		Token: "test-token", Client: api.NewClient(server.URL, "test-token", time.Second, authorization.Resolve(nil, "credentials", true)),
		Mode: output.ModeHuman, Config: config.Config{BaseURL: server.URL, TimeoutSeconds: 2, TableWidth: 120},
		Now: func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) },
	}, requests
}

func TestTaskAmbiguityNoninteractiveNeverSelectsMutationTarget(t *testing.T) {
	for _, match := range []struct {
		name, ref, response string
		fuzzy               bool
	}{
		{"duplicate exact titles", "Review", duplicateTaskResponse, false},
		{"substring", "Rev", duplicateTaskResponse, false},
		{"natural due hint", "Review tomorrow", `{"results":[{"id":"first","content":"Review","project_id":"work","due":{"date":"2026-09-30"}},{"id":"second","content":"Review","project_id":"home","due":{"date":"2026-09-30"}}],"next_cursor":""}`, false},
		{"different fuzzy ranks", "rev", `{"results":[{"id":"first","content":"Review","project_id":"work","section_id":"launch"},{"id":"second","content":"Annual review","project_id":"home"}],"next_cursor":""}`, true},
	} {
		for _, noInput := range []bool{false, true} {
			name := match.name + "/non-TTY"
			if noInput {
				name = match.name + "/no-input"
			}
			t.Run(name, func(t *testing.T) {
				ctx, requests := newTaskAmbiguityContext(t, match.response, false)
				ctx.Fuzzy = match.fuzzy
				ctx.Global.NoInput = noInput
				err := taskComplete(ctx, []string{match.ref})
				var ambiguous *AmbiguousMatchError
				if !errors.As(err, &ambiguous) || toExitCode(err) != exitUsage {
					t.Fatalf("expected usage ambiguity, got %v", err)
				}
				if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks"}) {
					t.Fatalf("noninteractive ambiguity performed enrichment or mutation: %v", got)
				}
				if ctx.Stdout.(*bytes.Buffer).Len() != 0 || ctx.Stderr.(*bytes.Buffer).Len() != 0 {
					t.Fatalf("noninteractive ambiguity prompted or reported success: stdout=%q stderr=%q", ctx.Stdout, ctx.Stderr)
				}
			})
		}
	}
}

func TestTaskExactIDResolutionAvoidsAmbiguityEnrichment(t *testing.T) {
	for _, ref := range []string{"id:second", "https://app.todoist.com/app/task/review-second"} {
		t.Run(ref, func(t *testing.T) {
			ctx, requests := newTaskAmbiguityContext(t, duplicateTaskResponse, false)
			task, err := resolveTaskRef(ctx, ref)
			if err != nil || task.ID != "second" {
				t.Fatalf("exact reference resolved to %#v, %v", task, err)
			}
			if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks/second"}) {
				t.Fatalf("exact reference performed enumeration or enrichment: %v", got)
			}
			if ctx.Stderr.(*bytes.Buffer).Len() != 0 {
				t.Fatalf("exact reference prompted: %q", ctx.Stderr)
			}
		})
	}
}

func TestTaskAmbiguityMachineErrorContractUnchanged(t *testing.T) {
	for _, mode := range []output.Mode{output.ModeJSON, output.ModePlain, output.ModeNDJSON} {
		t.Run(string(mode), func(t *testing.T) {
			ctx, requests := newTaskAmbiguityContext(t, duplicateTaskResponse, false)
			ctx.Mode = mode
			ctx.Global.NoInput = true
			err := taskComplete(ctx, []string{"Review"})
			if toExitCode(err) != exitUsage {
				t.Fatalf("expected usage error, got %v", err)
			}
			writeError(ctx, err)
			want := `{
  "details": {
    "entity": "task",
    "input": "Review",
    "matches": [
      "Review",
      "Review"
    ],
    "type": "ambiguous_match"
  },
  "error": "ambiguous task match for \"Review\"; matches: Review, Review",
  "meta": {}
}
`
			if mode != output.ModeJSON {
				want = "error: ambiguous task match for \"Review\"; matches: Review, Review\n"
			}
			if got := ctx.Stderr.(*bytes.Buffer).String(); got != want {
				t.Fatalf("machine ambiguity contract changed: %q", got)
			}
			if ctx.Stdout.(*bytes.Buffer).Len() != 0 {
				t.Fatalf("machine ambiguity wrote stdout: %q", ctx.Stdout)
			}
			if got := requests.snapshot(); !reflect.DeepEqual(got, []string{"GET /tasks"}) {
				t.Fatalf("machine ambiguity performed enrichment or mutation: %v", got)
			}
		})
	}
}
