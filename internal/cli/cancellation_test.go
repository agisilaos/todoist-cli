package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestTaskAdaptersHonorCallerCancellation(t *testing.T) {
	for _, adapter := range []string{"target", "action", "filter"} {
		t.Run(adapter, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/tasks/filter" {
					fmt.Fprint(w, `{"results":[]}`)
				} else {
					fmt.Fprint(w, `{"id":"synthetic","content":"Task"}`)
				}
			}))
			defer server.Close()
			ctx := newApplyTestContext(t.TempDir(), server.URL)
			operation, cancel := context.WithCancel(context.Background())
			cancel()
			var err error
			switch adapter {
			case "target":
				_, err = (cliTaskResolver{ctx: ctx}).ResolveTaskRef(operation, "id:synthetic")
			case "action":
				_, err = (&taskActionResolver{ctx: ctx}).ResolveTaskRef(operation, "id:synthetic")
			case "filter":
				_, err = (cliTaskFilterLister{ctx: ctx}).ListByFilter(operation, "today")
			}
			if !errors.Is(err, context.Canceled) || requests.Load() != 0 {
				t.Fatalf("caller cancellation discarded: err=%v requests=%d", err, requests.Load())
			}
			if ctx.OperationContext != nil {
				t.Fatal("adapter changed the enclosing operation context")
			}
		})
	}
}

