package cli

func skillLocationProperties() map[string]any {
	return map[string]any{
		"target": map[string]any{"type": "string", "enum": []string{"codex", "claude-code"}},
		"scope":  map[string]any{"type": "string", "enum": []string{"local", "global"}},
		"path":   map[string]any{"type": "string", "description": "Absolute destination, canonicalized when safely resolvable"},
	}
}

func skillStringArray() map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
}

func skillResultSchema() map[string]any {
	props := skillLocationProperties()
	props["operation"] = map[string]any{"type": "string", "enum": []string{"install", "update", "uninstall"}}
	props["status"] = map[string]any{"type": "string", "enum": []string{"installed", "updated", "uninstalled", "unchanged"}}
	props["files"], props["retained"] = skillStringArray(), skillStringArray()
	props["package_digest"] = map[string]any{"type": "string"}
	props["version"] = map[string]any{"type": "string"}
	props["backup_path"] = map[string]any{"type": "string"}
	return map[string]any{"type": "object", "properties": props, "required": []string{"operation", "status", "target", "scope", "path", "files", "retained"}}
}

func skillListSchema() map[string]any {
	props := skillLocationProperties()
	props["status"] = map[string]any{"type": "string", "enum": []string{"absent", "installed", "outdated", "modified", "conflict", "error"}}
	props["installed"] = map[string]any{"type": "boolean"}
	props["modified"], props["missing"] = skillStringArray(), skillStringArray()
	for _, key := range []string{"available_digest", "package_digest", "version", "code", "error"} {
		props[key] = map[string]any{"type": "string"}
	}
	return map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": props, "required": []string{"target", "scope", "path", "status", "installed", "available_digest", "modified", "missing"}}}
}
