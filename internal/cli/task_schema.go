package cli

import "strings"

func legacyTaskItemSchema() map[string]any {
	properties := make(map[string]any)
	for _, name := range strings.Fields("id content description project_id section_id parent_id added_at completed_at updated_at") {
		properties[name] = map[string]any{"type": "string"}
	}
	for _, name := range []string{"priority", "note_count"} {
		properties[name] = map[string]any{"type": "integer"}
	}
	properties["checked"] = map[string]any{"type": "boolean"}
	properties["labels"] = map[string]any{"type": []string{"array", "null"}, "items": map[string]any{"type": "string"}}
	properties["due"] = map[string]any{
		"type": []string{"object", "null"}, "properties": map[string]any{
			"date": map[string]any{"type": "string"}, "datetime": map[string]any{"type": "string"}, "string": map[string]any{"type": "string"},
		},
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"required":    []string{"id", "content", "project_id", "section_id", "labels", "priority", "checked"},
		"description": "Legacy task resource; missing response values can become Go defaults. Select --task-output-version 2 for returned presence.",
	}
}

func nullableTaskProperties(text, integers, booleans string) map[string]any {
	properties := make(map[string]any)
	for kind, names := range map[string]string{"string": text, "integer": integers, "boolean": booleans} {
		for _, name := range strings.Fields(names) {
			properties[name] = map[string]any{"type": []string{kind, "null"}}
		}
	}
	return properties
}

func nullableTaskObject(properties map[string]any) map[string]any {
	return map[string]any{"type": []string{"object", "null"}, "properties": properties, "additionalProperties": false}
}

func faithfulTaskItemSchema() map[string]any {
	properties := nullableTaskProperties(
		"user_id id project_id section_id parent_id added_by_uid assigned_by_uid responsible_uid added_at completed_at completed_by_uid updated_at order_key content description",
		"priority child_order note_count day_order completed_count postponed_count",
		"is_collapsed checked is_deleted is_uncompletable",
	)
	properties["labels"] = map[string]any{"type": []string{"array", "null"}, "items": map[string]any{"type": "string"}}
	properties["due"] = nullableTaskObject(nullableTaskProperties("date datetime string lang timezone", "", "is_recurring"))
	properties["deadline"] = nullableTaskObject(nullableTaskProperties("date lang", "", ""))
	properties["duration"] = nullableTaskObject(nullableTaskProperties("unit", "amount", ""))
	properties["reference_item"] = map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"is_reference": map[string]any{"type": "boolean"},
			"source":       map[string]any{"const": "content_prefix"},
		},
		"required":    []string{"is_reference", "source"},
		"description": "Derived from the exact returned content prefix '* '; absent when content is unavailable. Independent of any returned is_uncompletable flag or completion state.",
	}
	properties["responsible_uid"].(map[string]any)["description"] = "Returned assignee ID; null means unassigned. Mutation assignee_id is a different request name."
	properties["child_order"].(map[string]any)["description"] = "Returned sibling position, including zero."
	properties["order_key"].(map[string]any)["description"] = "Lexicographic ordering among siblings in the same project, section and parent; null means not migrated."
	properties["note_count"].(map[string]any)["description"] = "Deprecated upstream field, currently always zero; not a reliable comment count."
	properties["is_uncompletable"].(map[string]any)["description"] = "Advisory explicit response flag not in the current official API specification; never synthesized from content."
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object", "properties": properties, "additionalProperties": false,
		"description": "CLI task-resource v2 over Todoist API v1. All returned API facts are optional: absent stays omitted, explicit null and typed false/zero/empty values remain exact. Malformed facts are omitted with stderr diagnostics. No defaults or input-based inference.",
	}
}
