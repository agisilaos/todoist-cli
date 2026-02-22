package tasks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
)

type TaskResolver interface {
	ResolveTaskRef(ctx context.Context, ref string) (api.Task, error)
}

type TaskFilterLister interface {
	ListByFilter(ctx context.Context, filter string) ([]api.Task, error)
}

type Service struct {
	Resolver TaskResolver
	Lister   TaskFilterLister
}

type ResolveCompletionInput struct {
	ID     string
	Ref    string
	Filter string
	Yes    bool
	Force  bool
}

type ResolveCompletionResult struct {
	Mode   string
	ID     string
	IDs    []string
	Filter string
}

type ResolveMoveInput struct {
	ID      string
	Ref     string
	Filter  string
	Yes     bool
	Force   bool
	Project string
	Section string
	Parent  string
}

type ResolveMoveResult struct {
	Mode   string
	ID     string
	IDs    []string
	Filter string
}

type ResolveTaskTargetInput struct {
	ID  string
	Ref string
}

func (s Service) ResolveCompletionTargets(ctx context.Context, in ResolveCompletionInput) (ResolveCompletionResult, error) {
	id, err := normalizeTaskID(in.ID)
	if err != nil {
		return ResolveCompletionResult{}, err
	}
	ref := strings.TrimSpace(in.Ref)
	filter := strings.TrimSpace(in.Filter)

	if filter != "" {
		if id != "" || ref != "" {
			return ResolveCompletionResult{}, errors.New("--filter cannot be combined with --id or positional task reference")
		}
		if !in.Yes && !in.Force {
			return ResolveCompletionResult{}, errors.New("bulk complete with --filter requires --yes (or --force)")
		}
		if s.Lister == nil {
			return ResolveCompletionResult{}, errors.New("task filter lister is not configured")
		}
		tasks, err := s.Lister.ListByFilter(ctx, filter)
		if err != nil {
			return ResolveCompletionResult{}, err
		}
		ids := make([]string, 0, len(tasks))
		for _, task := range tasks {
			ids = append(ids, task.ID)
		}
		return ResolveCompletionResult{Mode: "bulk", IDs: ids, Filter: filter}, nil
	}

	if id == "" && ref != "" {
		if s.Resolver == nil {
			return ResolveCompletionResult{}, errors.New("task resolver is not configured")
		}
		task, err := s.Resolver.ResolveTaskRef(ctx, ref)
		if err != nil {
			return ResolveCompletionResult{}, err
		}
		id = task.ID
	}

	if id == "" {
		return ResolveCompletionResult{}, errors.New("task complete requires --id or a reference")
	}
	return ResolveCompletionResult{Mode: "single", ID: id, IDs: []string{id}}, nil
}

func (s Service) ResolveMoveTargets(ctx context.Context, in ResolveMoveInput) (ResolveMoveResult, error) {
	id, err := normalizeTaskID(in.ID)
	if err != nil {
		return ResolveMoveResult{}, err
	}
	ref := strings.TrimSpace(in.Ref)
	filter := strings.TrimSpace(in.Filter)

	if strings.TrimSpace(in.Project) == "" && strings.TrimSpace(in.Section) == "" && strings.TrimSpace(in.Parent) == "" {
		return ResolveMoveResult{}, errors.New("at least one of --project, --section, or --parent is required")
	}

	if filter != "" {
		if id != "" || ref != "" {
			return ResolveMoveResult{}, errors.New("--filter cannot be combined with --id or positional task reference")
		}
		if !in.Yes && !in.Force {
			return ResolveMoveResult{}, errors.New("bulk move with --filter requires --yes (or --force)")
		}
		if s.Lister == nil {
			return ResolveMoveResult{}, errors.New("task filter lister is not configured")
		}
		tasks, err := s.Lister.ListByFilter(ctx, filter)
		if err != nil {
			return ResolveMoveResult{}, err
		}
		ids := make([]string, 0, len(tasks))
		for _, task := range tasks {
			ids = append(ids, task.ID)
		}
		return ResolveMoveResult{Mode: "bulk", IDs: ids, Filter: filter}, nil
	}

	if id == "" && ref != "" {
		if s.Resolver == nil {
			return ResolveMoveResult{}, errors.New("task resolver is not configured")
		}
		task, err := s.Resolver.ResolveTaskRef(ctx, ref)
		if err != nil {
			return ResolveMoveResult{}, err
		}
		id = task.ID
	}
	if id == "" {
		return ResolveMoveResult{}, errors.New("--id is required (or pass a text reference)")
	}
	return ResolveMoveResult{Mode: "single", ID: id, IDs: []string{id}}, nil
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

func (r ResolveCompletionResult) Validate() error {
	switch r.Mode {
	case "single":
		if strings.TrimSpace(r.ID) == "" {
			return fmt.Errorf("single mode requires id")
		}
	case "bulk":
		if strings.TrimSpace(r.Filter) == "" {
			return fmt.Errorf("bulk mode requires filter")
		}
	default:
		return fmt.Errorf("unknown mode: %s", r.Mode)
	}
	return nil
}
