package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
	appreview "github.com/agisilaos/todoist-cli/internal/app/review"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type reviewFixture struct {
	mu        sync.Mutex
	tasks     map[string]map[string]any
	writes    []string
	failMove  bool
	uncertain bool
	pages     int
}

func newReviewFixture(t *testing.T) (*reviewFixture, *Context) {
	t.Helper()
	fixture := &reviewFixture{tasks: map[string]map[string]any{}}
	for i := 1; i <= 4; i++ {
		id := fmt.Sprint(i)
		fixture.tasks[id] = map[string]any{"id": id, "content": "Task " + id, "description": "Details", "project_id": "p1", "labels": []string{}, "priority": 1, "updated_at": "v1", "due": map[string]any{"date": fmt.Sprintf("2026-09-%02d", 20+i), "is_recurring": i == 3}}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/tasks/filter" {
			fixture.pages++
			ids := []string{"4", "2"}
			next := "second"
			if r.URL.Query().Get("cursor") == "second" {
				ids = []string{"3", "1", "2"}
				next = ""
			}
			tasks := []any{}
			for _, id := range ids {
				tasks = append(tasks, fixture.tasks[id])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": tasks, "next_cursor": next})
			return
		}
		pieces := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pieces) < 2 || pieces[0] != "tasks" {
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
			return
		}
		task, ok := fixture.tasks[pieces[1]]
		if !ok {
			http.Error(w, "missing", 404)
			return
		}
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(task)
			return
		}
		fixture.writes = append(fixture.writes, r.URL.Path)
		if len(pieces) == 3 && pieces[2] == "move" && fixture.failMove {
			fixture.failMove = false
			http.Error(w, "move rejected", 400)
			return
		}
		if fixture.uncertain {
			http.Error(w, "uncertain", 408)
			return
		}
		if len(pieces) == 3 && pieces[2] == "close" {
			task["checked"] = true
		} else {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			for key, value := range payload {
				task[key] = value
			}
		}
		task["updated_at"] = fmt.Sprintf("v%d", len(fixture.writes)+1)
		_ = json.NewEncoder(w).Encode(task)
	}))
	t.Cleanup(server.Close)
	ctx := newApplyTestContext(t.TempDir(), server.URL)
	ctx.Mode = output.ModeJSON
	ctx.Stdout = &bytes.Buffer{}
	ctx.Stderr = &bytes.Buffer{}
	return fixture, ctx
}
func runScriptedReview(t *testing.T, ctx *Context, script, out string) error {
	t.Helper()
	operation, cancel := context.WithCancel(context.Background())
	defer cancel()
	return runReview(ctx, "overdue | today", out, newReviewInput(operation, strings.NewReader(script)))
}
func readReviewReport(t *testing.T, ctx *Context) reviewReport {
	t.Helper()
	var report reviewReport
	if err := json.Unmarshal(ctx.Stdout.(*bytes.Buffer).Bytes(), &report); err != nil {
		t.Fatalf("report: %v %s", err, ctx.Stdout)
	}
	return report
}

