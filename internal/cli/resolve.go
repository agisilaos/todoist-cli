package cli

import (
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
	return value, nil
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
