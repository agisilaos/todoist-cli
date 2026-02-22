package agent

import (
	"errors"
	"fmt"
	"strings"
)

func SummarizeActions(actions []Action) PlanSummary {
	var s PlanSummary
	for _, a := range actions {
		switch {
		case strings.HasPrefix(a.Type, "task_"):
			s.Tasks++
		case strings.HasPrefix(a.Type, "project_"):
			s.Projects++
		case strings.HasPrefix(a.Type, "section_"):
			s.Sections++
		case strings.HasPrefix(a.Type, "label_"):
			s.Labels++
		case strings.HasPrefix(a.Type, "comment_"):
			s.Comments++
		}
	}
	return s
}

func ValidatePlan(plan Plan, expectedVersion int, allowEmptyActions bool) error {
	if plan.ConfirmToken == "" {
		return errors.New("plan missing confirm_token (see `todoist schema --name plan --json`)")
	}
	if len(plan.Actions) == 0 && !allowEmptyActions {
		return errors.New("plan has no actions")
	}
	if expectedVersion > 0 && plan.Version != 0 && plan.Version != expectedVersion {
		return fmt.Errorf("unsupported plan version %d (expected %d)", plan.Version, expectedVersion)
	}
	allowed := map[string]struct{}{
		"task_add":          {},
		"task_update":       {},
		"task_move":         {},
		"task_complete":     {},
		"task_reopen":       {},
		"task_delete":       {},
		"project_add":       {},
		"project_update":    {},
		"project_archive":   {},
		"project_unarchive": {},
		"project_delete":    {},
		"section_add":       {},
		"section_update":    {},
		"section_delete":    {},
		"label_add":         {},
		"label_update":      {},
		"label_delete":      {},
		"comment_add":       {},
		"comment_update":    {},
		"comment_delete":    {},
	}
	for _, a := range plan.Actions {
		if _, ok := allowed[a.Type]; !ok {
			return fmt.Errorf("unsupported action type: %s", a.Type)
		}
		if err := ValidateActionFields(a); err != nil {
			return err
		}
	}
	return nil
}

func ValidateActionFields(a Action) error {
	switch a.Type {
	case "task_add":
		if a.Content == "" {
			return errors.New("task_add requires content")
		}
	case "task_update", "task_move", "task_complete", "task_reopen", "task_delete":
		if a.TaskID == "" {
			return fmt.Errorf("%s requires task_id", a.Type)
		}
		if a.Type == "task_move" && a.Project == "" && a.ProjectID == "" && a.Section == "" && a.SectionID == "" && a.Parent == "" {
			return errors.New("task_move requires project/project_id, section/section_id, or parent")
		}
	case "project_add":
		if a.Name == "" {
			return errors.New("project_add requires name")
		}
	case "project_update", "project_archive", "project_unarchive", "project_delete":
		if a.ProjectID == "" {
			return fmt.Errorf("%s requires project_id", a.Type)
		}
	case "section_add":
		if a.Name == "" || (a.Project == "" && a.ProjectID == "") {
			return errors.New("section_add requires name and project/project_id")
		}
	case "section_update", "section_delete":
		if a.SectionID == "" {
			return fmt.Errorf("%s requires section_id", a.Type)
		}
	case "label_add":
		if a.Name == "" {
			return errors.New("label_add requires name")
		}
	case "label_update", "label_delete":
		if a.LabelID == "" {
			return fmt.Errorf("%s requires label_id", a.Type)
		}
	case "comment_add":
		if a.Content == "" {
			return errors.New("comment_add requires content")
		}
		if a.TaskID == "" && a.ProjectID == "" && a.Project == "" {
			return errors.New("comment_add requires task_id or project/project_id")
		}
	case "comment_update", "comment_delete":
		if a.CommentID == "" {
			return fmt.Errorf("%s requires comment_id", a.Type)
		}
	}
	return nil
}
