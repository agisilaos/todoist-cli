package activities

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

type ListInput struct {
	Since     string
	Until     string
	Type      string
	Event     string
	ProjectID string
	By        string
	Limit     int
	Cursor    string
	All       bool
}

func NormalizeListInput(in ListInput) (ListInput, error) {
	out := in
	out.Since = strings.TrimSpace(out.Since)
	out.Until = strings.TrimSpace(out.Until)
	out.Type = strings.ToLower(strings.TrimSpace(out.Type))
	out.Event = strings.ToLower(strings.TrimSpace(out.Event))
	out.ProjectID = strings.TrimSpace(out.ProjectID)
	out.By = strings.TrimSpace(out.By)
	out.Cursor = strings.TrimSpace(out.Cursor)

	switch out.Type {
	case "", "task", "comment", "project":
	default:
		return ListInput{}, errors.New("--type must be one of: task, comment, project")
	}
	if out.Limit <= 0 {
		out.Limit = 50
	}
	if out.Limit > 100 {
		out.Limit = 100
	}
	return out, nil
}

func BuildQuery(in ListInput) url.Values {
	query := url.Values{}
	if in.Since != "" {
		query.Set("date_from", in.Since)
	}
	if in.Until != "" {
		query.Set("date_to", in.Until)
	}
	if in.Type != "" {
		switch in.Type {
		case "task":
			query.Set("object_type", "item")
		case "comment":
			query.Set("object_type", "note")
		default:
			query.Set("object_type", in.Type)
		}
	}
	if in.Event != "" {
		query.Set("event_type", in.Event)
	}
	if in.ProjectID != "" {
		query.Set("parent_project_id", in.ProjectID)
	}
	if in.By != "" {
		query.Set("initiator_id", in.By)
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}
	query.Set("limit", strconv.Itoa(in.Limit))
	return query
}
