package comments

import (
	"errors"
	"strings"
)

type ListInput struct {
	TaskID    string
	ProjectID string
}

type AddInput struct {
	Content   string
	TaskID    string
	ProjectID string
}

type UpdateInput struct {
	ID      string
	Content string
}

func ValidateList(in ListInput) error {
	if strings.TrimSpace(in.TaskID) == "" && strings.TrimSpace(in.ProjectID) == "" {
		return errors.New("--task or --project is required")
	}
	return nil
}

func BuildAddPayload(in AddInput) (map[string]any, error) {
	content := strings.TrimSpace(in.Content)
	taskID := strings.TrimSpace(in.TaskID)
	projectID := strings.TrimSpace(in.ProjectID)
	if content == "" {
		return nil, errors.New("--content is required")
	}
	if taskID == "" && projectID == "" {
		return nil, errors.New("--task or --project is required")
	}
	body := map[string]any{"content": content}
	if taskID != "" {
		body["task_id"] = taskID
	}
	if projectID != "" {
		body["project_id"] = projectID
	}
	return body, nil
}

func BuildUpdatePayload(in UpdateInput) (string, map[string]any, error) {
	id := strings.TrimSpace(in.ID)
	content := strings.TrimSpace(in.Content)
	if id == "" || content == "" {
		return "", nil, errors.New("--id and --content are required")
	}
	return id, map[string]any{"content": content}, nil
}
