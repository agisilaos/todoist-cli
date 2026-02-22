package tasks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
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

func (s Service) ResolveCompletionTargets(ctx context.Context, in ResolveCompletionInput) (ResolveCompletionResult, error) {
	id := stripIDPrefix(in.ID)
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

	id = stripIDPrefix(id)
	if id == "" {
		return ResolveCompletionResult{}, errors.New("task complete requires --id or a reference")
	}
	return ResolveCompletionResult{Mode: "single", ID: id, IDs: []string{id}}, nil
}

func stripIDPrefix(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "id:") {
		return strings.TrimSpace(value[3:])
	}
	return value
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
