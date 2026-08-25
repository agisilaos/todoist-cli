package cli

import (
	"errors"
	"fmt"
	"time"
)

type applyErrorMode string

const (
	applyErrorModeFail     applyErrorMode = "fail"
	applyErrorModeContinue applyErrorMode = "continue"
)

type replayStoreError struct {
	err error
}

func (e *replayStoreError) Error() string {
	return e.err.Error()
}

func (e *replayStoreError) Unwrap() error {
	return e.err
}

func applyActionsWithMode(ctx *Context, confirmToken string, actions []Action, onError applyErrorMode) ([]applyResult, error) {
	store, err := loadReplayStore(ctx)
	if err != nil {
		return nil, &replayStoreError{err: fmt.Errorf("load replay journal: %w", err)}
	}
	return applyActionsWithReplayStore(ctx, confirmToken, actions, onError, store)
}

func applyActionsWithReplayStore(ctx *Context, confirmToken string, actions []Action, onError applyErrorMode, store replayStore) ([]applyResult, error) {
	results := make([]applyResult, 0, len(actions))
	for idx, action := range actions {
		emitProgress(ctx, "agent_action_start", map[string]any{"index": idx, "action_type": action.Type})
		emitProgress(ctx, "agent_action_validated", map[string]any{"index": idx, "action_type": action.Type})
		replayKey := makeReplayKey(confirmToken, idx, action)
		if store.Contains(replayKey) {
			results = append(results, applyResult{Action: action, SkippedReplay: true})
			emitProgress(ctx, "agent_action_skipped_replay", map[string]any{"index": idx, "action_type": action.Type})
			continue
		}
		emitProgress(ctx, "agent_action_dispatched", map[string]any{"index": idx, "action_type": action.Type})
		if err := applyAction(ctx, action); err != nil {
			results = append(results, applyResult{Action: action, Error: err})
			emitActionFailure(ctx, idx, action, err, nil)
			if onError != applyErrorModeContinue {
				return results, err
			}
			continue
		}
		nowFn := time.Now
		if ctx != nil && ctx.Now != nil {
			nowFn = ctx.Now
		}
		if err := store.RecordApplied(replayKey, nowFn()); err != nil {
			recordErr := &replayStoreError{err: fmt.Errorf("todoist action succeeded but replay recording failed; rerunning may duplicate it: %w", err)}
			results = append(results, applyResult{Action: action, Error: recordErr})
			emitActionFailure(ctx, idx, action, recordErr, map[string]any{
				"stage":            "replay_record",
				"remote_succeeded": true,
			})
			return results, recordErr
		}
		results = append(results, applyResult{Action: action})
		emitProgress(ctx, "agent_action_complete", map[string]any{"index": idx, "action_type": action.Type})
		emitProgress(ctx, "agent_action_succeeded", map[string]any{"index": idx, "action_type": action.Type})
	}
	return results, nil
}

func emitActionFailure(ctx *Context, idx int, action Action, err error, extra map[string]any) {
	fields := map[string]any{
		"index":       idx,
		"action_type": action.Type,
		"error":       err.Error(),
	}
	for key, value := range extra {
		fields[key] = value
	}
	emitProgress(ctx, "agent_action_error", fields)
	emitProgress(ctx, "agent_action_failed", fields)
}

func shouldAbortApply(onError applyErrorMode, err error) bool {
	if err == nil {
		return false
	}
	if onError != applyErrorModeContinue {
		return true
	}
	var replayErr *replayStoreError
	return errors.As(err, &replayErr)
}
