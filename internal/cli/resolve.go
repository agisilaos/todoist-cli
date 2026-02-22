package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
)

func resolveProjectID(ctx *Context, value string) (string, error) {
	normalized, directID, err := apprefs.NormalizeEntityRef(value, "project")
	if err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if normalized == "" {
		return "", nil
	}
	if directID {
		return normalized, nil
	}
	projects, err := listAllProjects(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, normalized) {
			return p.ID, nil
		}
	}
	if useFuzzy(ctx) {
		candidates := fuzzyCandidates(normalized, projects, func(p api.Project) string { return p.Name }, func(p api.Project) string { return p.ID })
		if len(candidates) == 1 {
			return candidates[0].ID, nil
		}
		if len(candidates) > 1 {
			if chosen, ok, err := promptAmbiguousChoice(ctx, "project", normalized, candidates); err != nil {
				return "", err
			} else if ok {
				return chosen, nil
			}
			return "", ambiguousMatchCodeError("project", normalized, candidates)
		}
	}
	return normalized, nil
}

func resolveSectionID(ctx *Context, value string, project string) (string, error) {
	normalized, directID := apprefs.NormalizeRef(value)
	if normalized == "" {
		return "", nil
	}
	if directID {
		return normalized, nil
	}
	sections, err := listAllSections(ctx, project)
	if err != nil {
		return "", err
	}
	for _, s := range sections {
		if strings.EqualFold(s.Name, normalized) {
			return s.ID, nil
		}
	}
	if useFuzzy(ctx) {
		candidates := fuzzyCandidates(normalized, sections, func(s api.Section) string { return s.Name }, func(s api.Section) string { return s.ID })
		if len(candidates) == 1 {
			return candidates[0].ID, nil
		}
		if len(candidates) > 1 {
			if chosen, ok, err := promptAmbiguousChoice(ctx, "section", normalized, candidates); err != nil {
				return "", err
			} else if ok {
				return chosen, nil
			}
			return "", ambiguousMatchCodeError("section", normalized, candidates)
		}
	}
	return normalized, nil
}

func resolveLabelName(ctx *Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	labels, err := listAllLabels(ctx)
	if err != nil {
		return "", err
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

type fuzzyCandidate = apprefs.Candidate
type AmbiguousMatchError = apprefs.AmbiguousMatchError

func fuzzyCandidates[T any](value string, items []T, nameFn func(T) string, idFn func(T) string) []fuzzyCandidate {
	return apprefs.FuzzyCandidates(value, items, nameFn, idFn)
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
	id, candidates := apprefs.ResolveFuzzy(value, items, nameFn, idFn)
	if id != "" {
		return id, nil
	}
	if len(candidates) > 1 {
		return "", ambiguousMatchCodeError("value", value, candidates)
	}
	return "", nil
}

func listAllProjects(ctx *Context) ([]api.Project, error) {
	if cache := ctx.cache(); cache != nil && cache.projectsLoaded {
		return cloneSlice(cache.projects), nil
	}
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "200")
	all, _, err := fetchPaginated[api.Project](ctx, "/projects", query, true)
	if err != nil {
		return nil, err
	}
	if cache := ctx.cache(); cache != nil {
		cache.projects = cloneSlice(all)
		cache.projectsLoaded = true
	}
	return all, nil
}

func listAllSections(ctx *Context, project string) ([]api.Section, error) {
	key := strings.TrimSpace(project)
	if cache := ctx.cache(); cache != nil {
		if cached, ok := cache.sectionsByProject[key]; ok {
			return cloneSlice(cached), nil
		}
	}
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "200")
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err == nil && id != "" {
			query.Set("project_id", id)
			key = id
		}
	}
	all, _, err := fetchPaginated[api.Section](ctx, "/sections", query, true)
	if err != nil {
		return nil, err
	}
	if cache := ctx.cache(); cache != nil {
		cache.sectionsByProject[key] = cloneSlice(all)
		if trimmed := strings.TrimSpace(project); trimmed != "" && trimmed != key {
			cache.sectionsByProject[trimmed] = cloneSlice(all)
		}
	}
	return all, nil
}

func listAllLabels(ctx *Context) ([]api.Label, error) {
	if cache := ctx.cache(); cache != nil && cache.labelsLoaded {
		return cloneSlice(cache.labels), nil
	}
	if err := ensureClient(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("limit", "200")
	all, _, err := fetchPaginated[api.Label](ctx, "/labels", query, true)
	if err != nil {
		return nil, err
	}
	if cache := ctx.cache(); cache != nil {
		cache.labels = cloneSlice(all)
		cache.labelsLoaded = true
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