func TestTaskResolutionHonorsInFlightCallerDeadline(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx := newApplyTestContext(t.TempDir(), server.URL)
	parent := context.Background()
	ctx.OperationContext = parent
	operation, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := (cliTaskResolver{ctx: ctx}).ResolveTaskRef(operation, "id:synthetic")
	if !errors.Is(err, context.DeadlineExceeded) || ctx.OperationContext != parent {
		t.Fatalf("deadline discarded or parent changed: %v", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("request did not reach the transport")
	}
}

func TestExecutePropagatesOperationCancellation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"id":"synthetic"}`)
	}))
	defer server.Close()
	operation, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	code := executeTestWithEnvironment([]string{"--config", t.TempDir() + "/config.json", "--base-url", server.URL, "task", "view", "id:synthetic", "--json", "--no-input"}, &out, &errOut, Environment{
		OperationContext: operation,
		Getenv: func(key string) string {
			if key == "TODOIST_TOKEN" {
				return "synthetic"
			}
			return ""
		},
	})
	if code != exitError || requests.Load() != 0 || out.Len() != 0 || !bytes.Contains(errOut.Bytes(), []byte("context canceled")) {
		t.Fatalf("exit %d requests %d stdout %q stderr %q", code, requests.Load(), out.String(), errOut.String())
	}
}

func TestCancelledApplyAbortsContinueMode(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer server.Close()
	ctx := newApplyTestContext(t.TempDir(), server.URL)
	operation, cancel := context.WithCancel(context.Background())
	cancel()
	ctx.OperationContext = operation
	results, err := applyActionsWithMode(ctx, "synthetic", replayTestActions(), applyErrorModeContinue)
	if !errors.Is(err, context.Canceled) || len(results) != 1 || requests.Load() != 0 {
		t.Fatalf("cancelled apply continued: err=%v results=%d requests=%d", err, len(results), requests.Load())
	}
}

func TestPlannerProcessHonorsOperationCancellation(t *testing.T) {
	ctx := newApplyTestContext(t.TempDir(), "http://unused.invalid")
	ctx.lookupCache = &lookupCache{projectsLoaded: true, labelsLoaded: true, activeTasksLoaded: true, sectionsByProject: map[string][]api.Section{"": {}}}
	operation, cancel := context.WithCancel(context.Background())
	cancel()
	ctx.OperationContext = operation
	if _, err := runPlanner(ctx, "echo '{}'", "synthetic", 1, plannerContextOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("planner discarded cancellation: %v", err)
	}
}

func TestBulkTaskCancellationStopsAfterFirstMutation(t *testing.T) {
	for _, command := range []string{"complete", "move"} {
		t.Run(command, func(t *testing.T) {
			operation, cancel := context.WithCancel(context.Background())
			defer cancel()
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					fmt.Fprint(w, `{"results":[{"id":"first","parent_id":null},{"id":"second","parent_id":null}],"next_cursor":null}`)
					return
				}
				writes.Add(1)
				cancel()
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()
			ctx := newApplyTestContext(t.TempDir(), server.URL)
			ctx.OperationContext = operation
			args := []string{"--filter", "today", "--yes"}
			var err error
			if command == "move" {
				err = taskMove(ctx, append(args, "--project", "id:project"))
			} else {
				err = taskComplete(ctx, args)
			}
			if !errors.Is(err, context.Canceled) || writes.Load() != 1 {
				t.Fatalf("bulk cancellation did not stop: err=%v writes=%d", err, writes.Load())
			}
		})
	}
}

func TestManualLoginValidationHonorsCancellation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()
	ctx := newAuthTestContext(t)
	ctx.Config.BaseURL = server.URL
	operation, cancel := context.WithCancel(context.Background())
	cancel()
	ctx.OperationContext = operation
	if err := validateManualLoginToken(ctx, "synthetic"); err == nil || requests.Load() != 0 {
		t.Fatalf("cancelled login dispatched validation: err=%v requests=%d", err, requests.Load())
	}
}

// This transport models a single request exhausting its deadline while the
// enclosing operation remains alive, without sleeping for the configured timeout.
type cancellationTransport func(*http.Request) (*http.Response, error)

func (f cancellationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBulkTaskRequestTimeoutStopsOnUncertainty(t *testing.T) {
	for _, command := range []string{"complete", "move"} {
		t.Run(command, func(t *testing.T) {
			ctx := newApplyTestContext(t.TempDir(), "https://example.com")
			var out bytes.Buffer
			ctx.Stdout, ctx.Mode = &out, output.ModeJSON
			var visited []string
			ctx.Client.SetTransport(cancellationTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"results":[{"id":"first","parent_id":null},{"id":"slow","parent_id":null},{"id":"last","parent_id":null}],"next_cursor":null}`
				if r.Method == http.MethodPost {
					visited = append(visited, r.URL.Path)
					if strings.Contains(r.URL.Path, "/slow/") {
						return nil, context.DeadlineExceeded
					}
					body = `{}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			args := []string{"--filter", "today", "--yes"}
			var err error
			if command == "move" {
				err = taskMove(ctx, append(args, "--project", "id:project"))
			} else {
				err = taskComplete(ctx, args)
			}
			var summary map[string]any
			if err == nil || json.Unmarshal(out.Bytes(), &summary) != nil {
				t.Fatalf("uncertain timeout lost batch accounting: err=%v output=%s", err, out.String())
			}
			success := "completed"
			if command == "move" {
				success = "moved"
			}
			if summary[success] != float64(1) || summary["failed"] != float64(1) || summary["count"] != float64(3) || summary["uncertain"] != float64(1) || summary["unattempted"] != float64(1) || len(visited) != 2 || !strings.Contains(visited[1], "/slow/") {
				t.Fatalf("bad summary: %v visits=%v", summary, visited)
			}
		})
	}
}

func TestApplyNonTaskRequestTimeoutContinues(t *testing.T) {
	ctx := newApplyTestContext(t.TempDir(), "https://example.com")
	ctx.Client.SetTransport(cancellationTransport(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		if strings.Contains(string(data), `"name":"B"`) {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"synthetic"}`))}, nil
	}))
	actions := []Action{{Type: "project_add", Name: "A"}, {Type: "project_add", Name: "B"}, {Type: "project_add", Name: "C"}}
	results, err := applyActionsWithMode(ctx, "timeout-test", actions, applyErrorModeContinue)
	if err != nil || len(results) != 3 || results[0].Error != nil || !errors.Is(results[1].Error, context.DeadlineExceeded) || results[2].Error != nil {
		t.Fatalf("request timeout stopped continue mode: err=%v results=%v", err, results)
	}
	assertReplayKeys(t, ctx, "timeout-test", actions, []int{0, 2})
}

