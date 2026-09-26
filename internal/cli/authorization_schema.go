package cli

func authorizationReportSchema() map[string]any {
	scopes := map[string]any{"type": []string{"array", "null"}, "items": map[string]any{"type": "string"}}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"metadata_version": map[string]any{"type": []string{"integer", "null"}},
			"metadata_status":  map[string]any{"enum": []string{"valid", "legacy", "external", "missing", "invalid", "unsupported"}},
			"mode":             map[string]any{"enum": []any{"read-only", "read-write", "unknown", nil}},
			"origin":           map[string]any{"enum": []string{"oauth-pkce", "oauth-device", "manual", "unknown"}},
			"requested_scopes": scopes, "effective_scopes": scopes,
			"scope_evidence":          map[string]any{"enum": []string{"token-response", "oauth-request", "none"}},
			"write_capable":           map[string]any{"type": "boolean"},
			"write_capability_reason": map[string]any{"enum": []string{"read-only", "read-write", "unknown-compatibility", "no-credential", "invalid-metadata", "unsupported-metadata"}},
		},
		"required": []string{"metadata_version", "metadata_status", "mode", "origin", "requested_scopes", "effective_scopes", "scope_evidence", "write_capable", "write_capability_reason"},
	}
}

func authStatusSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"profile": map[string]any{"type": "string"}, "configured": map[string]any{"type": "boolean"}, "source": map[string]any{"type": "string"}, "authorization": authorizationReportSchema(),
	}, "required": []string{"profile", "configured", "source", "authorization"}}
}

func doctorReportSchema() map[string]any {
	integer := map[string]any{"type": "integer", "minimum": 0}
	return map[string]any{"type": "object", "properties": map[string]any{
		"checks": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
			"name": map[string]any{"type": "string"}, "status": map[string]any{"enum": []string{"ok", "warn", "fail"}}, "message": map[string]any{"type": "string"},
			"details": map[string]any{"type": "object", "properties": map[string]any{"authorization": authorizationReportSchema()}},
		}, "required": []string{"name", "status", "message"}}},
		"summary": map[string]any{"type": "object", "properties": map[string]any{"ok": integer, "warn": integer, "fail": integer, "total": integer}, "required": []string{"ok", "warn", "fail", "total"}},
	}, "required": []string{"checks", "summary"}}
}
