package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type plannerContextOptions struct {
	ProjectFilters []string
	LabelFilters   []string
	CompletedDays  int
}

func parseContextOptions(ctx *Context, projects, labels []string, completed string) (plannerContextOptions, error) {
	days := 0
	if completed != "" {
		val, err := parseDays(completed)
		if err != nil {
			return plannerContextOptions{}, err
		}
		days = val
	}
	return plannerContextOptions{
		ProjectFilters: projects,
		LabelFilters:   labels,
		CompletedDays:  days,
	}, nil
}

func buildPlannerContext(ctx *Context, opts plannerContextOptions) (PlannerContext, error) {
	projects, err := listAllProjects(ctx)
	if err != nil {
		return PlannerContext{}, err
	}
	projectIDs, err := filterProjectIDs(ctx, projects, opts.ProjectFilters)
	if err != nil {
		return PlannerContext{}, err
	}
	filteredProjects := filterProjects(projects, projectIDs)

	sections, err := listAllSections(ctx, "")
	if err != nil {
		return PlannerContext{}, err
	}
	filteredSections := filterSections(sections, projectIDs)

	labels, err := listAllLabels(ctx)
	if err != nil {
		return PlannerContext{}, err
	}
	filteredLabels, err := filterLabels(ctx, labels, opts.LabelFilters)
	if err != nil {
		return PlannerContext{}, err
	}

	var completed []api.Task
	if opts.CompletedDays > 0 {
		since := ctx.Now().AddDate(0, 0, -opts.CompletedDays).UTC().Format(time.RFC3339)
		completed, err = listCompletedTasks(ctx, since)
		if err != nil {
			return PlannerContext{}, err
		}
	}
	activeTasks, err := listAllActiveTasks(ctx)
	if err != nil {
		return PlannerContext{}, err
	}
	filteredActiveTasks := filterActiveTasksForContext(activeTasks, projectIDs, opts.LabelFilters)

	return PlannerContext{
		Projects:       toAnySlice(filteredProjects),
		Sections:       toAnySlice(filteredSections),
		Labels:         toAnySlice(filteredLabels),
		ActiveTasks:    toAnySlice(filteredActiveTasks),
		CompletedTasks: toAnySlice(completed),
	}, nil
}

func parseDays(value string) (int, error) {
	val := strings.TrimSpace(strings.ToLower(value))
	val = strings.TrimSuffix(val, "d")
	days, err := strconv.Atoi(val)
	if err != nil || days < 0 {
		return 0, &CodeError{Code: exitUsage, Err: errors.New("context-completed must be an integer or Nd")}
	}
	return days, nil
}

func filterProjectIDs(ctx *Context, projects []api.Project, filters []string) (map[string]struct{}, error) {
	if len(filters) == 0 {
		return nil, nil
	}
	ids := make(map[string]struct{})
	for _, f := range filters {
		found := false
		for _, p := range projects {
			if strings.EqualFold(p.ID, f) || strings.EqualFold(p.Name, f) {
				ids[p.ID] = struct{}{}
				found = true
				break
			}
		}
		if !found && useFuzzy(ctx) {
			if id, err := resolveFuzzy(f, projects, func(p api.Project) string { return p.Name }, func(p api.Project) string { return p.ID }); err == nil {
				ids[id] = struct{}{}
				found = true
			} else if err != nil {
				return nil, err
			}
		}
		if !found {
			return nil, &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown project: %s", f)}
		}
	}
	return ids, nil
}

func filterProjects(projects []api.Project, ids map[string]struct{}) []api.Project {
	if ids == nil || len(ids) == 0 {
		return projects
	}
	out := make([]api.Project, 0, len(projects))
	for _, p := range projects {
		if _, ok := ids[p.ID]; ok {
			out = append(out, p)
		}
	}
	return out
}

func filterSections(sections []api.Section, projectIDs map[string]struct{}) []api.Section {
	if projectIDs == nil || len(projectIDs) == 0 {
		return sections
	}
	out := make([]api.Section, 0, len(sections))
	for _, s := range sections {
		if _, ok := projectIDs[s.ProjectID]; ok {
			out = append(out, s)
		}
	}
	return out
}

func filterLabels(ctx *Context, labels []api.Label, filters []string) ([]api.Label, error) {
	if len(filters) == 0 {
		return labels, nil
	}
	out := make([]api.Label, 0, len(filters))
	for _, f := range filters {
		found := false
		for _, l := range labels {
			if strings.EqualFold(l.Name, f) || strings.EqualFold(l.ID, f) {
				out = append(out, l)
				found = true
				break
			}
		}
		if !found && useFuzzy(ctx) {
			if name, err := resolveFuzzy(f, labels, func(l api.Label) string { return l.Name }, func(l api.Label) string { return l.Name }); err == nil {
				for _, l := range labels {
					if strings.EqualFold(l.Name, name) {
						out = append(out, l)
						found = true
						break
					}
				}
			} else if err != nil {
				return nil, err
			}
		}
		if !found {
			return nil, &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown label: %s", f)}
		}
	}
	return out, nil
}

func listCompletedTasks(ctx *Context, since string) ([]api.Task, error) {
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "50")
	if since != "" {
		query.Set("since", since)
	}
	var all []api.Task
	for {
		var page api.Paginated[api.Task]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/tasks/completed/by_completion_date", query, &page)
		cancel()
		if err != nil {
			return nil, err
		}
		setRequestID(ctx, reqID)
		all = append(all, page.Results...)
		if page.NextCursor == "" {
			break
		}
		query.Set("cursor", page.NextCursor)
	}
	return all, nil
}

func filterActiveTasksForContext(tasks []api.Task, projectIDs map[string]struct{}, labelFilters []string) []api.Task {
	out := make([]api.Task, 0, len(tasks))
	labelSet := map[string]struct{}{}
	for _, label := range labelFilters {
		trimmed := strings.TrimSpace(strings.ToLower(label))
		if trimmed == "" {
			continue
		}
		labelSet[trimmed] = struct{}{}
	}
	for _, task := range tasks {
		if len(projectIDs) > 0 {
			if _, ok := projectIDs[task.ProjectID]; !ok {
				continue
			}
		}
		if len(labelSet) > 0 {
			hasAny := false
			for _, label := range task.Labels {
				if _, ok := labelSet[strings.ToLower(strings.TrimSpace(label))]; ok {
					hasAny = true
					break
				}
			}
			if !hasAny {
				continue
			}
		}
		out = append(out, task)
		if len(out) >= 50 {
			break
		}
	}
	return out
}
