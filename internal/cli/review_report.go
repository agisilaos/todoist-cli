package cli

import (
	"fmt"

	"github.com/agisilaos/todoist-cli/internal/output"
)

type reviewActionOutcome struct {
	Index                  int    `json:"index"`
	Type                   string `json:"type"`
	Outcome                string `json:"outcome"`
	Replayed               bool   `json:"replayed,omitempty"`
	Error                  string `json:"error,omitempty"`
	RemoteOutcomeUncertain bool   `json:"remote_outcome_uncertain,omitempty"`
}
type reviewTaskOutcome struct {
	ID          string                `json:"id"`
	Content     string                `json:"content"`
	Disposition string                `json:"disposition"`
	Outcome     string                `json:"outcome"`
	Actions     []reviewActionOutcome `json:"actions"`
}
type reviewReport struct {
	Version  int                 `json:"version"`
	Phase    string              `json:"phase"`
	Plan     Plan                `json:"plan"`
	PlanPath string              `json:"plan_path,omitempty"`
	Tasks    []reviewTaskOutcome `json:"tasks"`
	Counts   map[string]int      `json:"counts"`
	Error    string              `json:"error,omitempty"`
}

func writeReviewReport(ctx *Context, plan Plan, results []applyResult, phase, path string, cause error) error {
	report := reviewReport{Version: 1, Phase: phase, Plan: plan, PlanPath: path, Tasks: []reviewTaskOutcome{}, Counts: map[string]int{}}
	if cause != nil {
		report.Error = cause.Error()
	}
	var replay *reviewReplayStore
	if phase == "applied" {
		if file, err := loadReplayStore(ctx); err == nil {
			replay = &reviewReplayStore{fileReplayStore: file, ctx: ctx, plan: plan}
		}
	}
	for _, task := range plan.Review.Tasks {
		row := reviewTaskOutcome{ID: task.ID, Content: task.Content, Disposition: task.Disposition, Outcome: "unattempted", Actions: []reviewActionOutcome{}}
		if task.Disposition == "keep" {
			row.Outcome = "kept"
		}
		if task.Disposition == "skip" {
			row.Outcome = "skipped"
		}
		ok, failed := 0, 0
		for _, idx := range task.Actions {
			action := plan.Actions[idx]
			outcome := reviewActionOutcome{Index: idx, Type: action.Type, Outcome: "unattempted"}
			if phase == "preview" {
				outcome.Outcome = "proposed"
			}
			if idx < len(results) {
				r := results[idx]
				if r.Error != nil {
					outcome.Outcome = "failed"
					outcome.Error = r.Error.Error()
					failed++
				} else {
					outcome.Outcome = "applied"
					outcome.Replayed = r.SkippedReplay
					ok++
				}
			} else if replay != nil && replay.Contains(makeReplayKey(plan.ConfirmToken, idx, action)) {
				outcome.Outcome = "applied"
				outcome.Replayed = true
				ok++
			}
			if replay != nil && replay.journal.Reviews[replay.taskKey(task.ID)].Pending && replay.journal.Reviews[replay.taskKey(task.ID)].PendingIndex == idx && outcome.Outcome != "applied" {
				outcome.RemoteOutcomeUncertain = true
			}
			row.Actions = append(row.Actions, outcome)
		}
		if len(task.Actions) > 0 {
			switch {
			case phase == "preview":
				row.Outcome = "proposed"
			case ok == len(task.Actions):
				row.Outcome = "applied"
			case ok > 0:
				row.Outcome = "partially_applied"
			case failed > 0:
				row.Outcome = "failed"
			}
		}
		report.Tasks = append(report.Tasks, row)
		report.Counts[row.Outcome]++
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, report)
	}
	heading := phase
	if phase == "applied" {
		heading = "application finished"
		if cause != nil {
			heading = "application stopped"
		}
	}
	fmt.Fprintf(ctx.Stdout, "Review %s: %d selected tasks\n", heading, len(report.Tasks))
	for _, task := range report.Tasks {
		fmt.Fprintf(ctx.Stdout, "%s\t%s\t%s\t%q\n", task.ID, task.Disposition, task.Outcome, task.Content)
		for _, a := range task.Actions {
			fmt.Fprintf(ctx.Stdout, "  %d %s: %s", a.Index+1, a.Type, a.Outcome)
			if a.Replayed {
				fmt.Fprint(ctx.Stdout, " (recorded previously)")
			}
			if a.Error != "" {
				fmt.Fprintf(ctx.Stdout, " — %s", a.Error)
			}
			if a.RemoteOutcomeUncertain {
				fmt.Fprint(ctx.Stdout, " — remote outcome uncertain; inspect before fresh review")
			}
			fmt.Fprintln(ctx.Stdout)
		}
	}
	if path != "" {
		fmt.Fprintf(ctx.Stdout, "Plan: %s\n", path)
	}
	if cause != nil {
		fmt.Fprintf(ctx.Stdout, "Stopped: %v\n", cause)
	}
	writeReviewRecovery(ctx, report)
	return nil
}
