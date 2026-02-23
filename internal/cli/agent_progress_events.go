package cli

func emitAgentPlanLoaded(ctx *Context, command string, actionCount int, source string) {
	emitProgress(ctx, "agent_plan_loaded", map[string]any{
		"command":      command,
		"action_count": actionCount,
		"source":       source,
	})
}

func emitAgentApplySummary(ctx *Context, command string, results []applyResult, dryRun bool, applyErr error) {
	okCount, failedCount, skippedReplay := summarizeApplyResults(results)
	fields := map[string]any{
		"command":        command,
		"dry_run":        dryRun,
		"ok_count":       okCount,
		"failed_count":   failedCount,
		"skipped_replay": skippedReplay,
		"action_count":   len(results),
	}
	if applyErr != nil {
		fields["error"] = applyErr.Error()
	}
	emitProgress(ctx, "agent_apply_summary", fields)
}

func summarizeApplyResults(results []applyResult) (okCount, failedCount, skippedReplay int) {
	for _, result := range results {
		if result.SkippedReplay {
			skippedReplay++
			continue
		}
		if result.Error != nil {
			failedCount++
			continue
		}
		okCount++
	}
	return okCount, failedCount, skippedReplay
}
