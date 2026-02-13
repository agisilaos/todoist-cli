package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func resolveAssigneeID(ctx *Context, assigneeRef, projectRef, taskID string) (string, error) {
	ref := strings.TrimSpace(assigneeRef)
	if ref == "" {
		return "", nil
	}
	if strings.EqualFold(ref, "me") {
		reqCtx, cancel := requestContext(ctx)
		id, reqID, err := ctx.Client.SyncCurrentUserID(reqCtx)
		cancel()
		if err != nil {
			return "", err
		}
		setRequestID(ctx, reqID)
		return id, nil
	}
	stripped := stripIDPrefix(ref)
	if stripped != ref || isNumeric(stripped) {
		return stripped, nil
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
	for _, c := range collaborators {
		if strings.EqualFold(c.ID, ref) || strings.EqualFold(c.Name, ref) || strings.EqualFold(c.Email, ref) {
			return c.ID, nil
		}
	}
	var candidates []fuzzyCandidate
	lower := strings.ToLower(ref)
	for _, c := range collaborators {
		if strings.Contains(strings.ToLower(c.Name), lower) || strings.Contains(strings.ToLower(c.Email), lower) {
			candidates = append(candidates, fuzzyCandidate{
				ID:   c.ID,
				Name: fmt.Sprintf("%s <%s>", c.Name, c.Email),
			})
		}
	}
	if len(candidates) == 1 {
		return candidates[0].ID, nil
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
