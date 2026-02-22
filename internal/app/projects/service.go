package projects

import (
	"errors"
	"strings"
)

type AddInput struct {
	Name        string
	Description string
	ParentID    string
	Color       string
	Favorite    bool
	ViewStyle   string
	WorkspaceID string
}

type UpdateInput struct {
	ID          string
	Name        string
	Description string
	Color       string
	Favorite    bool
	ViewStyle   string
}

func BuildAddPayload(in AddInput) (map[string]any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("--name is required")
	}
	body := map[string]any{"name": name}
	if strings.TrimSpace(in.Description) != "" {
		body["description"] = strings.TrimSpace(in.Description)
	}
	if strings.TrimSpace(in.ParentID) != "" {
		body["parent_id"] = strings.TrimSpace(in.ParentID)
	}
	if strings.TrimSpace(in.Color) != "" {
		body["color"] = strings.TrimSpace(in.Color)
	}
	if in.Favorite {
		body["is_favorite"] = true
	}
	if strings.TrimSpace(in.ViewStyle) != "" {
		body["view_style"] = strings.TrimSpace(in.ViewStyle)
	}
	if strings.TrimSpace(in.WorkspaceID) != "" {
		body["workspace_id"] = strings.TrimSpace(in.WorkspaceID)
	}
	return body, nil
}

func BuildUpdatePayload(in UpdateInput) (string, map[string]any, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return "", nil, errors.New("--id is required")
	}
	body := map[string]any{}
	if strings.TrimSpace(in.Name) != "" {
		body["name"] = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.Description) != "" {
		body["description"] = strings.TrimSpace(in.Description)
	}
	if strings.TrimSpace(in.Color) != "" {
		body["color"] = strings.TrimSpace(in.Color)
	}
	if in.Favorite {
		body["is_favorite"] = true
	}
	if strings.TrimSpace(in.ViewStyle) != "" {
		body["view_style"] = strings.TrimSpace(in.ViewStyle)
	}
	if len(body) == 0 {
		return "", nil, errors.New("no fields to update")
	}
	return id, body, nil
}
