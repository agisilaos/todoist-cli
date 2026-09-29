package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	appreview "github.com/agisilaos/todoist-cli/internal/app/review"
)

func loadReviewTestStore(t *testing.T, ctx *Context, plan Plan) *reviewReplayStore {
	t.Helper()
	file, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return &reviewReplayStore{fileReplayStore: file, ctx: ctx, plan: plan}
}

func reportReviewTestResults(t *testing.T, ctx *Context, plan Plan, results []applyResult, cause error) reviewReport {
	t.Helper()
	ctx.Stdout = &bytes.Buffer{}
	if err := writeReviewReport(ctx, plan, results, "applied", "", cause); err != nil {
		t.Fatal(err)
	}
	return readReviewReport(t, ctx)
}

func TestReviewPersistenceFailureBoundaries(t *testing.T) {
	for _, test := range []struct {
		name                                        string
		failWrite, mutations, results, pendingIndex int
		pending                                     bool
	}{
		{"pending", 1, 0, 0, 0, false},
		{"record", 2, 1, 1, 0, true},
		{"clear rejection", 4, 2, 2, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, ctx := newReviewFixture(t)
			fixture.failMove = true
			plan := fixtureReviewPlan(t, ctx)
			if err := persistReplayJournal(replayJournalPath(ctx), emptyReplayJournal()); err != nil {
				t.Fatal(err)
			}
			store := loadReviewTestStore(t, ctx, plan)
			writes := 0
			injected := errors.New("injected persistence failure")
			var beforeFailure []byte
			store.persist = func(path string, journal replayJournal) error {
				writes++
				if writes == test.failWrite {
					var err error
					beforeFailure, err = os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					return injected
				}
				return persistReplayJournal(path, journal)
			}
			results, err := applyActionsWithPreparation(ctx, plan.ConfirmToken, plan.Actions, applyErrorModeFail, store.fileReplayStore, store.prepare)
			if !errors.Is(err, injected) || !shouldAbortApply(applyErrorModeContinue, err) {
				t.Fatalf("expected terminal persistence failure, got %v", err)
			}
			if writes != test.failWrite || len(fixture.writes) != test.mutations || len(results) != test.results {
				t.Fatalf("writes=%d mutations=%v results=%+v", writes, fixture.writes, results)
			}
			if len(results) > 0 && results[len(results)-1].Error == nil {
				t.Fatal("failed recording reported success")
			}
			after, readErr := os.ReadFile(store.path)
			if readErr != nil || !bytes.Equal(beforeFailure, after) {
				t.Fatalf("failed persistence changed disk: %v", readErr)
			}
			reloaded := loadReviewTestStore(t, ctx, plan)
			memory, marshalErr := json.Marshal(store.journal)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			disk, marshalErr := json.Marshal(reloaded.journal)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if !bytes.Equal(memory, disk) {
				t.Fatal("memory and disk diverged after failure")
			}
			checkpoint := reloaded.journal.Reviews[store.taskKey("2")]
			if checkpoint.Pending != test.pending || checkpoint.PendingIndex != test.pendingIndex {
				t.Fatalf("checkpoint=%+v", checkpoint)
			}
			wantApplied := 0
			if test.pendingIndex == 1 {
				wantApplied = 1
			}
			if reloaded.Len() != wantApplied {
				t.Fatalf("recorded %d actions, want %d", reloaded.Len(), wantApplied)
			}
			if got := appreview.Text(checkpoint.Snapshot, "content"); test.pending && got != []string{"Task 2", "changed"}[test.pendingIndex] {
				t.Fatalf("checkpoint content=%q", got)
			}
			report := reportReviewTestResults(t, ctx, plan, results, err)
			outcome := report.Tasks[0].Actions[test.pendingIndex]
			wantOutcome := "failed"
			if test.failWrite == 1 {
				wantOutcome = "unattempted"
			}
			if outcome.Outcome != wantOutcome || outcome.RemoteOutcomeUncertain != test.pending {
				t.Fatalf("report=%+v", report)
			}
			if test.pending {
				if _, retryErr := applyReviewPlan(ctx, plan); toExitCode(retryErr) != exitConflict {
					t.Fatalf("retry=%v", retryErr)
				}
				if len(fixture.writes) != test.mutations {
					t.Fatal("retry dispatched a pending action")
				}
			}
		})
	}
}

