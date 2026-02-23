package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type MoveProjectToWorkspaceInput struct {
	ProjectID   string
	WorkspaceID string
	Visibility  string
}

func (c *Client) MoveProjectToWorkspace(ctx context.Context, in MoveProjectToWorkspaceInput) (Project, string, error) {
	projectID := strings.TrimSpace(in.ProjectID)
	workspaceID := strings.TrimSpace(in.WorkspaceID)
	if projectID == "" {
		return Project{}, "", fmt.Errorf("project_id is required")
	}
	if workspaceID == "" {
		return Project{}, "", fmt.Errorf("workspace_id is required")
	}
	body := map[string]any{
		"project_id":   projectID,
		"workspace_id": workspaceID,
	}
	if visibility := strings.TrimSpace(in.Visibility); visibility != "" {
		body["access"] = map[string]any{"visibility": visibility}
	}
	var raw map[string]any
	reqID, err := c.Post(ctx, "/projects/move_to_workspace", nil, body, &raw, true)
	if err != nil {
		return Project{}, reqID, err
	}
	project, err := decodeProjectFromMoveResponse(raw)
	if err != nil {
		return Project{}, reqID, err
	}
	return project, reqID, nil
}

func (c *Client) MoveProjectToPersonal(ctx context.Context, projectID string) (Project, string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return Project{}, "", fmt.Errorf("project_id is required")
	}
	var raw map[string]any
	reqID, err := c.Post(ctx, "/projects/move_to_personal", nil, map[string]any{"project_id": projectID}, &raw, true)
	if err != nil {
		return Project{}, reqID, err
	}
	project, err := decodeProjectFromMoveResponse(raw)
	if err != nil {
		return Project{}, reqID, err
	}
	return project, reqID, nil
}

func decodeProjectFromMoveResponse(raw map[string]any) (Project, error) {
	if len(raw) == 0 {
		return Project{}, fmt.Errorf("project move response is empty")
	}
	payload := any(raw)
	if wrapped, ok := raw["project"]; ok {
		payload = wrapped
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Project{}, err
	}
	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(project.ID) == "" {
		return Project{}, fmt.Errorf("project move response missing project id")
	}
	return project, nil
}
