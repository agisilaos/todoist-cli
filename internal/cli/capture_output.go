package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func writeCaptureReceipt(ctx *Context, task api.Task) error {
	if ctx.Mode != output.ModeHuman || ctx.Global.Quiet {
		return writeTaskList(ctx, []api.Task{task}, "", false)
	}
	var b strings.Builder
	if task.ID == "" {
		b.WriteString("Creation response received; task ID not returned. Verify in Todoist before retrying.\n")
	} else {
		b.WriteString("Created task\n")
	}
	field := func(name, value string) { fmt.Fprintf(&b, "%s: %s\n", name, captureText(value)) }
	field("Content", returnedText(task.Content))
	project := "Not returned"
	if task.ProjectID != "" {
		project = captureDestination(task.ProjectID, projectNameMap(ctx))
	}
	field("Project", project)
	if task.SectionID != "" {
		field("Section", captureDestination(task.SectionID, sectionNameMap(ctx)))
	}
	due, recurrence := "Not returned", "Not returned"
	if task.DueReturned && task.Due == nil {
		due, recurrence = "No due date", "None"
	}
	if task.Due != nil {
		due = task.Due.Datetime
		if due == "" {
			due = task.Due.Date
		}
		due = returnedText(due)
		if task.Due.IsRecurring != nil {
			recurrence = "None"
			if *task.Due.IsRecurring {
				recurrence = "Yes"
				if task.Due.String != "" {
					recurrence += " (" + task.Due.String + ")"
				}
			}
		}
	}
	field("Due", due)
	if task.Due != nil {
		if task.Due.Timezone != nil && *task.Due.Timezone != "" {
			field("Timezone", *task.Due.Timezone)
		} else if strings.Contains(due, "T") {
			if _, err := time.Parse(time.RFC3339Nano, due); err != nil {
				field("Timezone", "Not returned (time shown as returned)")
			}
		}
	}
	field("Recurrence", recurrence)
	priority := "Not returned"
	if task.Priority >= 1 && task.Priority <= 4 {
		priority = strconv.Itoa(5 - task.Priority)
	}
	field("Priority", priority)
	labels := "Not returned"
	if task.Labels != nil {
		labels = "None"
		if len(task.Labels) > 0 {
			// Quote each label to distinguish a comma in a name from a separator.
			quoted := make([]string, len(task.Labels))
			for i, label := range task.Labels {
				quoted[i] = strconv.Quote(label)
			}
			labels = strings.Join(quoted, ", ")
		}
	}
	// Labels have already been escaped by strconv.Quote.
	fmt.Fprintf(&b, "Labels: %s\n", labels)
	field("ID", returnedText(task.ID))
	if task.ID != "" && strings.IndexFunc(task.ID, captureControl) < 0 {
		ref := "id:" + task.ID
		if strings.IndexFunc(ref, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(":_-", r))
		}) >= 0 {
			ref = "'" + strings.ReplaceAll(ref, "'", "'\"'\"'") + "'"
		}
		fmt.Fprintf(&b, "View: todoist task view %s\n", ref)
	}
	b.WriteString("Edit: todoist task update --help; change destination: todoist task move --help\n")
	_, err := fmt.Fprint(ctx.Stdout, b.String())
	return err
}

func captureDestination(id string, names map[string]string) string {
	if name := names[id]; name != "" {
		return name
	}
	return id + " (name unavailable)"
}

func returnedText(value string) string {
	if value == "" {
		return "Not returned"
	}
	return value
}

func captureControl(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

// Keep full user text without permitting terminal controls or forged receipt lines.
func captureText(value string) string {
	var b strings.Builder
	for _, r := range value {
		if captureControl(r) {
			quoted := strconv.QuoteRune(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeCapturePreview(ctx *Context, action string, payload map[string]any) error {
	if ctx.Mode != output.ModeHuman || ctx.Global.Quiet {
		return writeDryRun(ctx, action, payload)
	}
	var b strings.Builder
	if text, ok := payload["text"].(string); ok {
		b.WriteString("Would submit for Todoist to interpret; no task created.\n")
		fmt.Fprintf(&b, "Text: %s\n", captureText(text))
		b.WriteString("Project, due date, recurrence, priority, and labels are not yet confirmed.\n")
	} else {
		b.WriteString("Would create a task with these submitted fields; no task created.\n")
		if action == "inbox add" {
			if _, ok := payload["project_id"]; !ok {
				b.WriteString("Project: Inbox (requested)\n")
			}
		}
		keys := make([]string, 0, len(payload))
		for key := range payload {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fieldNames := map[string]string{
			"content": "Content", "description": "Description", "project_id": "Project ID",
			"section_id": "Section ID", "parent_id": "Parent task ID", "labels": "Labels",
			"due_string": "Due expression", "due_date": "Due date", "due_datetime": "Due time",
			"due_lang": "Due language", "duration": "Duration", "duration_unit": "Duration unit",
			"deadline_date": "Deadline", "assignee_id": "Assignee ID",
		}
		for _, key := range keys {
			value := payload[key]
			if key == "priority" {
				if p, ok := value.(int); ok && p >= 1 && p <= 4 {
					fmt.Fprintf(&b, "Priority: %d\n", 5-p)
					continue
				}
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				return err
			}
			name := fieldNames[key]
			if name == "" {
				name = key
			}
			fmt.Fprintf(&b, "%s: %s\n", name, captureText(string(encoded)))
		}
		if _, ok := payload["due_string"]; ok {
			b.WriteString("Todoist will interpret the due expression when submitted.\n")
		}
	}
	fmt.Fprintf(&b, "Authorization: %s\n", currentAuthorization(ctx).Summary())
	_, err := fmt.Fprint(ctx.Stdout, b.String())
	return err
}
