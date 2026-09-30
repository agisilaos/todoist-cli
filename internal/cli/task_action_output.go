package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func taskActionHuman(ctx *Context) bool {
	return ctx.Mode == output.ModeHuman && !ctx.Global.Quiet
}

func writeTaskCompletionResult(ctx *Context, id string, before *api.Task) error {
	if !taskActionHuman(ctx) {
		return writeSimpleResult(ctx, "completed", id)
	}
	var b strings.Builder
	b.WriteString("Completion accepted\n")
	writeTaskActionIdentity(&b, id, "Task before completion", before)
	if before != nil && before.Due != nil && before.Due.IsRecurring != nil && *before.Due.IsRecurring {
		b.WriteString("Recurring task; next due date not returned.\n")
	}
	fmt.Fprint(ctx.Stdout, b.String())
	return nil
}

func writeTaskMoveResult(ctx *Context, id string, before *api.Task, requested map[string]any, response []byte) error {
	if !taskActionHuman(ctx) {
		return writeSimpleResult(ctx, "moved", id)
	}
	result := decodeTaskMoveResult(id, response)
	var b strings.Builder
	b.WriteString("Move accepted\n")
	if result.content.known && result.content.value != "" {
		fmt.Fprintf(&b, "Task: %s\n", captureText(result.content.value))
		fmt.Fprintf(&b, "ID: %s\n", captureText(id))
	} else {
		writeTaskActionIdentity(&b, id, "Task before move", before)
	}
	unavailable := !result.project.known
	for _, destination := range []struct {
		key, label string
		field      taskMoveField
	}{
		{"project_id", "Project", result.project},
		{"section_id", "Section", result.section},
		{"parent_id", "Parent task", result.parent},
	} {
		request, _ := requested[destination.key].(string)
		if destination.field.known {
			// Omit unrequested empty section/parent context for a compact result.
			if destination.field.value == "" && request == "" {
				continue
			}
			value := "None"
			if destination.field.value != "" {
				value = taskActionDestination(ctx, destination.key, destination.field.value, result.project.value)
			}
			fmt.Fprintf(&b, "%s: %s\n", destination.label, captureText(value))
		} else if request != "" {
			value := taskActionDestination(ctx, destination.key, request, "")
			fmt.Fprintf(&b, "Requested %s: %s\n", strings.ToLower(destination.label), captureText(value))
			unavailable = true
		}
	}
	if unavailable {
		b.WriteString("Destination details unavailable.\n")
	}
	fmt.Fprint(ctx.Stdout, b.String())
	return nil
}

func writeTaskActionPreview(ctx *Context, action, id string, before *api.Task, requested map[string]any) error {
	if !taskActionHuman(ctx) {
		return writeDryRun(ctx, "task "+action, requested)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Dry run: would %s task; no task changed.\n", action)
	writeTaskActionIdentity(&b, id, "Task before "+action, before)
	if action == "complete" && before != nil && before.Due != nil && before.Due.IsRecurring != nil && *before.Due.IsRecurring {
		b.WriteString("Recurring task before preview.\n")
	}
	for _, destination := range []struct{ key, label string }{
		{"project_id", "project"}, {"section_id", "section"}, {"parent_id", "parent task"},
	} {
		if value, _ := requested[destination.key].(string); value != "" {
			fmt.Fprintf(&b, "Requested %s: %s\n", destination.label, captureText(taskActionDestination(ctx, destination.key, value, "")))
		}
	}
	fmt.Fprintf(&b, "Authorization: %s\n", currentAuthorization(ctx).Summary())
	fmt.Fprint(ctx.Stdout, b.String())
	return nil
}

func writeTaskActionIdentity(b *strings.Builder, id, label string, before *api.Task) {
	if before != nil && before.Content != "" {
		fmt.Fprintf(b, "%s: %s\n", label, captureText(before.Content))
	}
	fmt.Fprintf(b, "ID: %s\n", captureText(id))
}

// Consult only already-loaded context. Rendering must never dispatch enrichment
// reads or substitute the caller's input name for a returned destination ID.
func taskActionDestination(ctx *Context, key, id, project string) string {
	if cache := ctx.lookupCache; cache != nil {
		switch key {
		case "project_id":
			for _, p := range cache.projects {
				if p.ID == id && p.Name != "" {
					return p.Name
				}
			}
		case "section_id":
			for _, sections := range cache.sectionsByProject {
				for _, s := range sections {
					if s.ID == id && s.Name != "" && (project == "" || s.ProjectID == project) {
						return s.Name
					}
				}
			}
		case "parent_id":
			for _, task := range cache.activeTasks {
				if task.ID == id && task.Content != "" {
					return task.Content + " (ID " + id + ")"
				}
			}
		}
	}
	return id + " (name unavailable)"
}

type taskMoveField struct {
	value string
	known bool
}

type taskMoveResult struct {
	content, project, section, parent taskMoveField
}

// Each optional fact is independent. Invalid facts stay unknown; an explicit
// mismatched identity makes the entire advisory response unusable.
func decodeTaskMoveResult(id string, data []byte) taskMoveResult {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return taskMoveResult{}
	}
	if raw, present := fields["id"]; present {
		var returnedID string
		if err := json.Unmarshal(raw, &returnedID); err != nil || returnedID != id {
			return taskMoveResult{}
		}
	}
	return taskMoveResult{
		content: taskMoveResponseField(fields["content"], false),
		project: taskMoveResponseField(fields["project_id"], false),
		section: taskMoveResponseField(fields["section_id"], true),
		parent:  taskMoveResponseField(fields["parent_id"], true),
	}
}

func taskMoveResponseField(raw json.RawMessage, allowNone bool) taskMoveField {
	if len(raw) == 0 {
		return taskMoveField{}
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return taskMoveField{known: allowNone}
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" && !allowNone {
		return taskMoveField{}
	}
	return taskMoveField{value: value, known: true}
}
