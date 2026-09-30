package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func promptAmbiguousTaskChoice(ctx *Context, input string, candidates []fuzzyCandidate, tasks []api.Task) (string, bool, error) {
	if ctx == nil || ctx.Global.NoInput || !isTTYReader(ctx.Stdin) {
		return "", false, nil
	}
	byID := make(map[string]api.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	needProjects, needSections := false, false
	for _, candidate := range candidates {
		task := byID[candidate.ID]
		needProjects = needProjects || task.ProjectID != ""
		needSections = needSections || task.SectionID != ""
	}
	// Name lookup is optional presentation context. Keep the choice available
	// with IDs when enrichment fails, and avoid these reads for machine clients.
	var projects, sections map[string]string
	if needProjects {
		projects = projectNameMap(ctx)
	}
	if needSections {
		sections = sectionNameMap(ctx)
	}

	fmt.Fprintf(ctx.Stderr, "Multiple tasks match %q:\n", input)
	for i, candidate := range candidates {
		fmt.Fprintf(ctx.Stderr, "  %d) %s (id:%s)\n", i+1, captureText(candidate.Name), captureText(candidate.ID))
		parts := taskChoiceContext(byID[candidate.ID], projects, sections, byID)
		for _, line := range wrapOverviewParts(parts, tableWidth(ctx)-5) {
			fmt.Fprintln(ctx.Stderr, "     "+line)
		}
	}
	return readAmbiguousChoice(ctx, candidates)
}

func taskChoiceContext(task api.Task, projects, sections map[string]string, tasks map[string]api.Task) []string {
	project := "Project unavailable"
	if task.ProjectID != "" {
		project = "Project: " + captureDestination(task.ProjectID, projects)
	}
	parts := []string{project}
	if task.SectionID != "" {
		parts = append(parts, "Section: "+captureDestination(task.SectionID, sections))
	}
	// An empty reference day keeps due information absolute rather than relying
	// on relative labels to distinguish otherwise identical tasks.
	due, _ := overviewDue(task, "")
	parts = append(parts, due)
	if task.Due != nil && task.Due.IsRecurring != nil && *task.Due.IsRecurring {
		recurrence := "Repeats"
		if task.Due.String != "" {
			recurrence += ": " + task.Due.String
		}
		parts = append(parts, recurrence)
	}
	if task.ParentID != "" {
		parent := task.ParentID + " (name unavailable)"
		if title := tasks[task.ParentID].Content; title != "" {
			parent = title + " (id:" + task.ParentID + ")"
		}
		parts = append(parts, "Parent: "+parent)
	}
	if len(task.Labels) > 0 {
		labels := make([]string, len(task.Labels))
		for i, label := range task.Labels {
			labels[i] = strconv.Quote(label)
		}
		parts = append(parts, "Labels: "+strings.Join(labels, ", "))
	}
	return parts
}