func TestReviewCancelAndPreview(t *testing.T) {
	for _, test := range []struct {
		name, ending string
		dry          bool
	}{{"cancel", "no\n", false}, {"dry", "", true}, {"EOF", "", false}} {
		t.Run(test.name, func(t *testing.T) {
			fixture, ctx := newReviewFixture(t)
			ctx.Global.DryRun = test.dry
			err := runScriptedReview(t, ctx, "\ninvalid\nkeep\nchange\ndue-date\ninvalid\ndue-date\n2026-10-01\ndone\ncomplete\nskip\n"+test.ending, "")
			if test.name == "EOF" {
				if err == nil {
					t.Fatal("expected EOF failure")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(fixture.writes) != 0 {
				t.Fatalf("mutations before confirmation: %v", fixture.writes)
			}
			report := readReviewReport(t, ctx)
			if len(report.Tasks) != 4 || fixture.pages != 2 {
				t.Fatalf("selection %+v pages=%d", report, fixture.pages)
			}
			if report.Tasks[0].Outcome != "kept" || report.Tasks[3].Outcome != "skipped" {
				t.Fatalf("accounting %+v", report.Tasks)
			}
			if test.dry && report.Tasks[1].Outcome != "proposed" {
				t.Fatal("preview mislabeled")
			}
		})
	}
}

func TestReviewApplyMoveFailureAndExportedRetry(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	fixture.failMove = true
	out := filepath.Join(t.TempDir(), "review.json")
	err := runScriptedReview(t, ctx, "keep\nchange\ncontent\nChanged\nproject\nid:p2\ndone\ncomplete\nskip\nyes\n", out)
	if err == nil {
		t.Fatal("expected move failure")
	}
	report := readReviewReport(t, ctx)
	if report.Tasks[1].Outcome != "partially_applied" || report.Tasks[2].Outcome != "unattempted" {
		t.Fatalf("bad partial accounting: %+v", report.Tasks)
	}
	if len(fixture.writes) != 2 {
		t.Fatal(fixture.writes)
	}
	plan, err := readPlanFile(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Stdout = &bytes.Buffer{}
	if err := agentApply(ctx, []string{"--plan", out, "--confirm", plan.ConfirmToken}); err != nil {
		t.Fatal(err)
	}
	retry := readReviewReport(t, ctx)
	if retry.Tasks[1].Outcome != "applied" || !retry.Tasks[1].Actions[0].Replayed || retry.Tasks[2].Outcome != "applied" {
		t.Fatalf("retry %+v", retry.Tasks)
	}
	if len(fixture.writes) != 4 || fixture.tasks["2"]["project_id"] != "p2" || fixture.tasks["2"]["content"] != "Changed" {
		t.Fatalf("state: %+v writes=%v", fixture.tasks, fixture.writes)
	}
	ctx.Stdout = &bytes.Buffer{}
	if err := agentApply(ctx, []string{"--plan", out, "--confirm", plan.ConfirmToken}); err != nil {
		t.Fatal(err)
	}
	if len(fixture.writes) != 4 {
		t.Fatal("replayed mutations")
	}
}

func fixtureReviewPlan(t *testing.T, ctx *Context) Plan {
	t.Helper()
	snapshot, err := fetchReviewTask(ctx, "2")
	if err != nil {
		t.Fatal(err)
	}
	return Plan{Version: 1, ConfirmToken: "review-test", Actions: []Action{{Type: "task_update", TaskID: "2", Content: "changed"}, {Type: "task_move", TaskID: "2", ProjectID: "p2"}}, Review: &coreagent.Review{Version: 1, Tasks: []coreagent.ReviewTask{{ID: "2", Content: "Task 2", Disposition: "change", Snapshot: snapshot, Actions: []int{0, 1}}}}}

}

func TestReviewStaleAndMissingPreflight(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			fixture, ctx := newReviewFixture(t)
			plan := fixtureReviewPlan(t, ctx)
			fixture.mu.Lock()
			if missing {
				delete(fixture.tasks, "2")
			} else {
				fixture.tasks["2"]["content"] = "edited elsewhere"
			}
			fixture.mu.Unlock()
			_, err := applyReviewPlan(ctx, plan)
			if err == nil || len(fixture.writes) != 0 {
				t.Fatalf("err=%v writes=%v", err, fixture.writes)
			}
		})
	}
}
func TestReviewPartialRetryChecksCheckpoint(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	fixture.failMove = true
	plan := fixtureReviewPlan(t, ctx)
	if _, err := applyReviewPlan(ctx, plan); err == nil {
		t.Fatal("expected failure")
	}
	fixture.mu.Lock()
	fixture.tasks["2"]["description"] = "external change"
	fixture.mu.Unlock()
	if _, err := applyReviewPlan(ctx, plan); toExitCode(err) != exitConflict {
		t.Fatalf("expected stale conflict, got %v", err)
	}
	if len(fixture.writes) != 2 {
		t.Fatal("stale retry sent mutation")
	}
}
func TestReviewUncertainWriteBlocksRetry(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	fixture.uncertain = true
	plan := fixtureReviewPlan(t, ctx)
	if _, err := applyReviewPlan(ctx, plan); err == nil {
		t.Fatal("expected failure")
	}
	count := len(fixture.writes)
	if _, err := applyReviewPlan(ctx, plan); err == nil || !strings.Contains(err.Error(), "uncertain") {
		t.Fatalf("unsafe retry: %v", err)
	}
	if len(fixture.writes) != count {
		t.Fatal("uncertain mutation repeated")
	}
}
func TestReviewReadOnlyPreviewAndApply(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	report := currentAuthorization(ctx)
	report.WriteCapable = false
	mode := "read-only"
	report.Mode = &mode
	ctx.Authorization = &report
	out := filepath.Join(t.TempDir(), "plan.json")
	if err := runScriptedReview(t, ctx, "keep\nchange\ncontent\nChanged\ndone\ncomplete\nskip\n", out); err != nil {
		t.Fatal(err)
	}
	if len(fixture.writes) != 0 {
		t.Fatal("read-only mutated")
	}
	plan, err := readPlanFile(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyReviewPlan(ctx, plan); toExitCode(err) != exitAuth {
		t.Fatalf("apply permission: %v", err)
	}
}
func TestReviewNoInputAndHelp(t *testing.T) {
	_, ctx := newReviewFixture(t)
	for _, args := range [][]string{nil, {"--filter", "today"}} {
		if err := reviewCommand(ctx, args); toExitCode(err) != exitUsage {
			t.Fatal(err)
		}
	}
	ctx.Global.NoInput = true
	if err := reviewCommand(ctx, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
}
func TestReviewPlanSaveNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveReviewPlan(path, Plan{}); err == nil {
		t.Fatal("overwrote existing plan")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal(string(data))
	}
}
func TestReviewSnapshotIgnoresUnrelatedFields(t *testing.T) {
	a := appreview.Snapshot{"id": json.RawMessage(`"a"`), "content": json.RawMessage(`"x"`), "unrelated": json.RawMessage(`1`)}
	b := appreview.Snapshot{"id": json.RawMessage(`"a"`), "content": json.RawMessage(`"x"`), "unrelated": json.RawMessage(`2`)}
	if !appreview.Equal(appreview.SnapshotOf(a), appreview.SnapshotOf(b)) {
		t.Fatal("unrelated difference")
	}
}
func TestReviewReplayPersistenceFailureRetainsPending(t *testing.T) {
	_, ctx := newReviewFixture(t)
	plan := fixtureReviewPlan(t, ctx)
	file, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewReplayStore{fileReplayStore: file, ctx: ctx, plan: plan}
	if err := store.before(0, plan.Actions[0]); err != nil {
		t.Fatal(err)
	}
	store.response = plan.Review.Tasks[0].Snapshot
	store.persist = func(string, replayJournal) error { return errors.New("disk full") }
	if err := store.RecordApplied(makeReplayKey(plan.ConfirmToken, 0, plan.Actions[0]), ctx.Now()); err == nil {
		t.Fatal("expected storage failure")
	}
	reloaded, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.journal.Reviews[store.taskKey("2")].Pending || reloaded.Contains(makeReplayKey(plan.ConfirmToken, 0, plan.Actions[0])) {
		t.Fatal("failed recording declared success")
	}
}

func TestReviewAllEditsReachPayload(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	script := "keep\nchange\ncontent\nNew title\ndescription\nNew description\nlabels\nwork, home\npriority\np1\ndue-datetime\n2026-10-01T10:30:00+02:00\ndue-lang\nen\nduration\n30 minute\ndeadline\n2026-10-02\nassignee\nid:user2\nsection\nid:s2\ndone\nkeep\nskip\nyes\n"
	if err := runScriptedReview(t, ctx, script, ""); err != nil {
		t.Fatal(err)
	}
	task := fixture.tasks["2"]
	for key, want := range map[string]any{"content": "New title", "description": "New description", "priority": float64(4), "due_datetime": "2026-10-01T10:30:00+02:00", "due_lang": "en", "duration": float64(30), "duration_unit": "minute", "deadline_date": "2026-10-02", "assignee_id": "user2", "section_id": "s2"} {
		if task[key] != want {
			t.Errorf("%s=%v want %v", key, task[key], want)
		}
	}
	labels, ok := task["labels"].([]any)
	if !ok || len(labels) != 2 || labels[0] != "work" || labels[1] != "home" {
		t.Fatalf("labels=%v", task["labels"])
	}
	if len(fixture.writes) != 2 {
		t.Fatal(fixture.writes)
	}
}

func TestReviewAgentRunCannotBypassStaleness(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	plan := fixtureReviewPlan(t, ctx)
	out := filepath.Join(t.TempDir(), "plan.json")
	if err := saveReviewPlan(out, plan); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	fixture.tasks["2"]["updated_at"] = "external"
	fixture.mu.Unlock()
	if err := agentRun(ctx, []string{"--plan", out, "--confirm", plan.ConfirmToken}); toExitCode(err) != exitConflict {
		t.Fatal(err)
	}
	if len(fixture.writes) != 0 {
		t.Fatal("run bypassed review precondition")
	}
}

func TestReviewNoOpPlanAndContinueRejection(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	ctx.Global.DryRun = true
	out := filepath.Join(t.TempDir(), "plan.json")
	if err := runScriptedReview(t, ctx, "keep\nskip\nkeep\nskip\n", out); err != nil {
		t.Fatal(err)
	}
	plan, err := readPlanFile(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Global.DryRun = false
	ctx.Stdout = &bytes.Buffer{}
	if err := agentApply(ctx, []string{"--plan", out, "--confirm", plan.ConfirmToken}); err != nil {
		t.Fatal(err)
	}
	if len(fixture.writes) != 0 {
		t.Fatal("no-op mutated")
	}
	if err := agentApply(ctx, []string{"--plan", out, "--confirm", plan.ConfirmToken, "--on-error", "continue"}); toExitCode(err) != exitUsage {
		t.Fatal(err)
	}
}

func TestReviewCancelledContextAccountsForAllTasks(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	operation, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runReview(ctx, "today", "", newReviewInput(operation, strings.NewReader(""))); err != nil {
		t.Fatal(err)
	}
	report := readReviewReport(t, ctx)
	if len(fixture.writes) != 0 || report.Counts["unattempted"] != 4 {
		t.Fatalf("%+v", report)
	}
}

func TestReviewPaginationLoopAndEmpty(t *testing.T) {
	for _, loop := range []bool{false, true} {
		t.Run(fmt.Sprint(loop), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				cursor := ""
				if loop {
					cursor = "repeat"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "next_cursor": cursor})
			}))
			defer server.Close()
			ctx := newApplyTestContext(t.TempDir(), server.URL)
			ctx.Mode = output.ModeJSON
			ctx.Stdout = &bytes.Buffer{}
			err := runScriptedReview(t, ctx, "", "")
			if loop {
				if err == nil || calls != 2 {
					t.Fatalf("err=%v calls=%d", err, calls)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				report := readReviewReport(t, ctx)
				if report.Phase != "empty" || len(report.Tasks) != 0 {
					t.Fatal(report)
				}
			}
		})
	}
}

