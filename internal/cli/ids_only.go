package cli

import (
	"fmt"
	"strings"
	"unicode"
)

// Eligibility is checked before loading configuration or dispatching commands:
// mutation commands also reuse list writers for their successful results.
func idsOnlyEligible(args []string, help bool) bool {
	if len(args) == 0 || help || args[0] == "help" {
		return true
	}
	switch args[0] {
	case "today", "upcoming", "completed", "activity":
		return true
	case "inbox":
		return len(args) == 1 || args[1] == "help"
	case "task", "project", "section", "label", "comment", "filter", "workspace", "reminder", "notification":
		// A bare resource command prints help.
		if len(args) == 1 {
			return true
		}
		return args[1] == "help" || args[1] == "list" || args[1] == "ls" ||
			(args[0] == "project" && args[1] == "collaborators") ||
			(args[0] == "filter" && args[1] == "show")
	}
	return false
}

func writeIDs[T any](ctx *Context, items []T, idOf func(T) string, cursor string) error {
	// Validate the entire fetched collection before emitting any IDs.
	for i, item := range items {
		id := idOf(item)
		if id == "" || strings.ContainsFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return fmt.Errorf("invalid ID at result %d: expected a nonempty ID without whitespace or control characters", i+1)
		}
	}
	for _, item := range items {
		if _, err := fmt.Fprintln(ctx.Stdout, idOf(item)); err != nil {
			return err
		}
	}
	return writeCursorNotice(ctx, cursor)
}
