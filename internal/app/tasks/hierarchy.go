package tasks

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func ValidTaskID(id string) bool {
	return id != "" && !strings.ContainsFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

var ErrInheritedSection = errors.New("clearing an inherited section requires --clear-parent")

func KnownTaskLink(task api.Task, name string) (string, error) {
	fact := task.ResponseFact(name)
	if fact.State == api.ResponseNull {
		return "", nil
	}
	if value, ok := fact.Text(); ok && ValidTaskID(value) {
		return value, nil
	}
	return "", fmt.Errorf("%s unavailable for task %s", name, task.ID)
}
func AncestryChain(id string, load func(string) (api.Task, error)) ([]api.Task, error) {
	seen := map[string]bool{}
	var chain []api.Task
	for id != "" {
		if seen[id] {
			return nil, errors.New("task ancestry cycle")
		}
		seen[id] = true
		task, err := load(id)
		if err != nil {
			return nil, err
		}
		parent, err := KnownTaskLink(task, "parent_id")
		if err != nil {
			return nil, err
		}
		chain = append(chain, task)
		id = parent
	}
	return chain, nil
}
func ClearHierarchyDestination(chain []api.Task, clearParent, clearSection bool) (map[string]any, bool, error) {
	if len(chain) == 0 {
		return nil, false, errors.New("task ancestry unavailable")
	}
	root := chain[len(chain)-1]
	project, ok := root.ResponseFact("project_id").Text()
	if !ok || !ValidTaskID(project) {
		return nil, false, errors.New("current project unavailable")
	}
	section, err := KnownTaskLink(root, "section_id")
	if err != nil {
		return nil, false, err
	}
	for _, task := range chain {
		if task.ResponseFact("project_id").State == api.ResponseInvalid || task.ResponseFact("section_id").State == api.ResponseInvalid {
			return nil, false, errors.New("malformed task placement evidence")
		}
		if value, ok := task.ResponseFact("project_id").Text(); ok && (!ValidTaskID(value) || value != project) {
			return nil, false, errors.New("contradictory task project ancestry")
		}
		if value, ok := task.ResponseFact("section_id").Text(); ok && (!ValidTaskID(value) || value != section) {
			return nil, false, errors.New("contradictory inherited section")
		}
	}
	child := len(chain) > 1
	if clearSection && !clearParent && child && section != "" {
		return nil, false, ErrInheritedSection
	}
	if clearSection {
		if section == "" && !clearParent {
			return nil, true, nil
		}
		if section == "" && !child {
			return nil, true, nil
		}
		return map[string]any{"project_id": project}, false, nil
	}
	if !child {
		return nil, true, nil
	}
	if section != "" {
		return map[string]any{"section_id": section}, false, nil
	}
	return map[string]any{"project_id": project}, false, nil
}
func CheckHierarchySelection(tasks []api.Task, parent string, load func(string) (api.Task, error)) error {
	selected := map[string]bool{}
	for _, task := range tasks {
		selected[task.ID] = true
	}
	for _, task := range tasks {
		chain, err := AncestryChain(task.ID, load)
		if err != nil {
			return err
		}
		for _, ancestor := range chain[1:] {
			if selected[ancestor.ID] {
				return errors.New("batch contains ancestor and descendant targets")
			}
		}
	}
	if parent != "" {
		chain, err := AncestryChain(parent, load)
		if err != nil {
			return err
		}
		for _, ancestor := range chain {
			if selected[ancestor.ID] {
				return errors.New("move destination is a selected task or its descendant")
			}
		}
	}
	return nil
}
