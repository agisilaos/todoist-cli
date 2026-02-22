package cli

import coreagent "github.com/agisilaos/todoist-cli/internal/agent"

type PlannerRequest = coreagent.PlannerRequest

type PlannerContext = coreagent.PlannerContext

type Plan = coreagent.Plan

type PlanSummary = coreagent.PlanSummary

type Action = coreagent.Action

type applyResult struct {
	Action        Action `json:"action"`
	Error         error  `json:"-"`
	SkippedReplay bool   `json:"skipped_replay,omitempty"`
}
