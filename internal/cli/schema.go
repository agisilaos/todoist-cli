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
	{Name: "task_write_ack", Description: "Accepted add/update/reschedule without optional task resource data", Schema: taskWriteAckSchema()},
	{Name: "task_unchanged", Description: "Known no-op update/move; zero writes dispatched", Schema: taskUnchangedSchema()},
	{Name: "task_partial_edit", Description: "Two-step edit with accepted due clear and incomplete remaining edit; nonzero exit", Schema: taskPartialEditSchema()},
	{Name: "task_batch", Description: "Filtered completion/move accounting; nonzero exit unless every target accepted", Schema: taskBatchSchema()},
	{Name: "task_expanded_view", Description: "Legacy parent/direct active children; all pages buffered", Schema: expandedTaskSchema(1)},
	{Name: "task_expanded_view_v2", Description: "Faithful parent/direct active children; separate envelope from task_item_v2", Schema: expandedTaskSchema(2)},
	{Name: "task_write_result", Description: "JSON add/update/reschedule resource or explicit acknowledgement/partial result", Schema: taskEditResultSchema(1, false)},
	{Name: "task_write_result_v2", Description: "JSON v2 add/update/reschedule resource or explicit acknowledgement/partial result", Schema: taskEditResultSchema(2, false)},
	{Name: "task_write_record", Description: "NDJSON add/update/reschedule task or acknowledgement/partial result", Schema: taskEditResultSchema(1, true)},
	{Name: "task_write_record_v2", Description: "NDJSON v2 add/update/reschedule task or acknowledgement/partial result", Schema: taskEditResultSchema(2, true)},
	{Name: "skill_result", Description: "Install, update, and uninstall result for a bundled agent skill", Schema: skillResultSchema()},
	{Name: "skill_list", Description: "Agent skill installation inventory; inspect each item's status", Schema: skillListSchema()},
	{Name: "review_report", Description: "Final report for review and application of review plans", Schema: reviewReportSchema()},
	{Name: "authorization", Description: "Safe authorization report for the active credential", Schema: authorizationReportSchema()},
	{Name: "auth_status", Description: "Offline credential presence and authorization from auth status", Schema: authStatusSchema()},
	{Name: "profile_list", Description: "Metadata-only credential profile inventory (JSON and one-record NDJSON)", Schema: profileListSchema()},
	{Name: "profile_current", Description: "Selected profile and effective credential source, including environment overrides", Schema: profileCurrentSchema()},
	{Name: "profile_use", Description: "Saved user default and effective selection after profile use", Schema: profileUseSchema()},
	{Name: "profile_remove", Description: "Completed profile removal with selection retained", Schema: profileRemoveSchema()},
	{Name: "doctor", Description: "Doctor diagnostics including safe credential authorization", Schema: doctorReportSchema()},
	{
		Name:        "ids_only",
		Description: "Wire-format descriptor for --ids-only (not a JSON Schema)",
		Schema: map[string]any{
			"kind":        "wire_format",
			"encoding":    "raw_id_lines",
			"stdout":      "One opaque ID followed by LF per result; no quoting, headings, metadata, or empty-state text. Empty results emit zero bytes.",
			"ordering":    "Preserves the command's result order and duplicates.",
			"validation":  "All fetched IDs are validated before output; empty IDs or IDs containing whitespace/control characters fail with exit 1.",
			"commands":    []string{"task list", "project list", "project collaborators", "section list", "label list", "comment list", "filter list", "workspace list", "reminder list", "notification list", "activity", "completed", "today", "upcoming", "inbox", "filter show"},
			"aliases":     "Existing ls aliases are supported.",
			"identity":    "Collaborators emit user IDs; activity emits event IDs, not object IDs.",
			"flags":       "Global placement before or after the command; parsing stops at --. Mutually exclusive with --json, --plain, and --ndjson.",
			"pagination":  "Existing fetching defaults and --all behavior; continuation notices with the next cursor or notification offset go only to stderr.",
			"errors":      map[string]any{"schema": "error", "stream": "stderr", "usage_exit": 2, "runtime_exits": []int{1, 3, 4, 5}, "quiet_json": "Compact JSON errors"},
			"unsupported": "Other commands are rejected before side effects, including mutations, single-object views, and view URL.",
			"exceptions":  "Version takes precedence over output conflicts; conflicts precede help. Root and command help remain available.",
		},
	},
	{Name: "task_item", Description: "Legacy task object for task view JSON and task NDJSON records", Schema: legacyTaskItemSchema()},
	{Name: "task_list", Description: "Legacy task JSON array", Schema: map[string]any{"type": "array", "items": legacyTaskItemSchema()}},
	{Name: "task_item_ndjson", Description: "Legacy task NDJSON record (same task object as JSON)", Schema: legacyTaskItemSchema()},
	{Name: "task_item_v2", Description: "Faithful task object for --task-output-version 2 JSON view and NDJSON records", Schema: faithfulTaskItemSchema()},
	{Name: "task_list_v2", Description: "Faithful task array for --task-output-version 2 JSON lists and returned add/update tasks", Schema: map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "array", "items": faithfulTaskItemSchema()}},
	{
		Name:        "error",
		Description: "Error envelope when --json is set",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"error":   map[string]string{"type": "string"},
				"code":    map[string]any{"type": "string", "description": "Stable authorization, profile, storage, OAuth, or SKILL_* lifecycle error code; see the specification"},
				"details": map[string]any{"type": "object", "properties": map[string]any{"profile": map[string]any{"type": "string"}, "source": map[string]any{"type": "string"}, "authorization": authorizationReportSchema(), "operation": map[string]any{"type": "string"}, "committed": map[string]any{"type": "boolean"}, "retry_command": map[string]any{"type": "string"}, "repair_command": map[string]any{"type": "string"}}},
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
				"review":        reviewMetadataSchema(),
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
							"reason":        map[string]string{"type": "string"},
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
				"authorization": authorizationReportSchema(),
				"plan":          map[string]any{"$ref": "#/plan"},
				"dry_run":       map[string]string{"type": "boolean"},
				"action_count":  map[string]string{"type": "integer"},
				"summary": map[string]any{
					"type": "object",
				},
			},
			"required": []string{"plan", "dry_run", "authorization"},
		},
	},
	{
		Name:        "planner_request",
		Description: "Planner input shape (instruction + context)",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"authorization": authorizationReportSchema(),
				"instruction":   map[string]string{"type": "string"},
				"profile":       map[string]string{"type": "string"},
				"now":           map[string]string{"type": "string"},
				"context": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projects":        map[string]any{"type": "array"},
						"sections":        map[string]any{"type": "array"},
						"labels":          map[string]any{"type": "array"},
						"active_tasks":    map[string]any{"type": "array"},
						"completed_tasks": map[string]any{"type": "array"},
					},
				},
			},
			"required": []string{"instruction", "profile", "now", "context", "authorization"},
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
	return output.WriteJSON(ctx.Stdout, list)
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
