package agent

import (
	"errors"
	"fmt"
	"net/http"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
	appsections "github.com/agisilaos/todoist-cli/internal/app/sections"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

type ActionRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

type ActionDeps struct {
	BuildTaskCreatePayload func(in apptasks.MutationInput) (map[string]any, error)
	BuildTaskUpdatePayload func(in apptasks.MutationInput) (map[string]any, error)
	BuildTaskMovePayload   func(projectID, projectRef, sectionID, sectionRef, parent string) (map[string]any, error)
	ResolveProjectID       func(reference string) (string, error)
	ResolveProjectSelector func(explicitID, reference string) (string, error)
}

func BuildActionRequest(action coreagent.Action, deps ActionDeps) (ActionRequest, error) {
	switch action.Type {
	case "task_add":
		if deps.BuildTaskCreatePayload == nil {
			return ActionRequest{}, errors.New("task create payload builder is not configured")
		}
		body, err := deps.BuildTaskCreatePayload(apptasks.MutationInput{
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
			return ActionRequest{}, err
		}
		return ActionRequest{Method: http.MethodPost, Path: "/tasks", Body: body}, nil
	case "task_update":
		if action.TaskID == "" {
			return ActionRequest{}, errors.New("task_update requires task_id")
		}
		if deps.BuildTaskUpdatePayload == nil {
			return ActionRequest{}, errors.New("task update payload builder is not configured")
		}
		body, err := deps.BuildTaskUpdatePayload(apptasks.MutationInput{
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
			return ActionRequest{}, err
		}
		if action.Content != "" {
			body["content"] = action.Content
		}
		return ActionRequest{Method: http.MethodPost, Path: "/tasks/" + action.TaskID, Body: body}, nil
	case "task_move":
		if action.TaskID == "" {
			return ActionRequest{}, errors.New("task_move requires task_id")
		}
		if deps.BuildTaskMovePayload == nil {
			return ActionRequest{}, errors.New("task move payload builder is not configured")
		}
		body, err := deps.BuildTaskMovePayload(action.ProjectID, action.Project, action.SectionID, action.Section, action.Parent)
		if err != nil {
			return ActionRequest{}, err
		}
		return ActionRequest{Method: http.MethodPost, Path: "/tasks/" + action.TaskID + "/move", Body: body}, nil
	case "task_complete":
		if action.TaskID == "" {
			return ActionRequest{}, errors.New("task_complete requires task_id")
		}
		return ActionRequest{Method: http.MethodPost, Path: "/tasks/" + action.TaskID + "/close"}, nil
	case "task_reopen":
		if action.TaskID == "" {
			return ActionRequest{}, errors.New("task_reopen requires task_id")
		}
		return ActionRequest{Method: http.MethodPost, Path: "/tasks/" + action.TaskID + "/reopen"}, nil
	case "task_delete":
		if action.TaskID == "" {
			return ActionRequest{}, errors.New("task_delete requires task_id")
		}
		return ActionRequest{Method: http.MethodDelete, Path: "/tasks/" + action.TaskID}, nil
	case "project_add":
		if action.Name == "" {
			return ActionRequest{}, errors.New("project_add requires name")
		}
		body := map[string]any{"name": action.Name}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if action.Parent != "" {
			if deps.ResolveProjectID == nil {
				return ActionRequest{}, errors.New("project resolver is not configured")
			}
			id, err := deps.ResolveProjectID(action.Parent)
			if err != nil {
				return ActionRequest{}, err
			}
			body["parent_id"] = id
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		return ActionRequest{Method: http.MethodPost, Path: "/projects", Body: body}, nil
	case "project_update":
		if action.ProjectID == "" {
			return ActionRequest{}, errors.New("project_update requires project_id")
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
		return ActionRequest{Method: http.MethodPost, Path: "/projects/" + action.ProjectID, Body: body}, nil
	case "project_archive":
		if action.ProjectID == "" {
			return ActionRequest{}, errors.New("project_archive requires project_id")
		}
		return ActionRequest{Method: http.MethodPost, Path: "/projects/" + action.ProjectID + "/archive"}, nil
	case "project_unarchive":
		if action.ProjectID == "" {
			return ActionRequest{}, errors.New("project_unarchive requires project_id")
		}
		return ActionRequest{Method: http.MethodPost, Path: "/projects/" + action.ProjectID + "/unarchive"}, nil
	case "project_delete":
		if action.ProjectID == "" {
			return ActionRequest{}, errors.New("project_delete requires project_id")
		}
		return ActionRequest{Method: http.MethodDelete, Path: "/projects/" + action.ProjectID}, nil
	case "section_add":
		if action.Name == "" {
			return ActionRequest{}, errors.New("section_add requires name")
		}
		if deps.ResolveProjectSelector == nil {
			return ActionRequest{}, errors.New("project selector resolver is not configured")
		}
		projectID, err := deps.ResolveProjectSelector(action.ProjectID, action.Project)
		if err != nil {
			return ActionRequest{}, err
		}
		if projectID == "" {
			return ActionRequest{}, errors.New("section_add requires project or project_id")
		}
		body, err := appsections.BuildAddPayload(appsections.AddInput{Name: action.Name, ProjectID: projectID})
		if err != nil {
			return ActionRequest{}, err
		}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		return ActionRequest{Method: http.MethodPost, Path: "/sections", Body: body}, nil
	case "section_update":
		if action.SectionID == "" || action.Name == "" {
			return ActionRequest{}, errors.New("section_update requires section_id and name")
		}
		sectionID, body, err := appsections.BuildUpdatePayload(appsections.UpdateInput{ID: action.SectionID, Name: action.Name})
		if err != nil {
			return ActionRequest{}, err
		}
		return ActionRequest{Method: http.MethodPost, Path: "/sections/" + sectionID, Body: body}, nil
	case "section_delete":
		if action.SectionID == "" {
			return ActionRequest{}, errors.New("section_delete requires section_id")
		}
		return ActionRequest{Method: http.MethodDelete, Path: "/sections/" + action.SectionID}, nil
	case "label_add":
		if action.Name == "" {
			return ActionRequest{}, errors.New("label_add requires name")
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
		return ActionRequest{Method: http.MethodPost, Path: "/labels", Body: body}, nil
	case "label_update":
		if action.LabelID == "" {
			return ActionRequest{}, errors.New("label_update requires label_id")
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
		return ActionRequest{Method: http.MethodPost, Path: "/labels/" + action.LabelID, Body: body}, nil
	case "label_delete":
		if action.LabelID == "" {
			return ActionRequest{}, errors.New("label_delete requires label_id")
		}
		return ActionRequest{Method: http.MethodDelete, Path: "/labels/" + action.LabelID}, nil
	case "comment_add":
		if action.Content == "" {
			return ActionRequest{}, errors.New("comment_add requires content")
		}
		body := map[string]any{"content": action.Content}
		if action.TaskID != "" {
			body["task_id"] = action.TaskID
		}
		if deps.ResolveProjectSelector == nil {
			return ActionRequest{}, errors.New("project selector resolver is not configured")
		}
		projectID, err := deps.ResolveProjectSelector(action.ProjectID, action.Project)
		if err != nil {
			return ActionRequest{}, err
		}
		if projectID != "" {
			body["project_id"] = projectID
		}
		return ActionRequest{Method: http.MethodPost, Path: "/comments", Body: body}, nil
	case "comment_update":
		if action.CommentID == "" || action.Content == "" {
			return ActionRequest{}, errors.New("comment_update requires comment_id and content")
		}
		return ActionRequest{
			Method: http.MethodPost,
			Path:   "/comments/" + action.CommentID,
			Body:   map[string]any{"content": action.Content},
		}, nil
	case "comment_delete":
		if action.CommentID == "" {
			return ActionRequest{}, errors.New("comment_delete requires comment_id")
		}
		return ActionRequest{Method: http.MethodDelete, Path: "/comments/" + action.CommentID}, nil
	default:
		return ActionRequest{}, fmt.Errorf("unsupported action type: %s", action.Type)
	}
}
