package cli

import (
	"flag"
	"fmt"
	"io"
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
		Description: "Response shape for task list",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"data": map[string]any{
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
							"due":          map[string]any{"type": "object"},
							"checked":      map[string]string{"type": "boolean"},
							"completed_at": map[string]string{"type": "string"},
						},
						"required": []string{"id", "content", "project_id", "section_id", "labels", "priority"},
					},
				},
				"meta": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"request_id":  map[string]string{"type": "string"},
						"count":       map[string]string{"type": "integer"},
						"next_cursor": map[string]string{"type": "string"},
					},
					"required": []string{"count"},
				},
			},
			"required": []string{"data", "meta"},
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
}

func schemaCommand(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var name string
	var help bool
	fs.StringVar(&name, "name", "", "Schema name")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSchemaHelp(ctx.Stdout)
		return nil
	}
	if ctx.Mode != output.ModeJSON {
		// Default to JSON-friendly output even in human mode for clarity.
		fmt.Fprintln(ctx.Stderr, "tip: use --json for machine-readable schema")
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
