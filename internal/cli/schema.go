package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/output"
)

type schemaDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Schema      interface{} `json:"schema"`
}

var schemas = []schemaDef{
	{
		Name:        "task_list",
		Description: "JSON response shape for `todoist task list --json` (array of tasks)",
		Schema: map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":           map[string]string{"type": "string"},
					"content":      map[string]string{"type": "string"},
					"description":  map[string]string{"type": "string"},
					"project_id":   map[string]string{"type": "string"},
					"section_id":   map[string]string{"type": "string"},
					"parent_id":    map[string]string{"type": "string"},
					"labels":       map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
					"priority":     map[string]string{"type": "integer"},
					"due":          map[string]any{"type": []any{"object", "null"}},
					"checked":      map[string]string{"type": "boolean"},
					"added_at":     map[string]string{"type": "string"},
					"completed_at": map[string]string{"type": "string"},
					"updated_at":   map[string]string{"type": "string"},
					"note_count":   map[string]string{"type": "integer"},
				},
				"required": []string{"id", "content", "project_id", "section_id", "labels", "priority", "checked"},
			},
		},
	},
	{
		Name:        "task_item_ndjson",
		Description: "One line item shape for `todoist task list --ndjson`",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":         map[string]string{"type": "string"},
				"content":    map[string]string{"type": "string"},
				"project_id": map[string]string{"type": "string"},
				"section_id": map[string]string{"type": "string"},
				"labels":     map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
				"priority":   map[string]string{"type": "integer"},
				"checked":    map[string]string{"type": "boolean"},
			},
			"required": []string{"id", "content", "project_id", "section_id", "labels", "priority", "checked"},
		},
	},
	{
		Name:        "error",
		Description: "Error envelope when --json is set",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"error": map[string]string{"type": "string"},
				"meta": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"request_id": map[string]string{"type": "string"},
					},
				},
			},
			"required": []string{"error", "meta"},
		},
	},
	{
		Name:        "plan",
		Description: "Agent plan produced by planner and persisted on disk",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"version":       map[string]string{"type": "integer"},
				"instruction":   map[string]string{"type": "string"},
				"created_at":    map[string]string{"type": "string"},
				"confirm_token": map[string]string{"type": "string"},
				"applied_at":    map[string]string{"type": "string"},
				"summary": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tasks":    map[string]string{"type": "integer"},
						"projects": map[string]string{"type": "integer"},
						"sections": map[string]string{"type": "integer"},
						"labels":   map[string]string{"type": "integer"},
						"comments": map[string]string{"type": "integer"},
					},
					"required": []string{"tasks", "projects", "sections", "labels", "comments"},
				},
				"actions": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"type":          map[string]string{"type": "string"},
							"task_id":       map[string]string{"type": "string"},
							"project_id":    map[string]string{"type": "string"},
							"section_id":    map[string]string{"type": "string"},
							"label_id":      map[string]string{"type": "string"},
							"comment_id":    map[string]string{"type": "string"},
							"content":       map[string]string{"type": "string"},
							"description":   map[string]string{"type": "string"},
							"name":          map[string]string{"type": "string"},
							"labels":        map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
							"project":       map[string]string{"type": "string"},
							"section":       map[string]string{"type": "string"},
							"parent":        map[string]string{"type": "string"},
							"priority":      map[string]string{"type": "integer"},
							"due":           map[string]string{"type": "string"},
							"due_date":      map[string]string{"type": "string"},
							"due_datetime":  map[string]string{"type": "string"},
							"due_lang":      map[string]string{"type": "string"},
							"duration":      map[string]string{"type": "integer"},
							"duration_unit": map[string]string{"type": "string"},
							"deadline_date": map[string]string{"type": "string"},
							"assignee_id":   map[string]string{"type": "string"},
							"color":         map[string]string{"type": "string"},
							"order":         map[string]string{"type": "integer"},
							"is_favorite":   map[string]string{"type": "boolean"},
							"idempotent":    map[string]string{"type": "boolean"},
						},
						"required": []string{"type"},
					},
				},
			},
			"required": []string{"version", "instruction", "confirm_token", "actions", "summary"},
		},
	},
	{
		Name:        "plan_preview",
		Description: "Agent plan dry-run output shape",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan":         map[string]any{"$ref": "#/plan"},
				"dry_run":      map[string]string{"type": "boolean"},
				"action_count": map[string]string{"type": "integer"},
				"summary": map[string]any{
					"type": "object",
				},
			},
			"required": []string{"plan", "dry_run"},
		},
	},
	{
		Name:        "planner_request",
		Description: "Planner input shape (instruction + context)",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"instruction": map[string]string{"type": "string"},
				"profile":     map[string]string{"type": "string"},
				"now":         map[string]string{"type": "string"},
				"context": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projects":        map[string]any{"type": "array"},
						"sections":        map[string]any{"type": "array"},
						"labels":          map[string]any{"type": "array"},
						"completed_tasks": map[string]any{"type": "array"},
					},
				},
			},
			"required": []string{"instruction", "profile", "now", "context"},
		},
	},
}

func schemaCommand(ctx *Context, args []string) error {
	fs := newFlagSet("schema")
	var name string
	var help bool
	fs.StringVar(&name, "name", "", "Schema name")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSchemaHelp(ctx.Stdout)
		return nil
	}
	list := schemas
	if name != "" {
		filtered := make([]schemaDef, 0, 1)
		for _, s := range schemas {
			if s.Name == name {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown schema: %s", name)}
		}
		list = filtered
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return output.WriteJSON(ctx.Stdout, list, output.Meta{})
}

func printSchemaHelp(out interface{ Write([]byte) (int, error) }) {
	var names []string
	for _, s := range schemas {
		names = append(names, s.Name)
	}
	fmt.Fprintf(out, `Usage:
  todoist schema [--name <schema>] [--json]

Schemas:
  %s

Examples:
  todoist schema --json
  todoist schema --name task_list --json
`, strings.Join(names, ", "))
}
