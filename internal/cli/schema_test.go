package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestTaskListSchemaIsArrayContract(t *testing.T) {
	var taskListSchema map[string]any
	for _, s := range schemas {
		if s.Name == "task_list" {
			var ok bool
			taskListSchema, ok = s.Schema.(map[string]any)
			if !ok {
				t.Fatalf("task_list schema has unexpected type: %T", s.Schema)
			}
			break
		}
	}
	if taskListSchema == nil {
		t.Fatalf("task_list schema not found")
	}
	if taskListSchema["type"] != "array" {
		t.Fatalf("task_list schema type=%v, want array", taskListSchema["type"])
	}
	items, ok := taskListSchema["items"].(map[string]any)
	if !ok {
		t.Fatalf("task_list items has unexpected type: %T", taskListSchema["items"])
	}
	if items["type"] != "object" {
		t.Fatalf("task_list items type=%v, want object", items["type"])
	}
	required, ok := items["required"].([]string)
	if !ok {
		t.Fatalf("task_list items required has unexpected type: %T", items["required"])
	}
	if !containsString(required, "id") || !containsString(required, "content") || !containsString(required, "priority") {
		t.Fatalf("task_list required fields missing: %#v", required)
	}
}

func TestSchemaCommandNameFilter(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
	}
	if err := schemaCommand(ctx, []string{"--name", "task_item_ndjson"}); err != nil {
		t.Fatalf("schema command: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(ctx.Stdout.(*bytes.Buffer).Bytes(), &got); err != nil {
		t.Fatalf("decode schema output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(got))
	}
	if got[0]["name"] != "task_item_ndjson" {
		t.Fatalf("unexpected schema name: %v", got[0]["name"])
	}
	schema, ok := got[0]["schema"].(map[string]any)
	if !ok {
		t.Fatalf("schema payload has unexpected type: %T", got[0]["schema"])
	}
	if schema["type"] != "object" {
		t.Fatalf("task_item_ndjson type=%v, want object", schema["type"])
	}
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}

func TestTaskV2SchemaPresenceContract(t *testing.T) {
	item := faithfulTaskItemSchema()
	if _, present := item["required"]; present {
		t.Fatal("v2 must not require absent response facts")
	}
	if item["additionalProperties"] != false {
		t.Fatal("v2 allowlist must be fixed")
	}
	properties := item["properties"].(map[string]any)
	if len(properties) != 30 {
		t.Fatalf("expected 28 API fields, advisory flag, derived classification; got %d", len(properties))
	}
	for name, schema := range properties {
		if name == "reference_item" {
			continue
		}
		property := schema.(map[string]any)
		if !containsString(property["type"].([]string), "null") {
			t.Errorf("explicit null excluded for %s", name)
		}
		if nested, ok := property["properties"].(map[string]any); ok {
			if _, required := property["required"]; required || property["additionalProperties"] != false {
				t.Errorf("nested presence/allowlist contract wrong for %s", name)
			}
			for member, value := range nested {
				if !containsString(value.(map[string]any)["type"].([]string), "null") {
					t.Errorf("nested null excluded: %s.%s", name, member)
				}
			}
		}
	}
	for _, name := range []string{"task_item", "task_item_v2", "task_list_v2"} {
		var out bytes.Buffer
		ctx := &Context{Stdout: &out, Stderr: &bytes.Buffer{}, Mode: output.ModeJSON}
		if err := schemaCommand(ctx, []string{"--name", name}); err != nil || !json.Valid(out.Bytes()) {
			t.Fatalf("schema %s undiscoverable/invalid: %v %s", name, err, out.String())
		}
	}
}
