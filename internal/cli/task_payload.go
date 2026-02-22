package cli

import (
	"strings"

	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

type taskMutationInput = apptasks.MutationInput

type cliTaskSelectorResolver struct {
	ctx *Context
}

func buildTaskCreatePayload(ctx *Context, in taskMutationInput) (map[string]any, error) {
	return apptasks.BuildCreatePayload(in, cliTaskSelectorResolver{ctx: ctx})
}

func buildTaskUpdatePayload(ctx *Context, in taskMutationInput) (map[string]any, error) {
	return apptasks.BuildUpdatePayload(in, cliTaskSelectorResolver{ctx: ctx})
}

func buildTaskMovePayload(ctx *Context, projectID, projectRef, sectionID, sectionRef, parent string) (map[string]any, error) {
	return apptasks.BuildMovePayload(projectID, projectRef, sectionID, sectionRef, parent, cliTaskSelectorResolver{ctx: ctx})
}

func resolveProjectSelector(ctx *Context, explicitID, reference string) (string, error) {
	return cliTaskSelectorResolver{ctx: ctx}.ResolveProjectSelector(explicitID, reference)
}

func resolveSectionSelector(ctx *Context, explicitID, reference, projectRef string) (string, error) {
	return cliTaskSelectorResolver{ctx: ctx}.ResolveSectionSelector(explicitID, reference, projectRef)
}

func resolveAssigneeSelector(ctx *Context, explicitID, reference, projectRef, taskID string) (string, error) {
	return cliTaskSelectorResolver{ctx: ctx}.ResolveAssigneeSelector(explicitID, reference, projectRef, taskID)
}

func (r cliTaskSelectorResolver) ResolveProjectSelector(explicitID, reference string) (string, error) {
	explicitID = strings.TrimSpace(stripIDPrefix(explicitID))
	if explicitID != "" {
		return explicitID, nil
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	return resolveProjectID(r.ctx, reference)
}

func (r cliTaskSelectorResolver) ResolveSectionSelector(explicitID, reference, projectRef string) (string, error) {
	explicitID = strings.TrimSpace(stripIDPrefix(explicitID))
	if explicitID != "" {
		return explicitID, nil
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	return resolveSectionID(r.ctx, reference, projectRef)
}

func (r cliTaskSelectorResolver) ResolveAssigneeSelector(explicitID, reference, projectRef, taskID string) (string, error) {
	explicitID = strings.TrimSpace(stripIDPrefix(explicitID))
	if explicitID != "" {
		return explicitID, nil
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	return resolveAssigneeID(r.ctx, reference, projectRef, taskID)
}
