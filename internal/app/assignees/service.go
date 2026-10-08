package assignees

import (
	"strings"

	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
)

type Collaborator struct {
	ID    string
	Name  string
	Email string
}

type ParsedRef struct {
	ID   string
	IsMe bool
}

func ParseRef(ref string) ParsedRef {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return ParsedRef{}
	}
	if strings.EqualFold(trimmed, "me") {
		return ParsedRef{IsMe: true}
	}
	normalized, direct := apprefs.NormalizeRef(trimmed)
	if direct {
		return ParsedRef{ID: normalized}
	}
	return ParsedRef{}
}

func MatchCollaboratorID(ref string, collaborators []Collaborator) (id string, candidates []apprefs.Candidate, found bool) {
	trimmed := strings.TrimSpace(ref)
	for _, c := range collaborators {
		if c.ID == trimmed || strings.EqualFold(c.Email, trimmed) {
			return c.ID, nil, true
		}
	}
	var exact, fuzzy []apprefs.Candidate
	lower := strings.ToLower(trimmed)
	for _, c := range collaborators {
		candidate := apprefs.Candidate{ID: c.ID, Name: c.Name + " <" + c.Email + ">"}
		if strings.EqualFold(c.Name, trimmed) {
			exact = append(exact, candidate)
		} else if strings.Contains(strings.ToLower(c.Name), lower) || strings.Contains(strings.ToLower(c.Email), lower) {
			fuzzy = append(fuzzy, candidate)
		}
	}
	if len(exact) > 0 {
		fuzzy = exact
	}
	if len(fuzzy) == 1 {
		return fuzzy[0].ID, nil, true
	}
	if len(fuzzy) > 1 {
		return "", fuzzy, false
	}
	return "", nil, false
}
