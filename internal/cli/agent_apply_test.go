package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestApplyActionsFailFastRerunSkipsDurableSuccess(t *testing.T) {
	var mu sync.Mutex
	requests := make([]string, 0, 4)
	attempts := map[string]int{}
	server := newApplyTestServer(t, func(content string) int {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, content)
		attempts[content]++
		if content == "B" && attempts[content] == 1 {
			return http.StatusBadRequest
		}
		return http.StatusOK
	})
	defer server.Close()

	ctx := newApplyTestContext(t.TempDir(), server.URL)
	actions := replayTestActions()
	first, err := applyActionsWithMode(ctx, "confirm", actions, applyErrorModeFail)
	if err == nil {
		t.Fatal("expected first run to stop on B")
	}
	if len(first) != 2 || first[0].Error != nil || first[1].Error == nil {
		t.Fatalf("unexpected first-run results: %#v", first)
	}
	assertReplayKeys(t, ctx, "confirm", actions, []int{0})

	second, err := applyActionsWithMode(ctx, "confirm", actions, applyErrorModeFail)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if len(second) != 3 || !second[0].SkippedReplay || second[1].Error != nil || second[2].Error != nil {
		t.Fatalf("unexpected rerun results: %#v", second)
	}
	assertReplayKeys(t, ctx, "confirm", actions, []int{0, 1, 2})

	mu.Lock()
	gotRequests := append([]string(nil), requests...)
	mu.Unlock()
	if want := []string{"A", "B", "B", "C"}; !reflect.DeepEqual(gotRequests, want) {
		t.Fatalf("request order = %v, want %v", gotRequests, want)
	}
}

func TestApplyActionsContinueRerunRetriesOnlyRemoteFailure(t *testing.T) {
	var mu sync.Mutex
	requests := make([]string, 0, 4)
	attempts := map[string]int{}
	server := newApplyTestServer(t, func(content string) int {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, content)
		attempts[content]++
		if content == "B" && attempts[content] == 1 {
			return http.StatusBadRequest
		}
		return http.StatusOK
	})
	defer server.Close()

	ctx := newApplyTestContext(t.TempDir(), server.URL)
	actions := replayTestActions()
	first, err := applyActionsWithMode(ctx, "confirm", actions, applyErrorModeContinue)
	if err != nil {
		t.Fatalf("continue run returned error: %v", err)
	}
	if len(first) != 3 || first[0].Error != nil || first[1].Error == nil || first[2].Error != nil {
		t.Fatalf("unexpected continue results: %#v", first)
	}
	assertReplayKeys(t, ctx, "confirm", actions, []int{0, 2})

	second, err := applyActionsWithMode(ctx, "confirm", actions, applyErrorModeContinue)
	if err != nil {
		t.Fatalf("continue rerun: %v", err)
	}
	if len(second) != 3 || !second[0].SkippedReplay || second[1].Error != nil || !second[2].SkippedReplay {
		t.Fatalf("unexpected continue rerun results: %#v", second)
	}
	assertReplayKeys(t, ctx, "confirm", actions, []int{0, 1, 2})

	mu.Lock()
	gotRequests := append([]string(nil), requests...)
	mu.Unlock()
	if want := []string{"A", "B", "C", "B"}; !reflect.DeepEqual(gotRequests, want) {
		t.Fatalf("request order = %v, want %v", gotRequests, want)
	}
}

func TestApplyActionsRecordsBeforeSuccessEvents(t *testing.T) {
	trace := &applyTrace{}
	events := make([]map[string]any, 0, 8)
	progress := &applyEventTraceWriter{trace: trace, events: &events}
	server := newApplyTestServer(t, func(string) int {
		trace.Add("todoist_mutation")
		return http.StatusOK
	})
	defer server.Close()
	ctx := newApplyTestContext(t.TempDir(), server.URL)
	ctx.Progress = &progressSink{out: progress}
	store := &fakeReplayStore{applied: map[string]bool{}, trace: trace}

	results, err := applyActionsWithReplayStore(ctx, "confirm", []Action{{Type: "task_add", Content: "A"}}, applyErrorModeFail, store)
	if err != nil {
		t.Fatalf("applyActionsWithReplayStore: %v", err)
	}
	if len(results) != 1 || results[0].Error != nil {
		t.Fatalf("unexpected results: %#v", results)
	}
	want := []string{
		"agent_action_start",
		"agent_action_validated",
		"agent_action_dispatched",
		"todoist_mutation",
		"replay_record",
		"agent_action_complete",
		"agent_action_succeeded",
	}
	if got := trace.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("event order = %v, want %v", got, want)
	}
}