func TestBulkCancellationReportsPartialProgress(t *testing.T) {
	for _, command := range []string{"complete", "move"} {
		for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON, output.ModeHuman} {
			t.Run(command+"/"+string(mode), func(t *testing.T) {
				ctx := newApplyTestContext(t.TempDir(), "https://example.com")
				operation, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx.OperationContext = operation
				var out bytes.Buffer
				ctx.Stdout, ctx.Mode = &out, mode
				writes := 0
				ctx.Client.SetTransport(cancellationTransport(func(r *http.Request) (*http.Response, error) {
					body := `{"results":[{"id":"first","parent_id":null},{"id":"second","parent_id":null},{"id":"last","parent_id":null}],"next_cursor":null}`
					if r.Method == http.MethodPost {
						writes++
						if writes == 2 {
							cancel()
							return nil, context.Canceled
						}
						body = `{}`
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
				}))
				args := []string{"--filter", "today", "--yes"}
				var err error
				if command == "move" {
					err = taskMove(ctx, append(args, "--project", "id:project"))
				} else {
					err = taskComplete(ctx, args)
				}
				if !errors.Is(err, context.Canceled) || writes != 2 {
					t.Fatalf("cancellation lost: %v writes=%d", err, writes)
				}
				if mode == output.ModeHuman {
					if !strings.Contains(out.String(), "accepted=1") || !strings.Contains(out.String(), "uncertain=1") || !strings.Contains(out.String(), "unattempted=1") {
						t.Fatalf("partial summary missing: %s", out.String())
					}
				} else {
					var summary map[string]any
					if json.Unmarshal(out.Bytes(), &summary) != nil || summary["uncertain"] != float64(1) || summary["unattempted"] != float64(1) || summary["failed"] != float64(1) {
						t.Fatalf("partial summary missing: %s", out.String())
					}
					key := "completed"
					if command == "move" {
						key = "moved"
					}
					if summary[key] != float64(1) {
						t.Fatal(summary)
					}
				}
			})
		}
	}
}

func TestApplyTaskRequestTimeoutStopsOnUncertainty(t *testing.T) {
	ctx := newApplyTestContext(t.TempDir(), "https://example.com")
	calls := 0
	ctx.Client.SetTransport(cancellationTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 2 {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"synthetic"}`))}, nil
	}))
	actions := replayTestActions()
	results, err := applyActionsWithMode(ctx, "task-timeout", actions, applyErrorModeContinue)
	if !errors.Is(err, context.DeadlineExceeded) || api.TaskWriteOutcome(err) != api.TaskWriteUncertain || len(results) != 2 || calls != 2 || operationContext(ctx).Err() != nil {
		t.Fatalf("uncertain task write continued: err=%v results=%v calls=%d", err, results, calls)
	}
	assertReplayKeys(t, ctx, "task-timeout", actions, []int{0})
	store, loadErr := loadReplayStore(ctx)
	if loadErr != nil || !store.HasPendingTaskWrite(makeReplayKey("task-timeout", 1, actions[1])) {
		t.Fatalf("uncertain pending evidence lost: %v", loadErr)
	}
	_, err = applyActionsWithMode(ctx, "task-timeout", actions, applyErrorModeContinue)
	if err == nil || calls != 2 {
		t.Fatalf("pending action was blindly retried: err=%v calls=%d", err, calls)
	}
}

func TestTaskAdaptersLeaveParentContextUntouchedDuringRequests(t *testing.T) {
	for _, adapter := range []string{"target", "action", "filter"} {
		t.Run(adapter, func(t *testing.T) {
			ctx := newApplyTestContext(t.TempDir(), "https://example.com")
			parent := context.Background()
			ctx.OperationContext = parent
			type marker struct{}
			operation := context.WithValue(parent, marker{}, "child")
			ctx.Client.SetTransport(cancellationTransport(func(r *http.Request) (*http.Response, error) {
				if ctx.OperationContext != parent {
					t.Error("adapter overwrote the shared parent during dispatch")
				}
				if r.Context().Value(marker{}) != "child" {
					t.Error("adapter discarded the caller's context")
				}
				body := `{"id":"task"}`
				if adapter == "filter" {
					body = `{"results":[{"id":"task","parent_id":null}],"next_cursor":null}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			var err error
			switch adapter {
			case "target":
				_, err = (cliTaskResolver{ctx: ctx}).ResolveTaskRef(operation, "id:task")
			case "action":
				_, err = (&taskActionResolver{ctx: ctx}).ResolveTaskRef(operation, "id:task")
			case "filter":
				_, err = (cliTaskFilterLister{ctx: ctx}).ListByFilter(operation, "today")
			}
			if err != nil || ctx.OperationContext != parent {
				t.Fatalf("adapter changed parent: %v", err)
			}
		})
	}
}

func TestTaskAdapterPreservesInvocationLookupCache(t *testing.T) {
	ctx := newApplyTestContext(t.TempDir(), "https://example.com")
	calls := 0
	ctx.Client.SetTransport(cancellationTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"results":[{"id":"task","content":"Shared task"}]}`))}, nil
	}))
	task, err := (cliTaskResolver{ctx: ctx}).ResolveTaskRef(context.Background(), "Shared task")
	if err != nil || task.ID != "task" {
		t.Fatalf("resolution: %v %v", task, err)
	}
	tasks, err := listAllActiveTasks(ctx)
	if err != nil || len(tasks) != 1 || calls != 1 {
		t.Fatalf("adapter lost invocation cache: %v calls=%d", err, calls)
	}
}
