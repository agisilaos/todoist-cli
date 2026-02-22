package cli

import (
	"fmt"
	"net/http"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
	appagent "github.com/agisilaos/todoist-cli/internal/app/agent"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

func applyAction(ctx *Context, action Action) error {
	req, err := appagent.BuildActionRequest(action, appagent.ActionDeps{
		BuildTaskCreatePayload: func(in apptasks.MutationInput) (map[string]any, error) {
			return buildTaskCreatePayload(ctx, in)
		},
		BuildTaskUpdatePayload: func(in apptasks.MutationInput) (map[string]any, error) {
			return buildTaskUpdatePayload(ctx, in)
		},
		BuildTaskMovePayload: func(projectID, projectRef, sectionID, sectionRef, parent string) (map[string]any, error) {
			return buildTaskMovePayload(ctx, projectID, projectRef, sectionID, sectionRef, parent)
		},
		ResolveProjectID: func(reference string) (string, error) {
			return resolveProjectID(ctx, reference)
		},
		ResolveProjectSelector: func(explicitID, reference string) (string, error) {
			return resolveProjectSelector(ctx, explicitID, reference)
		},
	})
	if err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	switch req.Method {
	case http.MethodPost:
		_, err = ctx.Client.Post(reqCtx, req.Path, nil, req.Body, nil, true)
		return err
	case http.MethodDelete:
		_, err = ctx.Client.Delete(reqCtx, req.Path, nil)
		return err
	default:
		return &CodeError{Code: exitError, Err: fmt.Errorf("unsupported method: %s", req.Method)}
	}
}

func summarizeActions(actions []Action) PlanSummary {
	return coreagent.SummarizeActions(actions)
}

func validatePlan(plan Plan, expectedVersion int, allowEmptyActions bool) error {
	if err := coreagent.ValidatePlan(plan, expectedVersion, allowEmptyActions); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	return nil
}

func validateActionFields(a Action) error {
	if err := coreagent.ValidateActionFields(a); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	return nil
}
