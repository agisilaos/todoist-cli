package agent

import (
	"errors"
	"strings"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
)

type PrepareInput struct {
	PlanPath        string
	Instruction     string
	Confirm         string
	ExpectedVersion int
	Force           bool
	DryRun          bool
}

type PrepareDeps struct {
	LoadPlan      func(path string) (coreagent.Plan, error)
	Plan          func(instruction string) (coreagent.Plan, error)
	EnforcePolicy func(plan coreagent.Plan) error
}

// PlanValidationError identifies owned validation failures without reclassifying
// errors from plan loading, external planning, or policy evaluation.
type PlanValidationError struct{ Err error }

func (e *PlanValidationError) Error() string { return e.Err.Error() }
func (e *PlanValidationError) Unwrap() error { return e.Err }

func PreparePlan(in PrepareInput, deps PrepareDeps) (coreagent.Plan, error) {
	var plan coreagent.Plan
	var err error
	if strings.TrimSpace(in.PlanPath) != "" {
		if deps.LoadPlan == nil {
			return coreagent.Plan{}, errors.New("plan loader is not configured")
		}
		plan, err = deps.LoadPlan(in.PlanPath)
	} else {
		if strings.TrimSpace(in.Instruction) == "" {
			return coreagent.Plan{}, errors.New("instruction is required when --plan is not provided")
		}
		if deps.Plan == nil {
			return coreagent.Plan{}, errors.New("planner is not configured")
		}
		plan, err = deps.Plan(in.Instruction)
	}
	if err != nil {
		return coreagent.Plan{}, err
	}
	if err := coreagent.ValidatePlan(plan, in.ExpectedVersion, in.DryRun); err != nil {
		return coreagent.Plan{}, &PlanValidationError{Err: err}
	}
	if deps.EnforcePolicy != nil {
		if err := deps.EnforcePolicy(plan); err != nil {
			return coreagent.Plan{}, err
		}
	}
	if !in.Force {
		if strings.TrimSpace(in.Confirm) == "" {
			return coreagent.Plan{}, errors.New("--confirm is required (or use --force)")
		}
		if plan.ConfirmToken != "" && strings.TrimSpace(in.Confirm) != plan.ConfirmToken {
			return coreagent.Plan{}, errors.New("confirmation token does not match plan")
		}
	}
	return plan, nil
}
