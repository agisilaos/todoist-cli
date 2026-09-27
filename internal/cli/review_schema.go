package cli

func reviewMetadataSchema() map[string]any {
	return map[string]any{"type": "object", "required": []string{"version", "filter", "tasks"}, "properties": map[string]any{
		"version": map[string]any{"const": 1}, "filter": map[string]string{"type": "string"},
		"tasks": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"id", "content", "disposition", "snapshot", "actions"}, "properties": map[string]any{
			"id": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"},
			"disposition": map[string]any{"enum": []string{"keep", "change", "complete", "skip"}},
			"snapshot":    map[string]any{"type": "object", "description": "Frozen reviewed task fields, including id and updated_at when available."},
			"actions":     map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 0}, "description": "Zero-based indices into the unchanged plan actions."},
		}}},
	}}
}
func reviewReportSchema() map[string]any {
	return map[string]any{"type": "object", "required": []string{"version", "phase", "plan", "tasks", "counts"}, "properties": map[string]any{
		"version": map[string]any{"const": 1}, "phase": map[string]any{"enum": []string{"empty", "preview", "cancelled", "applied"}},
		"plan":      map[string]any{"type": "object", "description": "Agent plan plus review metadata; cancelled sessions may have unset dispositions."},
		"plan_path": map[string]string{"type": "string"}, "error": map[string]string{"type": "string"},
		"counts": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}},
		"tasks": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"id", "content", "disposition", "outcome", "actions"}, "properties": map[string]any{
			"id": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"}, "disposition": map[string]any{"enum": []string{"", "keep", "change", "complete", "skip"}},
			"outcome": map[string]any{"enum": []string{"kept", "skipped", "proposed", "applied", "failed", "partially_applied", "unattempted"}},
			"actions": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"index", "type", "outcome"}, "properties": map[string]any{
				"index": map[string]any{"type": "integer", "minimum": 0}, "type": map[string]string{"type": "string"},
				"outcome": map[string]any{"enum": []string{"proposed", "applied", "failed", "unattempted"}}, "replayed": map[string]string{"type": "boolean"}, "error": map[string]string{"type": "string"}, "remote_outcome_uncertain": map[string]string{"type": "boolean"},
			}}},
		}}},
	}}
}
