package cli

import (
	"strings"
)

type quickAddResult struct {
	Content  string
	Project  string
	Labels   []string
	Priority int
	Due      string
}

func quickAddTokenKind(part string) string {
	switch {
	case strings.HasPrefix(part, "#") && len(part) > 1:
		return "project"
	case strings.HasPrefix(part, "@") && len(part) > 1:
		return "label"
	case isPriorityToken(part):
		return "priority"
	case strings.HasPrefix(strings.ToLower(part), "due:") && len(part) > 4:
		return "due"
	}
	return ""
}

func parseQuickAdd(input string) quickAddResult {
	parts := strings.Fields(input)
	var contentParts []string
	var res quickAddResult
	for _, part := range parts {
		switch quickAddTokenKind(part) {
		case "project":
			res.Project = strings.TrimPrefix(part, "#")
		case "label":
			res.Labels = append(res.Labels, strings.TrimPrefix(part, "@"))
		case "priority":
			res.Priority = mapPriorityToken(part)
		case "due":
			res.Due = part[4:]
		default:
			contentParts = append(contentParts, part)
		}
	}
	res.Content = strings.TrimSpace(strings.Join(contentParts, " "))
	return res
}

func isPriorityToken(part string) bool {
	if len(part) != 2 {
		return false
	}
	if part[0] != 'p' && part[0] != 'P' {
		return false
	}
	return part[1] >= '1' && part[1] <= '4'
}

func mapPriorityToken(part string) int {
	switch strings.ToLower(part) {
	case "p1":
		return 4
	case "p2":
		return 3
	case "p3":
		return 2
	case "p4":
		return 1
	default:
		return 0
	}
}
