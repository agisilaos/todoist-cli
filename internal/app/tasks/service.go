package tasks

import (
	"context"
	"errors"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
)

type TaskResolver interface {
	ResolveTaskRef(ctx context.Context, ref string) (api.Task, error)
}

type Service struct {
	Resolver TaskResolver
}

type ActionSelection struct {
	ID, Ref, Filter string
	Confirmed       bool
}

type MoveDestination struct {
	Project, Section, Parent  string
	ClearParent, ClearSection bool
}

func ValidateActionSelection(in ActionSelection) error {
	if in.ID != "" && in.Ref != "" || in.Filter != "" && (in.ID != "" || in.Ref != "") {
		return errors.New("task selectors cannot be combined")
	}
	if in.ID == "" && in.Ref == "" && in.Filter == "" {
		return errors.New("task target required")
	}
	if in.Filter != "" && !in.Confirmed {
		return errors.New("filtered batch requires --yes (or --force)")
	}
	return nil
}

func ValidateMoveDestination(in MoveDestination) error {
	clear := in.ClearParent || in.ClearSection
	destination := in.Project != "" || in.Section != "" || in.Parent != ""
	if !clear && !destination {
		return errors.New("move destination or clear flag required")
	}
	if clear && destination {
		return errors.New("hierarchy clear flags cannot accompany destination setters")
	}
	if in.Parent != "" && (in.Project != "" || in.Section != "") {
		return errors.New("parent cannot accompany project or section destination")
	}
	return nil
}

type ResolveTaskTargetInput struct {
	ID, Ref string
}

func (s Service) ResolveTaskTarget(ctx context.Context, in ResolveTaskTargetInput) (string, error) {
	id, err := normalizeTaskID(in.ID)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(in.Ref)
	if id == "" && ref != "" {
		if s.Resolver == nil {
			return "", errors.New("task resolver is not configured")
		}
		task, err := s.Resolver.ResolveTaskRef(ctx, ref)
		if err != nil {
			return "", err
		}
		id = task.ID
	}
	if id == "" {
		return "", errors.New("task id is required")
	}
	return id, nil
}

func normalizeTaskID(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	normalized, directID, err := apprefs.NormalizeEntityRef(value, "task")
	if err != nil {
		return "", err
	}
	if !directID {
		return trimmed, nil
	}
	return strings.TrimSpace(normalized), nil
}
