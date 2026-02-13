package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type agentPolicy struct {
	AllowActionTypes      []string `json:"allow_action_types"`
	DenyActionTypes       []string `json:"deny_action_types"`
	MaxDestructiveActions int      `json:"max_destructive_actions"`
}

func loadAgentPolicy(ctx *Context, path string) (*agentPolicy, error) {
	if path == "" && ctx != nil && ctx.ConfigPath != "" {
		defaultPath := filepath.Join(filepath.Dir(ctx.ConfigPath), "agent_policy.json")
		if _, err := os.Stat(defaultPath); err == nil {
			path = defaultPath
		}
	}
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var policy agentPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	return &policy, nil
}

func enforceAgentPolicy(plan Plan, policy *agentPolicy) error {
	if policy == nil {
		return nil
	}
	allow := map[string]struct{}{}
	for _, a := range policy.AllowActionTypes {
		allow[a] = struct{}{}
	}
	deny := map[string]struct{}{}
	for _, a := range policy.DenyActionTypes {
		deny[a] = struct{}{}
	}
	destructive := 0
	for _, action := range plan.Actions {
		if len(allow) > 0 {
			if _, ok := allow[action.Type]; !ok {
				return &CodeError{Code: exitUsage, Err: fmt.Errorf("policy denied action type: %s (not in allow list)", action.Type)}
			}
		}
		if _, ok := deny[action.Type]; ok {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("policy denied action type: %s", action.Type)}
		}
		if isDestructiveActionType(action.Type) {
			destructive++
		}
	}
	if policy.MaxDestructiveActions > 0 && destructive > policy.MaxDestructiveActions {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("policy exceeded max destructive actions: %d > %d", destructive, policy.MaxDestructiveActions)}
	}
	return nil
}

func isDestructiveActionType(actionType string) bool {
	switch actionType {
	case "task_delete", "project_delete", "section_delete", "label_delete", "comment_delete", "project_archive":
		return true
	default:
		return false
	}
}
