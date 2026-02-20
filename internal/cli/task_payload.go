package cli

import "strings"

type taskMutationInput struct {
	Content      string
	Description  string
	ProjectRef   string
	ProjectID    string
	SectionRef   string
	SectionID    string
	ParentID     string
	Labels       []string
	Priority     int
	DueString    string
	DueDate      string
	DueDatetime  string
	DueLang      string
	Duration     int
	DurationUnit string
	Deadline     string
	AssigneeRef  string
	AssigneeID   string
	AssigneeHint string
	TaskID       string
}

func buildTaskCreatePayload(ctx *Context, in taskMutationInput) (map[string]any, error) {
	body := map[string]any{"content": in.Content}
	if err := applyTaskMutationPayload(ctx, body, in); err != nil {
		return nil, err
	}
	return body, nil
}

func buildTaskUpdatePayload(ctx *Context, in taskMutationInput) (map[string]any, error) {
	body := map[string]any{}
	if err := applyTaskMutationPayload(ctx, body, in); err != nil {
		return nil, err
	}
	return body, nil
}

func applyTaskMutationPayload(ctx *Context, body map[string]any, in taskMutationInput) error {
	if in.Description != "" {
		body["description"] = in.Description
	}
	projectID, err := resolveProjectSelector(ctx, in.ProjectID, in.ProjectRef)
	if err != nil {
		return err
	}
	if projectID != "" {
		body["project_id"] = projectID
	}

	sectionProjectRef := strings.TrimSpace(in.ProjectRef)
	if sectionProjectRef == "" {
		sectionProjectRef = strings.TrimSpace(projectID)
	}
	sectionID, err := resolveSectionSelector(ctx, in.SectionID, in.SectionRef, sectionProjectRef)
	if err != nil {
		return err
	}
	if sectionID != "" {
		body["section_id"] = sectionID
	}

	if in.ParentID != "" {
		body["parent_id"] = in.ParentID
	}
	if len(in.Labels) > 0 {
		body["labels"] = in.Labels
	}
	if in.Priority > 0 {
		body["priority"] = in.Priority
	}
	if in.DueString != "" {
		body["due_string"] = in.DueString
	}
	if in.DueDate != "" {
		body["due_date"] = in.DueDate
	}
	if in.DueDatetime != "" {
		body["due_datetime"] = in.DueDatetime
	}
	if in.DueLang != "" {
		body["due_lang"] = in.DueLang
	}
	if in.Duration > 0 {
		body["duration"] = in.Duration
	}
	if in.DurationUnit != "" {
		body["duration_unit"] = in.DurationUnit
	}
	if in.Deadline != "" {
		body["deadline_date"] = in.Deadline
	}
	assigneeID, err := resolveAssigneeSelector(ctx, in.AssigneeID, in.AssigneeRef, in.AssigneeHint, in.TaskID)
	if err != nil {
		return err
	}
	if assigneeID != "" {
		body["assignee_id"] = assigneeID
	}
	return nil
}

func buildTaskMovePayload(ctx *Context, projectID, projectRef, sectionID, sectionRef, parent string) (map[string]any, error) {
	body := map[string]any{}
	resolvedProjectID, err := resolveProjectSelector(ctx, projectID, projectRef)
	if err != nil {
		return nil, err
	}
	if resolvedProjectID != "" {
		body["project_id"] = resolvedProjectID
	}
	sectionProjectRef := strings.TrimSpace(projectRef)
	if sectionProjectRef == "" {
		sectionProjectRef = strings.TrimSpace(resolvedProjectID)
	}
	resolvedSectionID, err := resolveSectionSelector(ctx, sectionID, sectionRef, sectionProjectRef)
	if err != nil {
		return nil, err
	}
	if resolvedSectionID != "" {
		body["section_id"] = resolvedSectionID
	}
	if parent != "" {
		body["parent_id"] = parent
	}
	return body, nil
}

func resolveProjectSelector(ctx *Context, explicitID, reference string) (string, error) {
	explicitID = strings.TrimSpace(stripIDPrefix(explicitID))
	if explicitID != "" {
		return explicitID, nil
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	return resolveProjectID(ctx, reference)
}

func resolveSectionSelector(ctx *Context, explicitID, reference, projectRef string) (string, error) {
	explicitID = strings.TrimSpace(stripIDPrefix(explicitID))
	if explicitID != "" {
		return explicitID, nil
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	return resolveSectionID(ctx, reference, projectRef)
}

func resolveAssigneeSelector(ctx *Context, explicitID, reference, projectRef, taskID string) (string, error) {
	explicitID = strings.TrimSpace(stripIDPrefix(explicitID))
	if explicitID != "" {
		return explicitID, nil
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	return resolveAssigneeID(ctx, reference, projectRef, taskID)
}
