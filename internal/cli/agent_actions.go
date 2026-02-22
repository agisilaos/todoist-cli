package cli

import (
	"errors"
	"fmt"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
)

func applyAction(ctx *Context, action Action) error {
	switch action.Type {
	case "task_add":
		body, err := buildTaskCreatePayload(ctx, taskMutationInput{
			Content:      action.Content,
			Description:  action.Description,
			ProjectRef:   action.Project,
			ProjectID:    action.ProjectID,
			SectionRef:   action.Section,
			SectionID:    action.SectionID,
			ParentID:     action.Parent,
			Labels:       action.Labels,
			Priority:     action.Priority,
			DueString:    action.Due,
			DueDate:      action.DueDate,
			DueDatetime:  action.DueDatetime,
			DueLang:      action.DueLang,
			Duration:     action.Duration,
			DurationUnit: action.DurationUnit,
			Deadline:     action.Deadline,
			AssigneeID:   action.Assignee,
		})
		if err != nil {
			return err
		}
		reqCtx, cancel := requestContext(ctx)
		_, err = ctx.Client.Post(reqCtx, "/tasks", nil, body, nil, true)
		cancel()
		return err
	case "task_update":
		if action.TaskID == "" {
			return errors.New("task_update requires task_id")
		}
		body, err := buildTaskUpdatePayload(ctx, taskMutationInput{
			Description:  action.Description,
			Labels:       action.Labels,
			Priority:     action.Priority,
			DueString:    action.Due,
			DueDate:      action.DueDate,
			DueDatetime:  action.DueDatetime,
			DueLang:      action.DueLang,
			Duration:     action.Duration,
			DurationUnit: action.DurationUnit,
			Deadline:     action.Deadline,
			AssigneeID:   action.Assignee,
			TaskID:       action.TaskID,
		})
		if err != nil {
			return err
		}
		if action.Content != "" {
			body["content"] = action.Content
		}
		reqCtx, cancel := requestContext(ctx)
		_, err = ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID, nil, body, nil, true)
		cancel()
		return err
	case "task_move":
		if action.TaskID == "" {
			return errors.New("task_move requires task_id")
		}
		body, err := buildTaskMovePayload(ctx, action.ProjectID, action.Project, action.SectionID, action.Section, action.Parent)
		if err != nil {
			return err
		}
		reqCtx, cancel := requestContext(ctx)
		_, err = ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID+"/move", nil, body, nil, true)
		cancel()
		return err
	case "task_complete":
		if action.TaskID == "" {
			return errors.New("task_complete requires task_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID+"/close", nil, nil, nil, true)
		cancel()
		return err
	case "task_reopen":
		if action.TaskID == "" {
			return errors.New("task_reopen requires task_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID+"/reopen", nil, nil, nil, true)
		cancel()
		return err
	case "task_delete":
		if action.TaskID == "" {
			return errors.New("task_delete requires task_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/tasks/"+action.TaskID, nil)
		cancel()
		return err
	case "project_add":
		if action.Name == "" {
			return errors.New("project_add requires name")
		}
		body := map[string]any{"name": action.Name}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if action.Parent != "" {
			id, err := resolveProjectID(ctx, action.Parent)
			if err != nil {
				return err
			}
			body["parent_id"] = id
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects", nil, body, nil, true)
		cancel()
		return err
	case "project_update":
		if action.ProjectID == "" {
			return errors.New("project_update requires project_id")
		}
		body := map[string]any{}
		if action.Name != "" {
			body["name"] = action.Name
		}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects/"+action.ProjectID, nil, body, nil, true)
		cancel()
		return err
	case "project_archive":
		if action.ProjectID == "" {
			return errors.New("project_archive requires project_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects/"+action.ProjectID+"/archive", nil, nil, nil, true)
		cancel()
		return err
	case "project_unarchive":
		if action.ProjectID == "" {
			return errors.New("project_unarchive requires project_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects/"+action.ProjectID+"/unarchive", nil, nil, nil, true)
		cancel()
		return err
	case "project_delete":
		if action.ProjectID == "" {
			return errors.New("project_delete requires project_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/projects/"+action.ProjectID, nil)
		cancel()
		return err
	case "section_add":
		if action.Name == "" {
			return errors.New("section_add requires name")
		}
		projectID, err := resolveProjectSelector(ctx, action.ProjectID, action.Project)
		if err != nil {
			return err
		}
		if projectID == "" {
			return errors.New("section_add requires project or project_id")
		}
		body := map[string]any{"name": action.Name, "project_id": projectID}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		reqCtx, cancel := requestContext(ctx)
		_, err = ctx.Client.Post(reqCtx, "/sections", nil, body, nil, true)
		cancel()
		return err
	case "section_update":
		if action.SectionID == "" || action.Name == "" {
			return errors.New("section_update requires section_id and name")
		}
		body := map[string]any{"name": action.Name}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/sections/"+action.SectionID, nil, body, nil, true)
		cancel()
		return err
	case "section_delete":
		if action.SectionID == "" {
			return errors.New("section_delete requires section_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/sections/"+action.SectionID, nil)
		cancel()
		return err
	case "label_add":
		if action.Name == "" {
			return errors.New("label_add requires name")
		}
		body := map[string]any{"name": action.Name}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/labels", nil, body, nil, true)
		cancel()
		return err
	case "label_update":
		if action.LabelID == "" {
			return errors.New("label_update requires label_id")
		}
		body := map[string]any{}
		if action.Name != "" {
			body["name"] = action.Name
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/labels/"+action.LabelID, nil, body, nil, true)
		cancel()
		return err
	case "label_delete":
		if action.LabelID == "" {
			return errors.New("label_delete requires label_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/labels/"+action.LabelID, nil)
		cancel()
		return err
	case "comment_add":
		if action.Content == "" {
			return errors.New("comment_add requires content")
		}
		body := map[string]any{"content": action.Content}
		if action.TaskID != "" {
			body["task_id"] = action.TaskID
		}
		projectID, err := resolveProjectSelector(ctx, action.ProjectID, action.Project)
		if err != nil {
			return err
		}
		if projectID != "" {
			body["project_id"] = projectID
		}
		reqCtx, cancel := requestContext(ctx)
		_, err = ctx.Client.Post(reqCtx, "/comments", nil, body, nil, true)
		cancel()
		return err
	case "comment_update":
		if action.CommentID == "" || action.Content == "" {
			return errors.New("comment_update requires comment_id and content")
		}
		body := map[string]any{"content": action.Content}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/comments/"+action.CommentID, nil, body, nil, true)
		cancel()
		return err
	case "comment_delete":
		if action.CommentID == "" {
			return errors.New("comment_delete requires comment_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/comments/"+action.CommentID, nil)
		cancel()
		return err
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
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
