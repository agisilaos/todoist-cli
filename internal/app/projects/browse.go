package projects

import (
	"errors"
	"fmt"
	"strings"
)

type BrowseURLInput struct {
	ID   string
	Name string
}

func BuildBrowseURL(in BrowseURLInput) (string, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return "", errors.New("project id is required")
	}
	slug := slugifyProjectName(in.Name)
	if slug == "" {
		slug = "project"
	}
	return fmt.Sprintf("https://app.todoist.com/app/project/%s-%s", slug, id), nil
}

func slugifyProjectName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