func TestReviewReservedOutputPaths(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	for _, name := range []string{filepath.Base(ctx.ConfigPath), "agent_replay.json", "last_plan.json", "credentials.json", "agent_policy.json"} {
		if err := runScriptedReview(t, ctx, "", filepath.Join(filepath.Dir(ctx.ConfigPath), name)); toExitCode(err) != exitUsage {
			t.Fatalf("%s: %v", name, err)
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Dir(ctx.ConfigPath), alias); err != nil {
		t.Fatal(err)
	}
	if err := validateReviewOutputPath(ctx, filepath.Join(alias, "agent_replay.json")); toExitCode(err) != exitUsage {
		t.Fatal(err)
	}
	if fixture.pages != 0 || len(fixture.writes) != 0 {
		t.Fatal("reserved output reached API")
	}
}
func TestReviewSelectionCancellationSucceeds(t *testing.T) {
	_, ctx := newReviewFixture(t)
	operation, cancel := context.WithCancel(context.Background())
	cancel()
	ctx.OperationContext = operation
	if err := runReview(ctx, "today", "", newReviewInput(operation, strings.NewReader(""))); err != nil {
		t.Fatal(err)
	}
	report := readReviewReport(t, ctx)
	if report.Phase != "cancelled" {
		t.Fatal(report.Phase)
	}
}
func TestReviewUnresolvedMoveNameReprompts(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	if err := runScriptedReview(t, ctx, "keep\nchange\nproject\nTypo destination\nproject\nid:p2\ndone\nkeep\nskip\nyes\n", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), "0 matches") {
		t.Fatal("unresolved name accepted")
	}
	if fixture.tasks["2"]["project_id"] != "p2" || len(fixture.writes) != 1 {
		t.Fatal(fixture.writes)
	}
}

func TestReviewAgentRunExportCannotFollowSymlink(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	plan := fixtureReviewPlan(t, ctx)
	source := filepath.Join(t.TempDir(), "plan.json")
	if err := saveReviewPlan(source, plan); err != nil {
		t.Fatal(err)
	}
	journal := replayJournalPath(ctx)
	if err := os.WriteFile(journal, []byte(`{"applied":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "export.json")
	if err := os.Symlink(journal, alias); err != nil {
		t.Fatal(err)
	}
	if err := agentRun(ctx, []string{"--plan", source, "--out", alias, "--confirm", plan.ConfirmToken}); err == nil {
		t.Fatal("followed output symlink")
	}
	data, _ := os.ReadFile(journal)
	if string(data) != `{"applied":{}}` || len(fixture.writes) != 0 {
		t.Fatalf("journal=%s writes=%v", data, fixture.writes)
	}
}
