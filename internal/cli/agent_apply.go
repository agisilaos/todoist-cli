package cli

import "time"

func applyActionsWithMode(ctx *Context, confirmToken string, actions []Action, onError string) ([]applyResult, error) {
	if onError == "" {
		onError = "fail"
	}
	journal, journalPath, err := loadReplayJournal(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]applyResult, 0, len(actions))
	for idx, action := range actions {
		emitProgress(ctx, "agent_action_start", map[string]any{"index": idx, "action_type": action.Type})
		replayKey := makeReplayKey(confirmToken, idx, action)
		if _, ok := journal.Applied[replayKey]; ok {
			results = append(results, applyResult{Action: action, SkippedReplay: true})
			emitProgress(ctx, "agent_action_skipped_replay", map[string]any{"index": idx, "action_type": action.Type})
			continue
		}
		err := applyAction(ctx, action)
		results = append(results, applyResult{Action: action, Error: err})
		if err != nil {
			emitProgress(ctx, "agent_action_error", map[string]any{"index": idx, "action_type": action.Type, "error": err.Error()})
		} else {
			nowFn := time.Now
			if ctx != nil && ctx.Now != nil {
				nowFn = ctx.Now
			}
			markReplayApplied(&journal, replayKey, nowFn())
			emitProgress(ctx, "agent_action_complete", map[string]any{"index": idx, "action_type": action.Type})
		}
		if err != nil && onError == "fail" {
			_ = saveReplayJournal(journalPath, journal)
			return results, err
		}
	}
	if err := saveReplayJournal(journalPath, journal); err != nil {
		return results, err
	}
	return results, nil
}
