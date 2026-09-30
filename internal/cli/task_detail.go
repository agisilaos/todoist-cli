package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func writeTaskDetail(ctx *Context, task api.Task, full bool) error {
	var b strings.Builder
	width := tableWidth(ctx)
	field := func(label, value string) {
		writeDetailText(&b, captureText(value), label+": ", "  ", width, false)
	}
	priority := task.Priority >= 1 && task.Priority <= 4
	ids := []string{task.ID, task.ProjectID, task.SectionID, task.ParentID}
	prefix := ""
	if priority {
		prefix = fmt.Sprintf("P%d  ", 5-task.Priority)
	}
	writeDetailText(&b, returnedText(task.Content), prefix, "    ", width, true, ids...)
	b.WriteByte('\n')
	project, section := detailDestinations(ctx, task)
	field("Project", project)
	field("Section", section)
	if !priority {
		field("Priority", "Not returned")
	}
	state := "Not returned"
	if task.Returned.Checked {
		state = "Active"
		if task.Checked {
			state = "Completed"
		}
	}
	field("State", state)
	if task.CompletedAt != "" {
		field("Completed at", task.CompletedAt)
	}
	writeDetailDue(task, field)
	labels := "Not returned"
	if task.Labels != nil {
		labels = "None"
		if len(task.Labels) > 0 {
			quoted := make([]string, len(task.Labels))
			for i, label := range task.Labels {
				quoted[i] = strconv.Quote(label)
			}
			labels = strings.Join(quoted, ", ")
		}
	}
	field("Labels", labels)
	field("ID", returnedText(task.ID))
	b.WriteString("\nDescription:\n")
	description := task.Description
	if description == "" {
		description = detailAbsent(task.Returned.Description)
	}
	writeDetailText(&b, description, "  ", "  ", width, true, ids...)
	if full {
		b.WriteByte('\n')
		field("Project ID", returnedText(task.ProjectID))
		field("Section ID", detailID(task.SectionID, task.Returned.SectionID))
		field("Parent ID", detailID(task.ParentID, task.Returned.ParentID))
		field("Added", returnedText(task.AddedAt))
		field("Updated", returnedText(task.UpdatedAt))
		if task.CompletedAt == "" {
			field("Completed at", detailAbsent(task.Returned.CompletedAt))
		}
		comments := "Not returned"
		if task.Returned.NoteCount {
			comments = strconv.Itoa(task.NoteCount)
		}
		field("Comments", comments)
		if task.Due != nil && task.Due.String != "" && (task.Due.Date != "" || task.Due.Datetime != "") &&
			(task.Due.IsRecurring == nil || !*task.Due.IsRecurring) {
			field("Due expression", task.Due.String)
		}
	}
	_, err := fmt.Fprint(ctx.Stdout, b.String())
	return err
}

func detailAbsent(returned bool) string {
	if returned {
		return "None"
	}
	return "Not returned"
}

func detailID(id string, returned bool) string {
	if id != "" {
		return id
	}
	return detailAbsent(returned)
}

func detailDestinations(ctx *Context, task api.Task) (string, string) {
	project := "Not returned"
	section := detailID(task.SectionID, task.Returned.SectionID)
	if task.ProjectID == "" {
		if task.SectionID != "" {
			section += " (name unavailable)"
		}
		return project, section
	}
	projects, err := listAllProjects(ctx)
	project = captureDestination(task.ProjectID, nil)
	if err != nil {
		project = task.ProjectID + " (name unavailable; lookup failed)"
	} else {
		for _, item := range projects {
			if item.ID == task.ProjectID && item.Name != "" {
				project = item.Name
				break
			}
		}
	}
	if task.SectionID != "" {
		// The returned project ID scopes sections even if project-name lookup failed.
		sections, err := detailSections(ctx, task.ProjectID)
		section = captureDestination(task.SectionID, nil)
		if err != nil {
			section = task.SectionID + " (name unavailable; lookup failed)"
		} else {
			for _, item := range sections {
				if item.ID == task.SectionID && item.ProjectID == task.ProjectID && item.Name != "" {
					section = item.Name
					break
				}
			}
		}
	}
	return project, section
}

