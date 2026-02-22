package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	appassignees "github.com/agisilaos/todoist-cli/internal/app/assignees"
)

func resolveAssigneeID(ctx *Context, assigneeRef, projectRef, taskID string) (string, error) {
	ref := strings.TrimSpace(assigneeRef)
	parsedRef := appassignees.ParseRef(ref)
	if ref == "" {
		return "", nil
	}
	if parsedRef.IsMe {
		reqCtx, cancel := requestContext(ctx)
		id, reqID, err := ctx.Client.SyncCurrentUserID(reqCtx)
		cancel()
		if err != nil {
			return "", err
		}
		setRequestID(ctx, reqID)
		return id, nil
	}
	if parsedRef.ID != "" {
		return parsedRef.ID, nil
	}

	projectID := ""
	if strings.TrimSpace(projectRef) != "" {
		id, err := resolveProjectID(ctx, projectRef)
		if err != nil {
			return "", err
		}
		projectID = id
	} else if strings.TrimSpace(taskID) != "" {
		reqCtx, cancel := requestContext(ctx)
		var task api.Task
		reqID, err := ctx.Client.Get(reqCtx, "/tasks/"+taskID, nil, &task)
		cancel()
		if err != nil {
			return "", err
		}
		setRequestID(ctx, reqID)
		projectID = task.ProjectID
	}
	if projectID == "" {
		return "", &CodeError{Code: exitUsage, Err: errors.New("--project is required when assignee is not an ID or \"me\"")}
	}

	collaborators, err := listProjectCollaborators(ctx, projectID)
	if err != nil {
		return "", err
	}
	appCollaborators := make([]appassignees.Collaborator, 0, len(collaborators))
	for _, c := range collaborators {
		appCollaborators = append(appCollaborators, appassignees.Collaborator{
			ID:    c.ID,
			Name:  c.Name,
			Email: c.Email,
		})
	}
	id, candidates, found := appassignees.MatchCollaboratorID(ref, appCollaborators)
	if found {
		return id, nil
	}
	if len(candidates) > 1 {
		if chosen, ok, err := promptAmbiguousChoice(ctx, "assignee", ref, candidates); err != nil {
			return "", err
		} else if ok {
			return chosen, nil
		}
		return "", ambiguousMatchCodeError("assignee", ref, candidates)
	}
	return "", &CodeError{Code: exitNotFound, Err: fmt.Errorf("assignee %q not found", ref)}
}

func listProjectCollaborators(ctx *Context, projectID string) ([]api.Collaborator, error) {
	if cache := ctx.cache(); cache != nil {
		if collaborators, ok := cache.collaboratorsByProject[projectID]; ok {
			return cloneSlice(collaborators), nil
		}
	}
	query := url.Values{}
	query.Set("limit", "200")
	all, _, err := fetchPaginated[api.Collaborator](ctx, "/projects/"+projectID+"/collaborators", query, true)
	if err != nil {
		return nil, err
	}
	if cache := ctx.cache(); cache != nil {
		cache.collaboratorsByProject[projectID] = cloneSlice(all)
	}
	return all, nil
}
