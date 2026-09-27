package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	appreview "github.com/agisilaos/todoist-cli/internal/app/review"
)

// A checkpoint and its action's replay record are replaced in one journal write.
// Pending is persisted BEFORE dispatch so interruption cannot silently retry a write.
type reviewCheckpoint struct {
	PendingIndex int                `json:"pending_index,omitempty"`
	Snapshot     appreview.Snapshot `json:"snapshot"`
	Pending      bool               `json:"pending,omitempty"`
}

type reviewReplayStore struct {
	keyPrefix string
	*fileReplayStore
	ctx      *Context
	plan     Plan
	index    int
	response appreview.Snapshot
}

func (s *reviewReplayStore) taskKey(id string) string {
	if s.keyPrefix == "" {
		data, _ := json.Marshal(struct {
			Token   string
			Actions []Action
			Review  any
		}{s.plan.ConfirmToken, s.plan.Actions, s.plan.Review})
		sum := sha256.Sum256(data)
		s.keyPrefix = hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256([]byte(s.keyPrefix + ":" + id))
	return hex.EncodeToString(sum[:])
}

func (s *reviewReplayStore) save(key string, checkpoint reviewCheckpoint, applied string, at time.Time) error {
	candidate := replayJournal{Applied: map[string]string{}, Reviews: map[string]reviewCheckpoint{}}
	for k, v := range s.journal.Applied {
		candidate.Applied[k] = v
	}
	for k, v := range s.journal.Reviews {
		candidate.Reviews[k] = v
	}
	candidate.Reviews[key] = checkpoint
	if applied != "" {
		candidate.Applied[applied] = at.UTC().Format(time.RFC3339)
	}
	if err := s.persist(s.path, candidate); err != nil {
		return err
	}
	s.journal = candidate
	return nil
}

func fetchReviewTask(ctx *Context, id string) (appreview.Snapshot, error) {
	req, cancel := requestContext(ctx)
	defer cancel()
	var task appreview.Snapshot
	_, err := ctx.Client.Get(req, "/tasks/"+url.PathEscape(id), nil, &task)
	if err != nil {
		return nil, err
	}
	if appreview.Text(task, "id") != id {
		return nil, fmt.Errorf("unexpected task response for %s", id)
	}
	return appreview.SnapshotOf(task), nil
}

func (s *reviewReplayStore) expected(id string) (reviewCheckpoint, error) {
	if checkpoint, ok := s.journal.Reviews[s.taskKey(id)]; ok {
		if checkpoint.Pending {
			return checkpoint, &CodeError{Code: exitConflict, Err: fmt.Errorf("task %s has an uncertain remote outcome; inspect Todoist and start a fresh review; do not retry this plan", id)}
		}
		return checkpoint, nil
	}
	for _, task := range s.plan.Review.Tasks {
		if task.ID == id {
			return reviewCheckpoint{Snapshot: task.Snapshot}, nil
		}
	}
	return reviewCheckpoint{}, fmt.Errorf("missing review snapshot for %s", id)
}

func (s *reviewReplayStore) check(id string) (reviewCheckpoint, error) {
	checkpoint, err := s.expected(id)
	if err != nil {
		return checkpoint, err
	}
	current, err := fetchReviewTask(s.ctx, id)
	if err != nil {
		return checkpoint, fmt.Errorf("cannot revalidate task %s; nothing further applied: %w", id, err)
	}
	if !appreview.Equal(checkpoint.Snapshot, current) {
		return checkpoint, &CodeError{Code: exitConflict, Err: fmt.Errorf("task %s changed since review; start a fresh review", id)}
	}
	return checkpoint, nil
}

func (s *reviewReplayStore) before(index int, action Action) error {
	checkpoint, err := s.check(action.TaskID)
	if err != nil {
		return err
	}
	s.index = index
	checkpoint.Pending = true
	checkpoint.PendingIndex = index
	if err := s.save(s.taskKey(action.TaskID), checkpoint, "", time.Time{}); err != nil {
		return &replayStoreError{err: err}
	}
	return nil
}

func (s *reviewReplayStore) failed(_ int, action Action, err error) error {
	var apiErr *api.APIError
	// Only definite client rejections are safe to retry automatically from this plan.
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 408 {
		checkpoint := s.journal.Reviews[s.taskKey(action.TaskID)]
		checkpoint.Pending = false
		if saveErr := s.save(s.taskKey(action.TaskID), checkpoint, "", time.Time{}); saveErr != nil {
			return &replayStoreError{err: fmt.Errorf("failed to record rejected action: %w", saveErr)}
		}
		return err
	}
	return fmt.Errorf("remote outcome uncertain; inspect Todoist before a fresh review: %w", err)
}

func (s *reviewReplayStore) RecordApplied(key string, at time.Time) error {
	action := s.plan.Actions[s.index]
	checkpoint := s.journal.Reviews[s.taskKey(action.TaskID)]
	// A later action on this task must compare with the observed post-action state.
	for i := s.index + 1; i < len(s.plan.Actions); i++ {
		if s.plan.Actions[i].TaskID == action.TaskID {
			if appreview.Text(s.response, "id") != action.TaskID || len(s.response) < 2 {
				return errors.New("successful mutation did not return a task snapshot; inspect remote state before a fresh review")
			}
			checkpoint.Snapshot = appreview.SnapshotOf(s.response)
			break
		}
	}
	checkpoint.Pending = false
	return s.save(s.taskKey(action.TaskID), checkpoint, key, at)
}

func applyReviewPlan(ctx *Context, plan Plan) ([]applyResult, error) {
	if err := validatePlan(plan, 1, true); err != nil {
		return nil, err
	}
	file, err := loadReplayStore(ctx)
	if err != nil {
		return nil, err
	}
	store := &reviewReplayStore{fileReplayStore: file, ctx: ctx, plan: plan}
	seen := map[string]bool{}
	for i, a := range plan.Actions {
		if store.Contains(makeReplayKey(plan.ConfirmToken, i, a)) {
			continue
		}
		if err := currentAuthorization(ctx).CheckMutation(); err != nil {
			return nil, err
		}
		if !seen[a.TaskID] {
			if _, err := store.check(a.TaskID); err != nil {
				return nil, err
			}
			seen[a.TaskID] = true
		}
	}
	return applyActionsWithHooks(ctx, plan.ConfirmToken, plan.Actions, applyErrorModeFail, store, &applyHooks{before: store.before, failed: store.failed, perform: store.perform})
}

func applyReviewAndReport(ctx *Context, plan Plan, path, onError, command string) error {
	if onError != "fail" {
		return &CodeError{Code: exitUsage, Err: errors.New("review plans require --on-error=fail")}
	}
	if ctx.OperationContext == nil {
		operation, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		ctx.OperationContext = operation
		defer func() { ctx.OperationContext = nil }()
	}
	results, applyErr := applyReviewPlan(ctx, plan)
	if applyErr == nil {
		_, _, replayed := summarizeApplyResults(results)
		if replayed != len(results) {
			plan.AppliedAt = ctx.Now().UTC().Format(time.RFC3339)
			if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
				applyErr = err
			}
		}
	}
	emitAgentApplySummary(ctx, command, results, false, applyErr)
	event := strings.ReplaceAll(command, " ", "_")
	if applyErr != nil {
		emitProgress(ctx, event+"_error", map[string]any{"error": applyErr.Error()})
	} else {
		emitProgress(ctx, event+"_complete", map[string]any{"action_count": len(plan.Actions)})
	}
	if err := writeReviewReport(ctx, plan, results, "applied", path, applyErr); err != nil {
		return err
	}
	return applyErr
}

func (s *reviewReplayStore) perform(action Action) error {
	s.response = nil
	for i := s.index + 1; i < len(s.plan.Actions); i++ {
		if s.plan.Actions[i].TaskID == action.TaskID {
			return applyActionResponse(s.ctx, action, &s.response)
		}
	}
	return applyAction(s.ctx, action)
}
