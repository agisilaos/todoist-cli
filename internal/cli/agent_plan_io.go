package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"

	"io"
)

func writePlanOutput(ctx *Context, plan Plan) error {
	return writePlanPreview(ctx, plan, false)
}

func writePlanFile(path string, plan Plan) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readPlanFile(path string, stdin io.Reader) (Plan, error) {
	var data []byte
	var err error
	source := path
	if path == "-" {
		if stdin == nil {
			return Plan{}, &CodeError{Code: exitUsage, Err: errors.New("stdin not available for --plan -")}
		}
		data, err = io.ReadAll(stdin)
		source = "stdin"
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		if path != "-" && errors.Is(err, os.ErrNotExist) {
			return Plan{}, &CodeError{Code: exitUsage, Err: fmt.Errorf("plan file not found: %s", path)}
		}
		return Plan{}, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, &CodeError{Code: exitUsage, Err: fmt.Errorf("invalid plan JSON in %s: %w", source, err)}
	}
	return plan, nil
}

func isPlanFileNotFoundError(err error) bool {
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var codeErr *CodeError
	if errors.As(err, &codeErr) {
		return strings.Contains(codeErr.Error(), "plan file not found:")
	}
	return false
}

func writePlanPreview(ctx *Context, plan Plan, dryRun bool) error {
	if ctx.Mode == output.ModeJSON {
		payload := map[string]any{
			"plan":         plan,
			"dry_run":      dryRun,
			"action_count": len(plan.Actions),
			"summary":      plan.Summary,
		}
		return output.WriteJSON(ctx.Stdout, payload, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Plan: %s\n", plan.Instruction)
	if dryRun {
		fmt.Fprintln(ctx.Stdout, "DRY RUN: no actions applied")
	}
	fmt.Fprintf(ctx.Stdout, "Confirm: %s\n", plan.ConfirmToken)
	fmt.Fprintf(ctx.Stdout, "Actions: %d (tasks=%d projects=%d sections=%d labels=%d comments=%d)\n",
		len(plan.Actions), plan.Summary.Tasks, plan.Summary.Projects, plan.Summary.Sections, plan.Summary.Labels, plan.Summary.Comments)
	for i, action := range plan.Actions {
		if strings.TrimSpace(action.Reason) != "" {
			fmt.Fprintf(ctx.Stdout, "%d. %s (%s)\n", i+1, action.Type, strings.TrimSpace(action.Reason))
			continue
		}
		fmt.Fprintf(ctx.Stdout, "%d. %s\n", i+1, action.Type)
	}
	return nil
}

func normalizeAndValidatePlan(plan *Plan, instruction string, now func() time.Time, expectedVersion int) error {
	if plan.Version == 0 {
		plan.Version = 1
	}
	if expectedVersion > 0 && plan.Version != expectedVersion {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported plan version %d (expected %d)", plan.Version, expectedVersion)}
	}
	if plan.Instruction == "" {
		plan.Instruction = instruction
	}
	if plan.CreatedAt == "" {
		plan.CreatedAt = now().UTC().Format(time.RFC3339)
	}
	if plan.Summary == (PlanSummary{}) {
		plan.Summary = summarizeActions(plan.Actions)
	}
	return validatePlan(*plan, expectedVersion, true)
}

func lastPlanPath(ctx *Context) string {
	if ctx.ConfigPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(ctx.ConfigPath), "last_plan.json")
}

func newConfirmToken() string {
	id := api.NewRequestID()
	if len(id) >= 4 {
		return id[:4]
	}
	return "confirm"
}

func toAnySlice[T any](items []T) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func writePlanApplyResult(ctx *Context, plan Plan, results []applyResult, applyErr error) error {
	if ctx.Mode == output.ModeJSON {
		type resultJSON struct {
			Action Action `json:"action"`
			Error  string `json:"error,omitempty"`
		}
		out := struct {
			Plan    Plan         `json:"plan"`
			Results []resultJSON `json:"results"`
		}{
			Plan: plan,
		}
		for _, r := range results {
			entry := resultJSON{Action: r.Action}
			if r.SkippedReplay {
				entry.Error = "skipped_replay"
				out.Results = append(out.Results, entry)
				continue
			}
			if r.Error != nil {
				entry.Error = r.Error.Error()
			}
			out.Results = append(out.Results, entry)
		}
		return output.WriteJSON(ctx.Stdout, out, output.Meta{RequestID: ctxRequestIDValue(ctx)})
	}
	fmt.Fprintf(ctx.Stdout, "Applied plan: %s\n", plan.Instruction)
	okCount, failedCount, skippedReplay := summarizeApplyResults(results)
	destructive := 0
	byType := map[string]int{}
	for _, result := range results {
		byType[result.Action.Type]++
		if isDestructiveActionType(result.Action.Type) {
			destructive++
		}
	}
	fmt.Fprintf(ctx.Stdout, "Summary: actions=%d ok=%d failed=%d skipped_replay=%d\n", len(results), okCount, failedCount, skippedReplay)
	fmt.Fprintf(ctx.Stdout, "Risk: destructive_actions=%d\n", destructive)
	if len(byType) > 0 {
		keys := make([]string, 0, len(byType))
		for key := range byType {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fmt.Fprintln(ctx.Stdout, "By action type:")
		for _, key := range keys {
			fmt.Fprintf(ctx.Stdout, "  - %s: %d\n", key, byType[key])
		}
	}
	fmt.Fprintln(ctx.Stdout, "Results:")
	for i, r := range results {
		status := "ok"
		if r.SkippedReplay {
			status = "skipped (replay)"
		}
		if r.Error != nil {
			status = "error: " + r.Error.Error()
		}
		fmt.Fprintf(ctx.Stdout, "%d. %s [%s]\n", i+1, r.Action.Type, status)
	}
	if applyErr != nil {
		fmt.Fprintf(ctx.Stdout, "Outcome: completed with error: %v\n", applyErr)
	} else {
		fmt.Fprintln(ctx.Stdout, "Outcome: success")
	}
	return applyErr
}
