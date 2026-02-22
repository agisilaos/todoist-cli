package sections

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

type ListInput struct {
	Limit     int
	Cursor    string
	ProjectID string
}

type AddInput struct {
	Name      string
	ProjectID string
}

type UpdateInput struct {
	ID   string
	Name string
}

func BuildListQuery(in ListInput) url.Values {
	query := url.Values{}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	query.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(in.ProjectID) != "" {
		query.Set("project_id", strings.TrimSpace(in.ProjectID))
	}
	if strings.TrimSpace(in.Cursor) != "" {
		query.Set("cursor", strings.TrimSpace(in.Cursor))
	}
	return query
}

func BuildAddPayload(in AddInput) (map[string]any, error) {
	name := strings.TrimSpace(in.Name)
	projectID := strings.TrimSpace(in.ProjectID)
	if name == "" || projectID == "" {
		return nil, errors.New("--name and --project are required")
	}
	return map[string]any{"name": name, "project_id": projectID}, nil
}

func BuildUpdatePayload(in UpdateInput) (string, map[string]any, error) {
	id := strings.TrimSpace(in.ID)
	name := strings.TrimSpace(in.Name)
	if id == "" || name == "" {
		return "", nil, errors.New("--id and --name are required")
	}
	return id, map[string]any{"name": name}, nil
}