func TestApplyActionsReplayWriteFailureIsFatalInContinueMode(t *testing.T) {
	trace := &applyTrace{}
	events := make([]map[string]any, 0, 8)
	progress := &applyEventTraceWriter{trace: trace, events: &events}
	var requestCount atomic.Int32
	server := newApplyTestServer(t, func(string) int {
		requestCount.Add(1)
		trace.Add("todoist_mutation")
		return http.StatusOK
	})
	defer server.Close()
	ctx := newApplyTestContext(t.TempDir(), server.URL)
	ctx.Progress = &progressSink{out: progress}
	wantErr := errors.New("injected write failure")
	store := &fakeReplayStore{applied: map[string]bool{}, recordErr: wantErr, trace: trace}
	actions := []Action{
		{Type: "task_add", Content: "A"},
		{Type: "task_add", Content: "B"},
	}

	results, err := applyActionsWithReplayStore(ctx, "confirm", actions, applyErrorModeContinue, store)
	if !errors.Is(err, wantErr) {
		t.Fatalf("apply error = %v, want wrapped %v", err, wantErr)
	}
	if !shouldAbortApply(applyErrorModeContinue, err) {
		t.Fatal("replay write failure must abort continue mode")
	}
	if len(results) != 1 || results[0].Error == nil {
		t.Fatalf("unexpected results: %#v", results)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected second action not to dispatch, got %d requests", got)
	}
	if containsEvent(events, "agent_action_complete") || containsEvent(events, "agent_action_succeeded") {
		t.Fatalf("unexpected success event after replay failure: %v", eventTypes(events))
	}
	failed := findEvent(events, "agent_action_failed")
	if failed["stage"] != "replay_record" || failed["remote_succeeded"] != true {
		t.Fatalf("unexpected replay failure event: %#v", failed)
	}
	wantTrace := []string{
		"agent_action_start",
		"agent_action_validated",
		"agent_action_dispatched",
		"todoist_mutation",
		"replay_record",
		"agent_action_error",
		"agent_action_failed",
	}
	if got := trace.Snapshot(); !reflect.DeepEqual(got, wantTrace) {
		t.Fatalf("failure event order = %v, want %v", got, wantTrace)
	}
}

func TestApplyActionsWritesOnlyForSuccessfulTodoistMutations(t *testing.T) {
	actions := replayTestActions()
	confirmToken := "confirm"
	store := &fakeReplayStore{applied: map[string]bool{
		makeReplayKey(confirmToken, 0, actions[0]): true,
	}}
	server := newApplyTestServer(t, func(content string) int {
		if content == "B" {
			return http.StatusBadRequest
		}
		return http.StatusOK
	})
	defer server.Close()
	ctx := newApplyTestContext(t.TempDir(), server.URL)

	results, err := applyActionsWithReplayStore(ctx, confirmToken, actions, applyErrorModeContinue, store)
	if err != nil {
		t.Fatalf("applyActionsWithReplayStore: %v", err)
	}
	if len(results) != 3 || !results[0].SkippedReplay || results[1].Error == nil || results[2].Error != nil {
		t.Fatalf("unexpected results: %#v", results)
	}
	if store.recordCalls != 1 {
		t.Fatalf("RecordApplied calls = %d, want 1", store.recordCalls)
	}
}

func TestAgentApplyCorruptedReplayJournalFailsClosedInContinueMode(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.json")
	plan := `{"version":1,"instruction":"add","confirm_token":"confirm","summary":{"tasks":1,"projects":0,"sections":0,"labels":0,"comments":0},"actions":[{"type":"task_add","content":"A"}]}`
	if err := os.WriteFile(planPath, []byte(plan), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent_replay.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("write corrupted journal: %v", err)
	}
	var requestCount atomic.Int32
	server := newApplyTestServer(t, func(string) int {
		requestCount.Add(1)
		return http.StatusOK
	})
	defer server.Close()
	ctx := newApplyTestContext(dir, server.URL)
	ctx.Mode = output.ModeJSON
	ctx.Stdout = &bytes.Buffer{}
	var progress bytes.Buffer
	ctx.Progress = &progressSink{out: &progress}

	err := agentApply(ctx, []string{"--plan", planPath, "--confirm", "confirm", "--on-error", "continue"})
	if err == nil {
		t.Fatal("expected corrupted journal to fail apply")
	}
	if got := requestCount.Load(); got != 0 {
		t.Fatalf("corrupted journal dispatched %d remote requests", got)
	}
	if _, err := os.Stat(lastPlanPath(ctx)); !os.IsNotExist(err) {
		t.Fatalf("last plan should not be written, stat error = %v", err)
	}
	events := progressLines(t, progress.String())
	if !containsEvent(events, "agent_apply_error") || containsEvent(events, "agent_apply_complete") {
		t.Fatalf("unexpected command events: %v", eventTypes(events))
	}
}

