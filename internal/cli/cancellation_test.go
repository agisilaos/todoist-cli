package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
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
	code := ExecuteWithEnvironment([]string{"--config", t.TempDir() + "/config.json", "--base-url", server.URL, "task", "view", "id:synthetic", "--json", "--no-input"}, &out, &errOut, Environment{
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
					fmt.Fprint(w, `{"results":[{"id":"first"},{"id":"second"}]}`)
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