func detailSections(ctx *Context, projectID string) ([]api.Section, error) {
	if cache := ctx.lookupCache; cache != nil {
		// A present global entry is complete, including a successful empty load.
		if sections, loaded := cache.sectionsByProject[""]; loaded {
			return cloneSlice(sections), nil
		}
	}
	return listAllSections(ctx, "id:"+projectID)
}

func writeDetailDue(task api.Task, field func(string, string)) {
	due := task.Due
	if due == nil {
		if task.DueReturned {
			field("Due", "No due date")
			field("Recurrence", "None")
		} else {
			field("Due", "Not returned")
			field("Recurrence", "Not returned")
		}
		return
	}
	value := due.Datetime
	if value == "" {
		value = due.Date
	}
	if due.Date != "" && due.Datetime != "" && !strings.HasPrefix(due.Datetime, due.Date) {
		field("Due date", due.Date)
		field("Due time", due.Datetime)
	} else {
		field("Due", returnedText(value))
	}
	if due.Timezone != nil && *due.Timezone != "" {
		field("Timezone", *due.Timezone)
	} else if strings.Contains(value, "T") {
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			field("Timezone", "Not returned (time shown as returned)")
		}
	}
	recurrence := "Not returned"
	expressionShown := false
	if due.IsRecurring != nil {
		recurrence = "None"
		if *due.IsRecurring {
			recurrence = "Yes"
			if due.String != "" {
				recurrence += " (" + due.String + ")"
				expressionShown = true
			}
		}
	}
	field("Recurrence", recurrence)
	if due.String != "" && !expressionShown && value == "" {
		field("Due expression", due.String)
	}
}

// Preserve hard line breaks and indentation, escaping controls within each line.
// Reuse the overview's terminal-cell convention without its title truncation.
func writeDetailText(b *strings.Builder, text, first, continuation string, width int, splitWords bool, ids ...string) {
	contentWidth := width - max(overviewTextWidth(first), overviewTextWidth(continuation))
	if contentWidth < 1 {
		contentWidth = 1
	}
	prefix := first
	for _, hardLine := range strings.Split(text, "\n") {
		for _, line := range wrapDetailLine(captureText(hardLine), contentWidth, splitWords, ids...) {
			b.WriteString(prefix)
			b.WriteString(line)
			b.WriteByte('\n')
			prefix = continuation
		}
	}
}

func wrapDetailLine(text string, width int, splitWords bool, ids ...string) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	line := ""
	runes := []rune(text)
	for start := 0; start < len(runes); {
		space := unicode.IsSpace(runes[start])
		end := start + 1
		for end < len(runes) && unicode.IsSpace(runes[end]) == space {
			end++
		}
		word := string(runes[start:end])
		// A normal word separator becomes the soft line break. Do not carry it
		// into the hanging indent; authored repeated spaces remain literal.
		if space && word == " " && end < len(runes) && line != "" && overviewTextWidth(line) >= width {
			lines = append(lines, line)
			line = ""
			start = end
			continue
		}
		// Whitespace-only fragments are authored indentation, including the
		// remainder of a repeated-space run that crossed a wrap boundary.
		if !space && strings.TrimSpace(line) != "" && overviewTextWidth(line)+overviewTextWidth(word) > width {
			if strings.HasSuffix(line, " ") && !strings.HasSuffix(line, "  ") {
				line = strings.TrimSuffix(line, " ")
			}
			lines = append(lines, line)
			line = ""
		}
		atomic := detailAtomicWord(word, ids)
		if !space && (!splitWords || atomic) {
			line += word
		} else {
			for _, r := range runes[start:end] {
				if line != "" && overviewTextWidth(line)+overviewRuneWidth(r) > width {
					lines = append(lines, line)
					line = ""
				}
				line += string(r)
			}
		}
		start = end
	}
	return append(lines, line)
}

func detailAtomicWord(word string, ids []string) bool {
	lower := strings.ToLower(word)
	if strings.Contains(lower, "https://") || strings.Contains(lower, "http://") {
		return true
	}
	value := strings.Trim(word, "\"'()[]{}.,;!?")
	for _, id := range ids {
		if id != "" && (value == id || value == "id:"+id) {
			return true
		}
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}
