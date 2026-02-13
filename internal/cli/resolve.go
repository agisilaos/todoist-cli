package cli

import (
	"errors"
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
	value = stripIDPrefix(value)
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
		candidates := fuzzyCandidates(value, projects, func(p api.Project) string { return p.Name }, func(p api.Project) string { return p.ID })
		if len(candidates) == 1 {
			return candidates[0].ID, nil
		}
		if len(candidates) > 1 {
			if chosen, ok, err := promptAmbiguousChoice(ctx, "project", value, candidates); err != nil {
				return "", err
			} else if ok {
				return chosen, nil
			}
			return "", ambiguousMatchCodeError("project", value, candidates)
		}
	}
	return value, nil
}

func resolveSectionID(ctx *Context, value string, project string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	value = stripIDPrefix(value)
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
		candidates := fuzzyCandidates(value, sections, func(s api.Section) string { return s.Name }, func(s api.Section) string { return s.ID })
		if len(candidates) == 1 {
			return candidates[0].ID, nil
		}
		if len(candidates) > 1 {
			if chosen, ok, err := promptAmbiguousChoice(ctx, "section", value, candidates); err != nil {
				return "", err
			} else if ok {
				return chosen, nil
			}
			return "", ambiguousMatchCodeError("section", value, candidates)
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
		candidates := fuzzyCandidates(value, labels, func(l api.Label) string { return l.Name }, func(l api.Label) string { return l.Name })
		if len(candidates) == 1 {
			return candidates[0].ID, nil
		}
		if len(candidates) > 1 {
			if chosen, ok, err := promptAmbiguousChoice(ctx, "label", value, candidates); err != nil {
				return "", err
			} else if ok {
				return chosen, nil
			}
			return "", ambiguousMatchCodeError("label", value, candidates)
		}
	}
	return value, nil
}

type fuzzyCandidate struct {
	ID   string
	Name string
}

type AmbiguousMatchError struct {
	Entity  string
	Input   string
	Matches []string
}

func (e *AmbiguousMatchError) Error() string {
	return fmt.Sprintf("ambiguous %s match for %q; matches: %s", e.Entity, e.Input, strings.Join(e.Matches, ", "))
}

func fuzzyCandidates[T any](value string, items []T, nameFn func(T) string, idFn func(T) string) []fuzzyCandidate {
	var out []fuzzyCandidate
	lower := strings.ToLower(value)
	for _, item := range items {
		name := strings.ToLower(nameFn(item))
		if strings.Contains(name, lower) {
			out = append(out, fuzzyCandidate{ID: idFn(item), Name: nameFn(item)})
		}
	}
	return out
}

func ambiguousMatchCodeError(entity, input string, candidates []fuzzyCandidate) error {
	matches := make([]string, 0, len(candidates))
	for _, c := range candidates {
		matches = append(matches, c.Name)
	}
	return &CodeError{Code: exitUsage, Err: &AmbiguousMatchError{
		Entity:  entity,
		Input:   input,
		Matches: matches,
	}}
}

func promptAmbiguousChoice(ctx *Context, entity, input string, candidates []fuzzyCandidate) (string, bool, error) {
	if ctx == nil || ctx.Global.NoInput || !isTTYReader(ctx.Stdin) {
		return "", false, nil
	}
	fmt.Fprintf(ctx.Stderr, "Multiple %ss match %q:\n", entity, input)
	for i, c := range candidates {
		fmt.Fprintf(ctx.Stderr, "  %d) %s (id:%s)\n", i+1, c.Name, c.ID)
	}
	fmt.Fprint(ctx.Stderr, "Choose number (or press Enter to cancel): ")
	line, err := readLine(ctx.Stdin)
	if err != nil {
		return "", false, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false, nil
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(candidates) {
		return "", false, &CodeError{Code: exitUsage, Err: errors.New("invalid selection for ambiguous match")}
	}
	return candidates[idx-1].ID, true, nil
}

func resolveFuzzy[T any](value string, items []T, nameFn func(T) string, idFn func(T) string) (string, error) {
	candidates := fuzzyCandidates(value, items, nameFn, idFn)
	if len(candidates) == 1 {
		return candidates[0].ID, nil
	}
	if len(candidates) > 1 {
		return "", ambiguousMatchCodeError("value", value, candidates)
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
