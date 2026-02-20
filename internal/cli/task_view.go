package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func taskView(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task view", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var full bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.BoolVar(&full, "full", false, "Show full task fields")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	ref := id
	if ref == "" && len(fs.Args()) > 0 {
		ref = strings.Join(fs.Args(), " ")
	}
	if ref == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("task view requires id or text reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	task, err := resolveTaskRef(ctx, ref)
	if err != nil {
		return err
	}
	return writeTaskView(ctx, task, full)
}

func resolveTaskRef(ctx *Context, ref string) (api.Task, error) {
	ref = strings.TrimSpace(ref)
	ref = stripIDPrefix(ref)
	if isNumeric(ref) {
		var task api.Task
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/tasks/"+ref, nil, &task)
		cancel()
		if err != nil {
			if isLegacyV1IDError(err) {
				return api.Task{}, &CodeError{Code: exitUsage, Err: errors.New("legacy numeric task IDs are not supported by Todoist API v1; use a current task ID from `todoist task list --json` or use text reference")}
			}
			return api.Task{}, err
		}
		setRequestID(ctx, reqID)
		return task, nil
	}
	tasks, err := listAllActiveTasks(ctx)
	if err != nil {
		return api.Task{}, err
	}
	candidates := fuzzyCandidates(ref, tasks, func(t api.Task) string { return t.Content }, func(t api.Task) string { return t.ID })
	if len(candidates) == 1 {
		for _, task := range tasks {
			if task.ID == candidates[0].ID {
				return task, nil
			}
		}
	}
	if len(candidates) > 1 {
		if chosen, ok, err := promptAmbiguousChoice(ctx, "task", ref, candidates); err != nil {
			return api.Task{}, err
		} else if ok {
			for _, task := range tasks {
				if task.ID == chosen {
					return task, nil
				}
			}
		}
		return api.Task{}, ambiguousMatchCodeError("task", ref, candidates)
	}
	return api.Task{}, &CodeError{Code: exitNotFound, Err: fmt.Errorf("task %q not found", ref)}
}

func isLegacyV1IDError(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return strings.Contains(apiErr.Message, "V1_ID_CANNOT_BE_USED")
}

func listAllActiveTasks(ctx *Context) ([]api.Task, error) {
	if cache := ctx.cache(); cache != nil && cache.activeTasksLoaded {
		return cloneSlice(cache.activeTasks), nil
	}
	query := url.Values{}
	query.Set("limit", "200")
	all, _, err := fetchPaginated[api.Task](ctx, "/tasks", query, true)
	if err != nil {
		return nil, err
	}
	if cache := ctx.cache(); cache != nil {
		cache.activeTasks = cloneSlice(all)
		cache.activeTasksLoaded = true
	}
	return all, nil
}

func matchTasksByContent(tasks []api.Task, ref string) []api.Task {
	refLower := strings.ToLower(ref)
	var matches []api.Task
	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.Content), refLower) {
			matches = append(matches, task)
		}
	}
	return matches
}
