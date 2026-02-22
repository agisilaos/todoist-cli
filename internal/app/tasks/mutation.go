package tasks

import "strings"

type MutationInput struct {
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

type SelectorResolver interface {
	ResolveProjectSelector(explicitID, reference string) (string, error)
	ResolveSectionSelector(explicitID, reference, projectRef string) (string, error)
	ResolveAssigneeSelector(explicitID, reference, projectRef, taskID string) (string, error)
}

func BuildCreatePayload(in MutationInput, resolver SelectorResolver) (map[string]any, error) {
	body := map[string]any{"content": in.Content}
	if err := applyMutationPayload(body, in, resolver); err != nil {
		return nil, err
	}
	return body, nil
}

func BuildUpdatePayload(in MutationInput, resolver SelectorResolver) (map[string]any, error) {
	body := map[string]any{}
	if err := applyMutationPayload(body, in, resolver); err != nil {
		return nil, err
	}
	return body, nil
}

func BuildMovePayload(projectID, projectRef, sectionID, sectionRef, parent string, resolver SelectorResolver) (map[string]any, error) {
	body := map[string]any{}
	resolvedProjectID, err := resolver.ResolveProjectSelector(projectID, projectRef)
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
	resolvedSectionID, err := resolver.ResolveSectionSelector(sectionID, sectionRef, sectionProjectRef)
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

func applyMutationPayload(body map[string]any, in MutationInput, resolver SelectorResolver) error {
	if in.Description != "" {
		body["description"] = in.Description
	}
	projectID, err := resolver.ResolveProjectSelector(in.ProjectID, in.ProjectRef)
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
	sectionID, err := resolver.ResolveSectionSelector(in.SectionID, in.SectionRef, sectionProjectRef)
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
	assigneeID, err := resolver.ResolveAssigneeSelector(in.AssigneeID, in.AssigneeRef, in.AssigneeHint, in.TaskID)
	if err != nil {
		return err
	}
	if assigneeID != "" {
		body["assignee_id"] = assigneeID
	}
	return nil
}
