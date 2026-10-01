package cli

// Command flags are parsed after globals. Preserve their values during the
// global pass, including values that happen to be global flag names. The flag
// registration contract test keeps this arity catalog in sync with commands.
func commandFlagTakesValue(name string) bool {
	switch name {
	case
		"assignee", "at", "auto-reminder", "before", "bin",
		"by", "client-id", "cmd", "color", "completed-by",
		"completed-sound-desktop", "completed-sound-mobile", "confirm", "content", "context-completed",
		"context-label", "context-project", "credential-store", "cursor", "daily",
		"date-format", "days", "deadline", "description", "due",
		"due-date", "due-datetime", "due-lang", "duration", "duration-unit",
		"event", "filter", "id", "instruction", "label",
		"limit", "name", "next-week", "oauth-authorize-url", "oauth-device-url",
		"oauth-listen", "oauth-redirect-uri", "oauth-token-url", "offset", "on-error",
		"order", "out", "parent", "path", "plan",
		"plan-version", "planner", "policy", "preset", "priority",
		"project", "query", "reminder-desktop", "reminder-email", "reminder-push",
		"scope", "section", "since", "sort", "start-day", "start-page",
		"task", "theme", "time-format", "timezone", "to-workspace",
		"truncate-width", "type", "until", "view", "visibility",
		"weekly", "workspace":
		return true
	default:
		return false
	}
}
