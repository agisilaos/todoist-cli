package cli

func profileErrorSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"code": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"},
	}, "required": []string{"code", "message"}}
}

func profileSelectionSchema() map[string]any {
	return map[string]any{"enum": []string{"flag", "environment", "project", "user", "fallback"}}
}

func profileMetadataProperties() map[string]any {
	return map[string]any{
		"configured": map[string]any{"type": "boolean"}, "backend": map[string]any{"type": "string"},
		"accessibility": map[string]any{"enum": []string{"unchecked"}},
		"recovery":      map[string]any{"type": "string"},
		"authorization": authorizationReportSchema(), "error": profileErrorSchema(),
	}
}

func profileListSchema() map[string]any {
	row := profileMetadataProperties()
	row["profile"] = map[string]any{"type": "string"}
	row["selected"] = map[string]any{"type": "boolean"}
	return map[string]any{"type": "object", "properties": map[string]any{
		"profiles": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": row,
			"required": []string{"profile", "selected", "configured", "backend", "accessibility", "recovery", "authorization"}}},
		"selected_profile": map[string]any{"type": "string"}, "selection_source": profileSelectionSchema(),
		"environment_token_active": map[string]any{"type": "boolean"},
	}, "required": []string{"profiles", "selected_profile", "selection_source", "environment_token_active"}}
}

func profileCurrentSchema() map[string]any {
	properties := profileMetadataProperties()
	properties["selected_profile"] = map[string]any{"type": "string"}
	properties["selection_source"] = profileSelectionSchema()
	properties["profile_active"] = map[string]any{"type": "boolean"}
	properties["source"] = map[string]any{"enum": []string{"", "env", "credentials"}}
	return map[string]any{"type": "object", "properties": properties,
		"required": []string{"selected_profile", "selection_source", "profile_active", "configured", "source", "backend", "accessibility", "recovery", "authorization"}}
}

func profileUseSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"profile": map[string]any{"type": "string"}, "saved": map[string]any{"const": true},
		"config_path": map[string]any{"type": "string"}, "selected_profile": map[string]any{"type": "string"},
		"selection_source": profileSelectionSchema(), "shadowed": map[string]any{"type": "boolean"},
		"environment_token_active": map[string]any{"type": "boolean"},
	}, "required": []string{"profile", "saved", "config_path", "selected_profile", "selection_source", "shadowed", "environment_token_active"}}
}

func profileRemoveSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"profile": map[string]any{"type": "string"}, "removed": map[string]any{"const": true},
		"selected_profile": map[string]any{"type": "string"}, "selection_source": profileSelectionSchema(),
		"selection_retained": map[string]any{"const": true}, "environment_token_active": map[string]any{"type": "boolean"},
	}, "required": []string{"profile", "removed", "selected_profile", "selection_source", "selection_retained", "environment_token_active"}}
}
