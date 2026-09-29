package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

// Only active list commands opt in. Other callers keep writeTaskList's contracts.
type taskOverview struct {
	Scope         string
	Empty         string
	Continued     bool
	ReferenceDate string
}

func writeTaskOverview(ctx *Context, tasks []api.Task, cursor string, wide bool, view taskOverview) error {
	if ctx.Mode != output.ModeHuman {
		return writeTaskList(ctx, tasks, cursor, wide)
	}
	if view.ReferenceDate == "" {
		now := time.Now()
		if ctx.Now != nil {
			now = ctx.Now()
		}
		view.ReferenceDate = now.UTC().Format("2006-01-02")
	}
	width := tableWidth(ctx)
	var b strings.Builder
	line := func(parts ...string) {
		for _, part := range wrapOverviewParts(parts, width) {
			fmt.Fprintln(&b, part)
		}
	}
	if !ctx.Global.Quiet {
		coverage := "All pages fetched"
		if cursor != "" {
			coverage = "More available"
		} else if view.Continued {
			coverage = "End of results"
		}
		if view.Continued {
			coverage = "Continuation · " + coverage
		}
		line(view.Scope, fmt.Sprintf("%d shown", len(tasks)), coverage)
		overdue, today := 0, 0
		for _, task := range tasks {
			_, category := overviewDue(task, view.ReferenceDate)
			switch category {
			case "overdue":
				overdue++
			case "today":
				today++
			}
		}
		line(fmt.Sprintf("Due labels: %s (UTC)", view.ReferenceDate), fmt.Sprintf("%d overdue", overdue), fmt.Sprintf("%d due today", today))
		if len(tasks) == 0 {
			empty := view.Empty
			if cursor != "" {
				empty = "No tasks on this page; more available."
			} else if view.Continued {
				empty = "No tasks returned in this continuation."
			}
			line(empty)
		} else {
			b.WriteByte('\n')
		}
	}
	if !wide && len(tasks) > 0 {
		projects := projectNameMap(ctx)
		for i, task := range tasks {
			if i > 0 {
				b.WriteByte('\n')
			}
			priority := "P?"
			if task.Priority >= 1 && task.Priority <= 4 {
				priority = "P" + strconv.Itoa(5-task.Priority)
			}
			title := wrapOverviewText(captureText(task.Content), width-4, true)
			if len(title) > 3 {
				title = title[:3]
				title[2] = overviewEllipsis(title[2], width-4)
			}
			for j, part := range title {
				prefix := "    "
				if j == 0 {
					prefix = priority + "  "
				}
				fmt.Fprintln(&b, prefix+part)
			}
			due, _ := overviewDue(task, view.ReferenceDate)
			context := []string{due}
			if task.Due != nil && task.Due.IsRecurring != nil && *task.Due.IsRecurring {
				context = append(context, "Repeats")
			}
			project := "Project unavailable"
			if task.ProjectID != "" {
				project = captureDestination(task.ProjectID, projects)
			}
			context = append(context, project, "ID "+returnedText(task.ID))
			for _, part := range wrapOverviewParts(context, width-4) {
				fmt.Fprintln(&b, "    "+part)
			}
		}
	}
	if _, err := fmt.Fprint(ctx.Stdout, b.String()); err != nil {
		return err
	}
	if wide && len(tasks) > 0 {
		if err := writeTaskList(ctx, tasks, "", true); err != nil {
			return err
		}
	}
	return writeCursorNotice(ctx, cursor)
}

func activeTaskScope(project, section, parent, label, ids string) string {
	parts := []string{}
	for _, field := range []struct{ name, value string }{
		{"Project", project}, {"Section", section}, {"Parent", parent}, {"Label", label}, {"IDs", ids},
	} {
		if field.value != "" {
			parts = append(parts, field.name+": "+strconv.Quote(field.value))
		}
	}
	if len(parts) == 0 {
		return "All projects · Active tasks"
	}
	return "Active tasks · " + strings.Join(parts, " · ")
}

func overviewDue(task api.Task, today string) (string, string) {
	if task.Due == nil {
		if task.DueReturned {
			return "No due date", ""
		}
		return "Due unavailable", ""
	}
	value := formatDue(task.Due)
	if task.Due.Date != "" && task.Due.Datetime != "" && !strings.HasPrefix(task.Due.Datetime, task.Due.Date) {
		// Retain the calendar date that explains the label when the timestamp
		// represents it with a different offset or returned value.
		value = task.Due.Date + " · " + task.Due.Datetime
	}
	if value == "" {
		return "Due unavailable", ""
	}
	date := ""
	// Match existing calendar-date precedence, but don't classify malformed dates.
	if task.Due.Date != "" {
		for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
			if parsed, err := time.Parse(layout, task.Due.Date); err == nil {
				date = parsed.Format("2006-01-02")
				break
			}
		}
	} else if parsed, err := time.Parse(time.RFC3339, task.Due.Datetime); err == nil {
		date = parsed.UTC().Format("2006-01-02")
	}
	if task.Due.Timezone != nil && *task.Due.Timezone != "" {
		value += " (" + *task.Due.Timezone + ")"
	}
	switch {
	case date == "":
		return "Due " + value + " (date unavailable)", ""
	case date < today:
		return "Overdue · " + value, "overdue"
	case date == today:
		return "Today · " + value, "today"
	default:
		return "Due " + value, "future"
	}
}

// Keep related metadata (such as "ID <value>") together whenever it fits.
// Separators belong between fields on a line, never alone at a wrapped edge.
func wrapOverviewParts(parts []string, width int) []string {
	lines := []string{}
	for _, part := range parts {
		wrapped := wrapOverviewText(captureText(part), width, false)
		last := len(lines) - 1
		if last >= 0 && overviewTextWidth(lines[last])+3+overviewTextWidth(wrapped[0]) <= width {
			lines[last] += " · " + wrapped[0]
			wrapped = wrapped[1:]
		}
		lines = append(lines, wrapped...)
	}
	return lines
}

// Metadata keeps indivisible values (especially IDs and timestamps) intact.
// Titles may split long words, and their caller limits them to three lines.
func wrapOverviewText(text string, width int, splitWords bool) []string {
	if width < 1 {
		width = 1
	}
	lines := []string{}
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && overviewTextWidth(line)+1+overviewTextWidth(word) <= width {
			line += " " + word
			continue
		}
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
		if splitWords {
			for _, r := range word {
				if line != "" && overviewTextWidth(line)+overviewRuneWidth(r) > width {
					lines = append(lines, line)
					line = ""
				}
				line += string(r)
			}
		} else {
			line = word
		}
	}
	if line != "" || len(lines) == 0 {
		lines = append(lines, line)
	}
	return lines
}

func overviewEllipsis(text string, width int) string {
	runes := []rune(strings.TrimSpace(text))
	for len(runes) > 0 && overviewTextWidth(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimRightFunc(string(runes), unicode.IsSpace) + "…"
}

func overviewTextWidth(text string) int {
	width := 0
	for _, r := range text {
		width += overviewRuneWidth(r)
	}
	return width
}

// Conservative terminal-cell widths without adding a rendering dependency.
// Ambiguous-width characters use one cell, as in typical Western terminals.
func overviewRuneWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff) || (r >= 0x20000 && r <= 0x3fffd)) {
		return 2
	}
	return 1
}
