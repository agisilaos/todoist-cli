package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func writeTaskView(ctx *Context, task api.Task, full bool) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, task, output.Meta{RequestID: ctxRequestIDValue(ctx)})
	}
	fmt.Fprintf(ctx.Stdout, "ID: %s\n", task.ID)
	fmt.Fprintf(ctx.Stdout, "Content: %s\n", task.Content)
	if task.Description != "" {
		fmt.Fprintf(ctx.Stdout, "Description: %s\n", task.Description)
	}
	if task.ProjectID != "" {
		fmt.Fprintf(ctx.Stdout, "Project: %s\n", task.ProjectID)
	}
	if task.SectionID != "" {
		fmt.Fprintf(ctx.Stdout, "Section: %s\n", task.SectionID)
	}
	if len(task.Labels) > 0 {
		fmt.Fprintf(ctx.Stdout, "Labels: %s\n", strings.Join(task.Labels, ", "))
	}
	if task.Due != nil {
		fmt.Fprintf(ctx.Stdout, "Due: %s\n", formatDue(task.Due))
	}
	if full {
		fmt.Fprintf(ctx.Stdout, "Priority: %d\n", task.Priority)
		fmt.Fprintf(ctx.Stdout, "Completed: %v\n", task.Checked)
		fmt.Fprintf(ctx.Stdout, "Added: %s\n", task.AddedAt)
		fmt.Fprintf(ctx.Stdout, "Updated: %s\n", task.UpdatedAt)
		fmt.Fprintf(ctx.Stdout, "CompletedAt: %s\n", task.CompletedAt)
		fmt.Fprintf(ctx.Stdout, "NoteCount: %d\n", task.NoteCount)
	}
	return nil
}

type taskTableConfig struct {
	ID       int
	Content  int
	Project  int
	Section  int
	Labels   int
	Due      int
	Priority int
	Status   int
}

func taskTableConfigFor(ctx *Context, wide bool) taskTableConfig {
	cfg := taskTableConfig{
		ID:       8,
		Content:  50,
		Project:  16,
		Section:  12,
		Labels:   12,
		Due:      12,
		Priority: 8,
		Status:   9,
	}
	if wide {
		cfg.ID = 12
		cfg.Content = 80
		cfg.Project = 24
		cfg.Section = 18
		cfg.Labels = 18
		cfg.Due = 16
	}
	columns := tableWidth(ctx)
	fixed := cfg.ID + cfg.Project + cfg.Section + cfg.Labels + cfg.Due + cfg.Priority + cfg.Status + 7
	available := columns - fixed
	if available < 20 {
		available = 20
	}
	maxContent := 80
	if wide {
		maxContent = 120
	}
	if available < cfg.Content {
		cfg.Content = available
	} else if available < maxContent {
		cfg.Content = available
	} else {
		cfg.Content = maxContent
	}
	return cfg
}

func writeTaskList(ctx *Context, tasks []api.Task, cursor string, wide bool) error {
	if tasks == nil {
		tasks = []api.Task{}
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, tasks, output.Meta{RequestID: ctxRequestIDValue(ctx), Count: len(tasks), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		return writeTaskNDJSON(ctx, tasks)
	}
	cfg := taskTableConfigFor(ctx, wide)
	projectNames := map[string]string(nil)
	sectionNames := map[string]string(nil)
	if ctx.Mode == output.ModeHuman {
		projectNames = projectNameMap(ctx)
		sectionNames = sectionNameMap(ctx)
	}
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		project := task.ProjectID
		if name, ok := projectNames[task.ProjectID]; ok {
			project = name
		}
		section := task.SectionID
		if name, ok := sectionNames[task.SectionID]; ok {
			section = name
		}
		labels := cleanCell(strings.Join(task.Labels, ","))
		content := cleanCell(task.Content)
		id := cleanCell(task.ID)
		due := formatDue(task.Due)
		if ctx.Mode == output.ModeHuman {
			content = truncateString(content, cfg.Content)
			project = truncateString(cleanCell(project), cfg.Project)
			section = truncateString(cleanCell(section), cfg.Section)
			labels = truncateString(labels, cfg.Labels)
			id = shortID(id, cfg.ID, wide)
			due = truncateString(due, cfg.Due)
		}
		rows = append(rows, []string{
			id,
			content,
			project,
			section,
			labels,
			due,
			strconv.Itoa(task.Priority),
			formatCompleted(task.Checked, task.CompletedAt),
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Content", "Project", "Section", "Labels", "Due", "Priority", "Completed"}, rows)
}

func writeTaskNDJSON(ctx *Context, tasks []api.Task) error {
	items := make([]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, task)
	}
	return output.WriteNDJSON(ctx.Stdout, items)
}

func formatDue(due *api.Due) string {
	if due == nil {
		return ""
	}
	if due.Datetime != "" {
		return due.Datetime
	}
	if due.Date != "" {
		return due.Date
	}
	if due.String != "" {
		return due.String
	}
	return ""
}

func formatCompleted(checked bool, completedAt string) string {
	if checked || completedAt != "" {
		return "yes"
	}
	return "no"
}

func sortTasks(tasks []api.Task, sortBy string) {
	switch sortBy {
	case "":
		return
	case "priority":
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].Priority > tasks[j].Priority
		})
	case "due":
		sort.SliceStable(tasks, func(i, j int) bool {
			return parseDue(tasks[i].Due).Before(parseDue(tasks[j].Due))
		})
	default:
		// ignore unknown sort
	}
}

func parseDue(due *api.Due) time.Time {
	if due == nil {
		return time.Time{}
	}
	if due.Datetime != "" {
		if t, err := time.Parse(time.RFC3339, due.Datetime); err == nil {
			return t
		}
	}
	if due.Date != "" {
		if t, err := time.Parse("2006-01-02", due.Date); err == nil {
			return t
		}
	}
	return time.Time{}
}