func TestReviewFailedOutcomeEvidence(t *testing.T) {
	for _, test := range []struct {
		name    string
		cause   error
		pending bool
	}{
		{"400", &api.APIError{Status: 400}, false},
		{"403", &api.APIError{Status: 403}, false},
		{"404", &api.APIError{Status: 404}, false},
		{"429", &api.APIError{Status: 429}, false},
		{"wrapped rejection", fmt.Errorf("request: %w", &api.APIError{Status: 422}), false},
		{"408", &api.APIError{Status: 408}, true},
		{"500", &api.APIError{Status: 500}, true},
		{"transport", errors.New("connection lost"), true},
		{"cancelled", context.Canceled, true},
		{"decode", errors.New("decode response"), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, ctx := newReviewFixture(t)
			plan := fixtureReviewPlan(t, ctx)
			store := loadReviewTestStore(t, ctx, plan)
			attempts, writes := 0, 0
			store.persist = func(path string, journal replayJournal) error { writes++; return persistReplayJournal(path, journal) }
			// Inject the final request outcome, independently of HTTP retry timing.
			prepare := func(index int, action Action) (preparedAction, error) {
				attempt, err := store.prepare(index, action)
				attempt.perform = func() error { attempts++; return test.cause }
				return attempt, err
			}
			results, err := applyActionsWithPreparation(ctx, plan.ConfirmToken, plan.Actions, applyErrorModeFail, store.fileReplayStore, prepare)
			if !errors.Is(err, test.cause) || attempts != 1 || len(results) != 1 || results[0].Error == nil {
				t.Fatalf("err=%v attempts=%d results=%+v", err, attempts, results)
			}
			reloaded := loadReviewTestStore(t, ctx, plan)
			checkpoint := reloaded.journal.Reviews[store.taskKey("2")]
			wantWrites := 2
			if test.pending {
				wantWrites = 1
			}
			if checkpoint.Pending != test.pending || checkpoint.PendingIndex != 0 || reloaded.Len() != 0 || writes != wantWrites {
				t.Fatalf("journal=%+v writes=%d", reloaded.journal, writes)
			}
			report := reportReviewTestResults(t, ctx, plan, results, err)
			if report.Tasks[0].Outcome != "failed" || report.Tasks[0].Actions[0].RemoteOutcomeUncertain != test.pending {
				t.Fatalf("report=%+v", report)
			}
			_, retryErr := applyReviewPlan(ctx, plan)
			if test.pending {
				if toExitCode(retryErr) != exitConflict || len(fixture.writes) != 0 {
					t.Fatalf("unsafe retry: %v writes=%v", retryErr, fixture.writes)
				}
			} else if retryErr != nil || len(fixture.writes) != 2 {
				t.Fatalf("definite rejection retry: %v writes=%v", retryErr, fixture.writes)
			}
		})
	}
}

func TestReviewPublishesCheckpointWithReplayRecord(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	fixture.failMove = true
	plan := fixtureReviewPlan(t, ctx)
	store := loadReviewTestStore(t, ctx, plan)
	var publications []replayJournal
	store.persist = func(path string, journal replayJournal) error {
		publications = append(publications, journal)
		return persistReplayJournal(path, journal)
	}
	results, err := applyActionsWithPreparation(ctx, plan.ConfirmToken, plan.Actions, applyErrorModeFail, store.fileReplayStore, store.prepare)
	if err == nil || len(results) != 2 || results[0].Error != nil || results[1].Error == nil || len(fixture.writes) != 2 {
		t.Fatalf("results=%+v err=%v writes=%v", results, err, fixture.writes)
	}
	if len(publications) != 4 {
		t.Fatalf("publications=%d", len(publications))
	}
	key := makeReplayKey(plan.ConfirmToken, 0, plan.Actions[0])
	taskKey := store.taskKey("2")
	for i, publication := range publications {
		checkpoint := publication.Reviews[taskKey]
		if checkpoint.Pending != (i == 0 || i == 2) {
			t.Fatalf("publication %d: %+v", i, checkpoint)
		}
		if i == 0 {
			if len(publication.Applied) != 0 || appreview.Text(checkpoint.Snapshot, "content") != "Task 2" {
				t.Fatalf("pending publication: %+v", publication)
			}
		} else if publication.Applied[key] != ctx.Now().UTC().Format(time.RFC3339) || appreview.Text(checkpoint.Snapshot, "content") != "changed" {
			t.Fatalf("checkpoint/replay publication %d: %+v", i, publication)
		}
	}
	// Preflight reporting must retain the already recorded update on a stale retry.
	fixture.mu.Lock()
	fixture.tasks["2"]["description"] = "external edit"
	fixture.mu.Unlock()
	ctx.Stdout = &bytes.Buffer{}
	if err := applyReviewAndReport(ctx, plan, "", "fail", "agent apply"); toExitCode(err) != exitConflict {
		t.Fatalf("stale retry=%v", err)
	}
	report := readReviewReport(t, ctx)
	if report.Tasks[0].Outcome != "partially_applied" || !report.Tasks[0].Actions[0].Replayed || report.Tasks[0].Actions[1].Outcome != "unattempted" || len(fixture.writes) != 2 {
		t.Fatalf("report=%+v writes=%v", report, fixture.writes)
	}
}

