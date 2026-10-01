package cli

import (
	"errors"

	"github.com/agisilaos/todoist-cli/internal/api"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

type taskAncestry struct {
	ctx     *Context
	tasks   map[string]api.Task
	fetched map[string]bool
}

func newTaskAncestry(ctx *Context, tasks []api.Task) (*taskAncestry, error) {
	out := &taskAncestry{ctx: ctx, tasks: map[string]api.Task{}, fetched: map[string]bool{}}
	for _, task := range tasks {
		if !apptasks.ValidTaskID(task.ID) {
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
func (a *taskAncestry) chain(id string) ([]api.Task, error) {
	return apptasks.AncestryChain(id, a.load)
}
func (a *taskAncestry) clearDestination(id string, clearParent, clearSection bool) (map[string]any, bool, error) {
	chain, err := a.chain(id)
	if err != nil {
		return nil, false, err
	}
	root := chain[len(chain)-1]
	if root.ResponseFact("project_id").State != api.ResponseValue || root.ResponseFact("section_id").State == api.ResponseAbsent {
		priorParent, _ := apptasks.KnownTaskLink(root, "parent_id")
		root, err = a.fetch(root.ID)
		if err != nil {
			return nil, false, err
		}
		refreshedParent, err := apptasks.KnownTaskLink(root, "parent_id")
		if err != nil || refreshedParent != priorParent {
			return nil, false, errors.New("task ancestry changed during preflight")
		}
		chain[len(chain)-1] = root
	}
	body, unchanged, err := apptasks.ClearHierarchyDestination(chain, clearParent, clearSection)
	if errors.Is(err, apptasks.ErrInheritedSection) {
		err = &CodeError{Code: exitUsage, Err: err}
	}
	return body, unchanged, err
}
func (a *taskAncestry) checkSelection(tasks []api.Task, parent string) error {
	return apptasks.CheckHierarchySelection(tasks, parent, a.load)
}
