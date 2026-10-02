package cli

func closedTaskObject(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func taskWriteAckSchema() map[string]any {
	return closedTaskObject(map[string]any{
		"id":               map[string]any{"type": "string", "minLength": 1},
		"status":           map[string]any{"const": "accepted"},
		"operation":        map[string]any{"enum": []string{"task_add", "task_update", "task_reschedule"}},
		"result_available": map[string]any{"const": false},
	}, "status", "operation", "result_available")
}
func taskUnchangedSchema() map[string]any {
	return closedTaskObject(map[string]any{"id": map[string]any{"type": "string", "minLength": 1}, "status": map[string]any{"const": "unchanged"}, "operation": map[string]any{"enum": []string{"task_update", "task_move"}}}, "id", "status", "operation")
}
func taskPartialEditSchema() map[string]any {
	step := closedTaskObject(map[string]any{
		"operation":  map[string]any{"enum": []string{"clear_due", "update_fields"}},
		"outcome":    map[string]any{"enum": []string{"accepted", "rejected", "uncertain", "not_dispatched"}},
		"dispatched": map[string]any{"type": "boolean"}, "request_id": map[string]any{"type": "string"},
	}, "operation", "outcome", "dispatched")
	return closedTaskObject(map[string]any{"id": map[string]any{"type": "string"}, "status": map[string]any{"const": "partial"}, "operation": map[string]any{"const": "task_update"}, "steps": map[string]any{"type": "array", "items": step, "minItems": 2, "maxItems": 2}}, "id", "status", "operation", "steps")
}
func taskBatchSchema() map[string]any {
	target := closedTaskObject(map[string]any{"id": map[string]any{"type": "string"}, "outcome": map[string]any{"enum": []string{"accepted", "rejected", "uncertain", "unattempted"}}, "dispatched": map[string]any{"type": "boolean"}, "request_id": map[string]any{"type": "string"}, "error": map[string]any{"type": "string"}}, "id", "outcome", "dispatched")
	properties := map[string]any{"filter": map[string]any{"type": "string"}, "operation": map[string]any{"enum": []string{"task_move", "task_complete"}}, "targets": map[string]any{"type": "array", "items": target}, "preflight_error": map[string]any{"type": "string"}}
	for _, field := range []string{"count", "dispatched", "unchanged", "accepted", "rejected", "uncertain", "unattempted", "failed", "moved", "completed"} {
		properties[field] = map[string]any{"type": "integer", "minimum": 0}
	}
	return closedTaskObject(properties, "filter", "operation", "count", "targets", "dispatched", "unchanged", "accepted", "rejected", "uncertain", "unattempted", "failed")
}
func expandedTaskSchema(version int) map[string]any {
	task := legacyTaskItemSchema()
	if version == 2 {
		task = faithfulTaskItemSchema()
	}
	return closedTaskObject(map[string]any{"task": task, "children": map[string]any{"type": "array", "items": task}, "children_complete": map[string]any{"const": true}}, "task", "children", "children_complete")
}
func taskEditResultSchema(version int, ndjson bool) map[string]any {
	task := legacyTaskItemSchema()
	if version == 2 {
		task = faithfulTaskItemSchema()
	}
	resource := task
	if !ndjson {
		resource = map[string]any{"type": "array", "items": task}
	}
	return map[string]any{"oneOf": []any{resource, taskWriteAckSchema(), taskUnchangedSchema(), taskPartialEditSchema()}}
}