func TestReviewMissingMutationSnapshotRetainsPending(t *testing.T) {
	for _, response := range []string{"", `{}`, `{"id":"other","content":"changed"}`, `{"id":"2"}`, `{bad json`} {
		t.Run(response, func(t *testing.T) {
			fixture, ctx := newReviewFixture(t)
			fixture.updateResponse = &response
			plan := fixtureReviewPlan(t, ctx)
			err := applyReviewAndReport(ctx, plan, "", "fail", "agent apply")
			if err == nil || len(fixture.writes) != 1 {
				t.Fatalf("err=%v writes=%v", err, fixture.writes)
			}
			store := loadReviewTestStore(t, ctx, plan)
			if !store.journal.Reviews[store.taskKey("2")].Pending || store.Len() != 0 {
				t.Fatalf("journal=%+v", store.journal)
			}
			report := readReviewReport(t, ctx)
			if report.Tasks[0].Outcome != "failed" || !report.Tasks[0].Actions[0].RemoteOutcomeUncertain || report.Tasks[0].Actions[1].Outcome != "unattempted" {
				t.Fatalf("report=%+v", report)
			}
			if _, err := applyReviewPlan(ctx, plan); err == nil || !strings.Contains(err.Error(), "uncertain") || len(fixture.writes) != 1 {
				t.Fatalf("unsafe retry: %v writes=%v", err, fixture.writes)
			}
		})
	}
}

func TestReviewReplayDoesNotWrite(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	plan := fixtureReviewPlan(t, ctx)
	if err := applyReviewAndReport(ctx, plan, "", "fail", "agent apply"); err != nil {
		t.Fatal(err)
	}
	store := loadReviewTestStore(t, ctx, plan)
	store.persist = func(string, replayJournal) error { t.Fatal("replay wrote journal"); return nil }
	results, err := applyActionsWithPreparation(ctx, plan.ConfirmToken, plan.Actions, applyErrorModeFail, store.fileReplayStore, store.prepare)
	if err != nil || len(results) != 2 || !results[0].SkippedReplay || !results[1].SkippedReplay {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	paths := []string{replayJournalPath(ctx), lastPlanPath(ctx)}
	before := make([]os.FileInfo, len(paths))
	data := make([][]byte, len(paths))
	for i, path := range paths {
		before[i], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		data[i], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx.Stdout = &bytes.Buffer{}
	if err := applyReviewAndReport(ctx, plan, "", "fail", "agent apply"); err != nil {
		t.Fatal(err)
	}
	for i, path := range paths {
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		afterData, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before[i], after) || !bytes.Equal(data[i], afterData) {
			t.Fatalf("replay rewrote %s", path)
		}
	}
	report := readReviewReport(t, ctx)
	if len(fixture.writes) != 2 || report.Tasks[0].Outcome != "applied" || !report.Tasks[0].Actions[0].Replayed || !report.Tasks[0].Actions[1].Replayed {
		t.Fatalf("report=%+v writes=%v", report, fixture.writes)
	}
}

func TestReviewJournalPreservesExistingEvidence(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	plan := fixtureReviewPlan(t, ctx)
	original := []byte(`{"applied":{"old-action":"2025-01-02T03:04:05Z"},"reviews":{"old-review":{"pending":true,"pending_index":2,"snapshot":{"id":"old","content":"Keep this evidence"}}},"future_field":true}`)
	if err := os.WriteFile(replayJournalPath(ctx), original, 0600); err != nil {
		t.Fatal(err)
	}
	store := loadReviewTestStore(t, ctx, plan)
	prior := store.journal.Reviews["old-review"]
	if err := store.fileReplayStore.RecordApplied("ordinary-action", ctx.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := applyReviewPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	reloaded := loadReviewTestStore(t, ctx, plan)
	if len(fixture.writes) != 2 || reloaded.Len() != 4 || reloaded.journal.Applied["old-action"] != "2025-01-02T03:04:05Z" || !reloaded.Contains("ordinary-action") || !reflect.DeepEqual(reloaded.journal.Reviews["old-review"], prior) {
		t.Fatalf("lost existing evidence: %+v", reloaded.journal)
	}
}
