package projects

import (
	"errors"
	"strings"
)

type MoveInput struct {
	Ref         string
	ToWorkspace string
	ToPersonal  bool
	Visibility  string
}

type MovePlan struct {
	Ref         string
	ToWorkspace string
	ToPersonal  bool
	Visibility  string
}

func BuildMovePlan(in MoveInput) (MovePlan, error) {
	ref := strings.TrimSpace(in.Ref)
	toWorkspace := strings.TrimSpace(in.ToWorkspace)
	visibility := strings.ToLower(strings.TrimSpace(in.Visibility))
	if ref == "" {
		return MovePlan{}, errors.New("project move requires a project reference")
	}
	if (toWorkspace == "" && !in.ToPersonal) || (toWorkspace != "" && in.ToPersonal) {
		return MovePlan{}, errors.New("project move requires exactly one target: --to-workspace or --to-personal")
	}
	if in.ToPersonal && visibility != "" {
		return MovePlan{}, errors.New("--visibility can only be used with --to-workspace")
	}
	if visibility != "" && visibility != "restricted" && visibility != "team" && visibility != "public" {
		return MovePlan{}, errors.New("--visibility must be one of: restricted, team, public")
	}
	return MovePlan{
		Ref:         ref,
		ToWorkspace: toWorkspace,
		ToPersonal:  in.ToPersonal,
		Visibility:  visibility,
	}, nil
}
