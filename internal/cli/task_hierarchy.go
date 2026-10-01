package cli

import (
	"errors"
	"fmt"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type taskAncestry struct {
	ctx     *Context
	tasks   map[string]api.Task
	fetched map[string]bool
}

func newTaskAncestry(ctx *Context, tasks []api.Task) (*taskAncestry, error) {
	out := &taskAncestry{ctx: ctx, tasks: map[string]api.Task{}, fetched: map[string]bool{}}
	for _, task := range tasks {
		if !usableTaskID(task.ID) {
			return nil, errors.New("task selection contains missing ID")
		}
		if _, ok := out.tasks[task.ID]; ok {
			return nil, errors.New("task selection contains duplicate ID")
		}
		out.tasks[task.ID] = task
	}
	return out, nil
}
func (a *taskAncestry) load(id string) (api.Task, error) {
	task, ok := a.tasks[id]
	if ok && (task.ResponseFact("parent_id").State == api.ResponseNull || task.ResponseFact("parent_id").State == api.ResponseValue) {
		return task, nil
	}
	return a.fetch(id)
}
func (a *taskAncestry) fetch(id string) (api.Task, error) {
	if a.fetched[id] {
		return a.tasks[id], nil
	}
	task, err := resolveTaskRef(a.ctx, "id:"+id)
	if err != nil {
		return task, err
	}
	a.tasks[id] = task
	a.fetched[id] = true
	return task, nil
}
func knownTaskLink(task api.Task, name string) (string, error) {
	fact := task.ResponseFact(name)
	if fact.State == api.ResponseNull {
		return "", nil
	}
	if value, ok := fact.Text(); ok && usableTaskID(value) {
		return value, nil
	}
	return "", fmt.Errorf("%s unavailable for task %s", name, task.ID)
}
func (a *taskAncestry) chain(id string) ([]api.Task, error) {
	seen := map[string]bool{}
	var chain []api.Task
	for id != "" {
		if seen[id] {
			return nil, errors.New("task ancestry cycle")
		}
		seen[id] = true
		task, err := a.load(id)
		if err != nil {
			return nil, err
		}
		parent, err := knownTaskLink(task, "parent_id")
		if err != nil {
			return nil, err
		}
		chain = append(chain, task)
		id = parent
	}
	return chain, nil
}
func (a *taskAncestry) clearDestination(id string, clearParent, clearSection bool) (map[string]any, bool, error) {
	chain, err := a.chain(id)
	if err != nil {
		return nil, false, err
	}
	root := chain[len(chain)-1]
	if root.ResponseFact("project_id").State != api.ResponseValue || root.ResponseFact("section_id").State == api.ResponseAbsent {
		priorParent, _ := knownTaskLink(root, "parent_id")
		root, err = a.fetch(root.ID)
		if err != nil {
			return nil, false, err
		}
		refreshedParent, err := knownTaskLink(root, "parent_id")
		if err != nil || refreshedParent != priorParent {
			return nil, false, errors.New("task ancestry changed during preflight")
		}
		chain[len(chain)-1] = root
	}
	project, ok := root.ResponseFact("project_id").Text()
	if !ok || !usableTaskID(project) {
		return nil, false, errors.New("current project unavailable")
	}
	section, err := knownTaskLink(root, "section_id")
	if err != nil {
		return nil, false, err
	}
	for _, task := range chain {
		if task.ResponseFact("project_id").State == api.ResponseInvalid || task.ResponseFact("section_id").State == api.ResponseInvalid {
			return nil, false, errors.New("malformed task placement evidence")
		}
		if value, ok := task.ResponseFact("project_id").Text(); ok && (!usableTaskID(value) || value != project) {
			return nil, false, errors.New("contradictory task project ancestry")
		}
		if value, ok := task.ResponseFact("section_id").Text(); ok && (!usableTaskID(value) || value != section) {
			return nil, false, errors.New("contradictory inherited section")
		}
	}
	child := len(chain) > 1
	if clearSection && !clearParent && child && section != "" {
		return nil, false, &CodeError{Code: exitUsage, Err: errors.New("clearing an inherited section requires --clear-parent")}
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
func (a *taskAncestry) checkSelection(tasks []api.Task, parent string) error {
	selected := map[string]bool{}
	for _, task := range tasks {
		selected[task.ID] = true
	}
	for _, task := range tasks {
		chain, err := a.chain(task.ID)
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
		chain, err := a.chain(parent)
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
