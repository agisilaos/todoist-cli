package refs

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeEntityRef resolves id:/numeric refs and Todoist web URLs into IDs.
// It returns directID=true when the returned value can be used as an ID directly.
func NormalizeEntityRef(value, entity string) (normalized string, directID bool, err error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false, nil
	}
	if parsed, ok := ParseTodoistEntityURL(trimmed); ok {
		want := strings.ToLower(strings.TrimSpace(entity))
		if parsed.Entity != want {
			return "", false, fmt.Errorf("expected %s URL, got %s URL", want, parsed.Entity)
		}
		return parsed.ID, true, nil
	}
	normalized, directID = NormalizeRef(trimmed)
	return normalized, directID, nil
}

type ParsedTodoistEntityURL struct {
	Entity string
	ID     string
}

func ParseTodoistEntityURL(raw string) (ParsedTodoistEntityURL, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return ParsedTodoistEntityURL{}, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ParsedTodoistEntityURL{}, false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "app.todoist.com" {
		return ParsedTodoistEntityURL{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "app" {
		return ParsedTodoistEntityURL{}, false
	}
	entity := strings.ToLower(strings.TrimSpace(parts[1]))
	switch entity {
	case "task", "project", "label", "filter":
	default:
		return ParsedTodoistEntityURL{}, false
	}
	id := strings.TrimSpace(parts[2])
	if id == "" {
		return ParsedTodoistEntityURL{}, false
	}
	if dash := strings.LastIndex(id, "-"); dash >= 0 && dash+1 < len(id) {
		id = id[dash+1:]
	}
	if id == "" {
		return ParsedTodoistEntityURL{}, false
	}
	return ParsedTodoistEntityURL{Entity: entity, ID: id}, true
}