func TestShouldAbortApply(t *testing.T) {
	remoteErr := errors.New("remote failure")
	replayErr := &replayStoreError{err: errors.New("replay failure")}
	tests := []struct {
		name string
		mode applyErrorMode
		err  error
		want bool
	}{
		{name: "nil error", mode: "continue", err: nil, want: false},
		{name: "default mode", mode: "", err: remoteErr, want: true},
		{name: "fail mode", mode: "fail", err: remoteErr, want: true},
		{name: "unknown mode", mode: "unknown", err: remoteErr, want: true},
		{name: "continue remote error", mode: applyErrorModeContinue, err: remoteErr, want: false},
		{name: "continue replay error", mode: applyErrorModeContinue, err: replayErr, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldAbortApply(test.mode, test.err); got != test.want {
				t.Fatalf("shouldAbortApply(%q, %v) = %v, want %v", test.mode, test.err, got, test.want)
			}
		})
	}
}

type fakeReplayStore struct {
	applied     map[string]bool
	recordErr   error
	recordCalls int
	trace       *applyTrace
}

func (s *fakeReplayStore) Contains(key string) bool {
	return s.applied[key]
}

func (s *fakeReplayStore) RecordApplied(key string, _ time.Time) error {
	s.recordCalls++
	if s.trace != nil {
		s.trace.Add("replay_record")
	}
	if s.recordErr != nil {
		return s.recordErr
	}
	s.applied[key] = true
	return nil
}

type applyEventTraceWriter struct {
	trace  *applyTrace
	events *[]map[string]any
}

func (w *applyEventTraceWriter) Write(data []byte) (int, error) {
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &event); err != nil {
		return 0, err
	}
	eventType, _ := event["type"].(string)
	w.trace.Add(eventType)
	*w.events = append(*w.events, event)
	return len(data), nil
}

type applyTrace struct {
	mu      sync.Mutex
	entries []string
}

func (t *applyTrace) Add(entry string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, entry)
}

func (t *applyTrace) Snapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.entries...)
}

func newApplyTestServer(t *testing.T, status func(content string) int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if code := status(payload.Content); code != http.StatusOK {
			http.Error(w, `{"error":"injected failure"}`, code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         fmt.Sprintf("task-%s", payload.Content),
			"content":    payload.Content,
			"project_id": "p1",
			"priority":   1,
		})
	}))
}

func newApplyTestContext(dir, baseURL string) *Context {
	return &Context{
		Stdout:     io.Discard,
		Stderr:     io.Discard,
		Stdin:      bytes.NewReader(nil),
		Now:        func() time.Time { return time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC) },
		Token:      "token",
		Client:     api.NewClient(baseURL, "token", time.Second),
		Config:     config.Config{TimeoutSeconds: 2},
		ConfigPath: filepath.Join(dir, "config.json"),
	}
}

func replayTestActions() []Action {
	return []Action{
		{Type: "task_add", Content: "A"},
		{Type: "task_add", Content: "B"},
		{Type: "task_add", Content: "C"},
	}
}

func assertReplayKeys(t *testing.T, ctx *Context, confirmToken string, actions []Action, present []int) {
	t.Helper()
	store, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatalf("load replay store: %v", err)
	}
	want := make(map[int]bool, len(present))
	for _, idx := range present {
		want[idx] = true
	}
	for idx, action := range actions {
		got := store.Contains(makeReplayKey(confirmToken, idx, action))
		if got != want[idx] {
			t.Fatalf("replay presence for action %d = %v, want %v", idx, got, want[idx])
		}
	}
}
