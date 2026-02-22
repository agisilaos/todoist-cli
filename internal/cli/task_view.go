package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func taskView(ctx *Context, args []string) error {
	fs := newFlagSet("task view")
	var id string
	var full bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.BoolVar(&full, "full", false, "Show full task fields")
	bindHelpFlag(fs, &help)
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
	original := strings.TrimSpace(ref)
	explicitID := strings.HasPrefix(strings.ToLower(original), "id:")
	ref = stripIDPrefix(original)
	if explicitID || isNumeric(ref) {
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
	if contentRef, dueHint, ok := parseNaturalTaskReference(ref); ok {
		matches := matchTasksByContent(tasks, contentRef)
		dueMatched := filterTasksByDueHint(matches, dueHint, ctx.Now().UTC())
		if len(dueMatched) == 1 {
			return dueMatched[0], nil
		}
		if len(dueMatched) > 1 {
			candidates := taskCandidates(dueMatched)
			if chosen, ok, err := promptAmbiguousChoice(ctx, "task", ref, candidates); err != nil {
				return api.Task{}, err
			} else if ok {
				for _, task := range dueMatched {
					if task.ID == chosen {
						return task, nil
					}
				}
			}
			return api.Task{}, ambiguousMatchCodeError("task", ref, candidates)
		}
	}

	var exactMatches []api.Task
	for _, task := range tasks {
		if strings.EqualFold(strings.TrimSpace(task.Content), ref) {
			exactMatches = append(exactMatches, task)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0], nil
	}
	if len(exactMatches) > 1 {
		candidates := taskCandidates(exactMatches)
		if chosen, ok, err := promptAmbiguousChoice(ctx, "task", ref, candidates); err != nil {
			return api.Task{}, err
		} else if ok {
			for _, task := range exactMatches {
				if task.ID == chosen {
					return task, nil
				}
			}
		}
		return api.Task{}, ambiguousMatchCodeError("task", ref, candidates)
	}

	contains := matchTasksByContent(tasks, ref)
	if len(contains) == 1 {
		return contains[0], nil
	}
	if len(contains) > 1 && !useFuzzy(ctx) {
		candidates := taskCandidates(contains)
		if chosen, ok, err := promptAmbiguousChoice(ctx, "task", ref, candidates); err != nil {
			return api.Task{}, err
		} else if ok {
			for _, task := range contains {
				if task.ID == chosen {
					return task, nil
				}
			}
		}
		return api.Task{}, ambiguousMatchCodeError("task", ref, candidates)
	}

	if !useFuzzy(ctx) {
		return api.Task{}, &CodeError{Code: exitNotFound, Err: fmt.Errorf("task %q not found", ref)}
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

func taskCandidates(tasks []api.Task) []fuzzyCandidate {
	candidates := make([]fuzzyCandidate, 0, len(tasks))
	for _, task := range tasks {
		candidates = append(candidates, fuzzyCandidate{
			ID:   task.ID,
			Name: task.Content,
		})
	}
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	return candidates
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

func parseNaturalTaskReference(ref string) (content string, dueHint string, ok bool) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", "", false
	}
	lower := strings.ToLower(trimmed)
	for _, hint := range []string{"today", "tomorrow", "overdue"} {
		suffix := " " + hint
		if !strings.HasSuffix(lower, suffix) {
			continue
		}
		content = strings.TrimSpace(trimmed[:len(trimmed)-len(suffix)])
		if content == "" {
			return "", "", false
		}
		return content, hint, true
	}
	return "", "", false
}

func filterTasksByDueHint(tasks []api.Task, dueHint string, now time.Time) []api.Task {
	hint := strings.ToLower(strings.TrimSpace(dueHint))
	if hint == "" {
		return tasks
	}
	today := now.UTC().Format("2006-01-02")
	tomorrow := now.UTC().AddDate(0, 0, 1).Format("2006-01-02")
	out := make([]api.Task, 0, len(tasks))
	for _, task := range tasks {
		taskDate := taskDueDate(task)
		if taskDate == "" {
			continue
		}
		switch hint {
		case "today":
			if taskDate == today {
				out = append(out, task)
			}
		case "tomorrow":
			if taskDate == tomorrow {
				out = append(out, task)
			}
		case "overdue":
			if taskDate < today {
				out = append(out, task)
			}
		}
	}
	return out
}

func taskDueDate(task api.Task) string {
	if task.Due == nil {
		return ""
	}
	if len(task.Due.Date) >= 10 {
		return task.Due.Date[:10]
	}
	if task.Due.Datetime != "" {
		if t, err := time.Parse(time.RFC3339, task.Due.Datetime); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return ""
}
