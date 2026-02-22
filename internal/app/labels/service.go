package labels

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

type ListInput struct {
	Limit  int
	Cursor string
}

type AddInput struct {
	Name     string
	Color    string
	Order    int
	Favorite bool
}

type UpdateInput struct {
	ID         string
	Name       string
	Color      string
	Order      int
	Favorite   bool
	Unfavorite bool
}

func BuildListQuery(in ListInput) url.Values {
	query := url.Values{}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	query.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(in.Cursor) != "" {
		query.Set("cursor", strings.TrimSpace(in.Cursor))
	}
	return query
}

func BuildAddPayload(in AddInput) (map[string]any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("--name is required")
	}
	body := map[string]any{"name": name}
	if strings.TrimSpace(in.Color) != "" {
		body["color"] = strings.TrimSpace(in.Color)
	}
	if in.Order > 0 {
		body["order"] = in.Order
	}
	if in.Favorite {
		body["is_favorite"] = true
	}
	return body, nil
}

func BuildUpdatePayload(in UpdateInput) (string, map[string]any, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return "", nil, errors.New("--id is required")
	}
	if in.Favorite && in.Unfavorite {
		return "", nil, errors.New("--favorite and --unfavorite are mutually exclusive")
	}
	body := map[string]any{}
	if strings.TrimSpace(in.Name) != "" {
		body["name"] = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.Color) != "" {
		body["color"] = strings.TrimSpace(in.Color)
	}
	if in.Order > 0 {
		body["order"] = in.Order
	}
	if in.Favorite {
		body["is_favorite"] = true
	}
	if in.Unfavorite {
		body["is_favorite"] = false
	}
	if len(body) == 0 {
		return "", nil, errors.New("no fields to update")
	}
	return id, body, nil
}
