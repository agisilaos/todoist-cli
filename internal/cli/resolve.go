package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func resolveProjectID(ctx *Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	projects, err := listAllProjects(ctx)
	if err != nil {
		return value, nil
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, value) {
			return p.ID, nil
		}
	}
	if useFuzzy(ctx) {
		if id, err := resolveFuzzy(value, projects, func(p api.Project) string { return p.Name }, func(p api.Project) string { return p.ID }); err == nil {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
	return value, nil
}

func resolveSectionID(ctx *Context, value string, project string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	sections, err := listAllSections(ctx, project)
	if err != nil {
		return value, nil
	}
	for _, s := range sections {
		if strings.EqualFold(s.Name, value) {
			return s.ID, nil
		}
	}
	if useFuzzy(ctx) {
		if id, err := resolveFuzzy(value, sections, func(s api.Section) string { return s.Name }, func(s api.Section) string { return s.ID }); err == nil {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
	return value, nil
}

func resolveLabelName(ctx *Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	labels, err := listAllLabels(ctx)
	if err != nil {
		return value, nil
	}
	for _, l := range labels {
		if strings.EqualFold(l.Name, value) {
			return l.Name, nil
		}
	}
	if useFuzzy(ctx) {
		if name, err := resolveFuzzy(value, labels, func(l api.Label) string { return l.Name }, func(l api.Label) string { return l.Name }); err == nil {
			return name, nil
		} else if err != nil {
			return "", err
		}
	}
	return value, nil
}

func resolveFuzzy[T any](value string, items []T, nameFn func(T) string, idFn func(T) string) (string, error) {
	var matches []T
	lower := strings.ToLower(value)
	for _, item := range items {
		name := strings.ToLower(nameFn(item))
		if strings.Contains(name, lower) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 1 {
		return idFn(matches[0]), nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, nameFn(m))
		}
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("ambiguous match for %q; matches: %s", value, strings.Join(names, ", "))}
	}
	return "", nil
}

func listAllProjects(ctx *Context) ([]api.Project, error) {
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "200")
	var all []api.Project
	for {
		var page api.Paginated[api.Project]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/projects", query, &page)
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

func listAllSections(ctx *Context, project string) ([]api.Section, error) {
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "200")
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err == nil && id != "" {
			query.Set("project_id", id)
		}
	}
	var all []api.Section
	for {
		var page api.Paginated[api.Section]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/sections", query, &page)
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

func listAllLabels(ctx *Context) ([]api.Label, error) {
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "200")
	var all []api.Label
	for {
		var page api.Paginated[api.Label]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/labels", query, &page)
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

func parseLimit(limit int) string {
	if limit <= 0 {
		return "50"
	}
	return strconv.Itoa(limit)
}

func inboxProjectID(ctx *Context) (string, error) {
	projects, err := listAllProjects(ctx)
	if err != nil {
		return "", err
	}
	for _, project := range projects {
		if project.IsInbox {
			return project.ID, nil
		}
	}
	return "", nil
}

func projectNameMap(ctx *Context) map[string]string {
	projects, err := listAllProjects(ctx)
	if err != nil {
		return nil
	}
	names := make(map[string]string, len(projects))
	for _, project := range projects {
		names[project.ID] = project.Name
	}
	return names
}

func sectionNameMap(ctx *Context) map[string]string {
	sections, err := listAllSections(ctx, "")
	if err != nil {
		return nil
	}
	names := make(map[string]string, len(sections))
	for _, section := range sections {
		names[section.ID] = section.Name
	}
	return names
}
