package filters

import (
	"errors"
	"strings"
)

type AddInput struct {
	Name     string
	Query    string
	Color    string
	Favorite bool
}

type UpdateInput struct {
	Ref        string
	Name       string
	Query      string
	Color      string
	Favorite   bool
	Unfavorite bool
}

type DeleteInput struct {
	Ref   string
	Yes   bool
	Force bool
}

func BuildAddPayload(in AddInput) (map[string]any, error) {
	name := strings.TrimSpace(in.Name)
	query := strings.TrimSpace(in.Query)
	if name == "" || query == "" {
		return nil, errors.New("--name and --query are required")
	}
	body := map[string]any{"name": name, "query": query}
	if strings.TrimSpace(in.Color) != "" {
		body["color"] = strings.TrimSpace(in.Color)
	}
	if in.Favorite {
		body["is_favorite"] = true
	}
	return body, nil
}

func BuildUpdatePayload(in UpdateInput) (string, map[string]any, error) {
	ref := strings.TrimSpace(in.Ref)
	if ref == "" {
		return "", nil, errors.New("filter update requires --id or a filter reference")
	}
	if in.Favorite && in.Unfavorite {
		return "", nil, errors.New("--favorite and --unfavorite are mutually exclusive")
	}
	body := map[string]any{}
	if strings.TrimSpace(in.Name) != "" {
		body["name"] = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.Query) != "" {
		body["query"] = strings.TrimSpace(in.Query)
	}
	if strings.TrimSpace(in.Color) != "" {
		body["color"] = strings.TrimSpace(in.Color)
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
	return ref, body, nil
}

func ValidateDelete(in DeleteInput) (string, error) {
	ref := strings.TrimSpace(in.Ref)
	if ref == "" {
		return "", errors.New("filter delete requires --id or a filter reference")
	}
	if !in.Yes && !in.Force {
		return "", errors.New("filter delete requires --yes")
	}
	return ref, nil
}
