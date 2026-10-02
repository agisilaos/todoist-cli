package cli

import (
	"errors"
	"fmt"
	"github.com/agisilaos/todoist-cli/internal/api"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
	"github.com/agisilaos/todoist-cli/internal/output"
	"net/url"
)

func writeExpandedTaskView(ctx *Context, task api.Task, full bool, sorting taskSortOptions) error {
	if !apptasks.ValidTaskID(task.ID) {
		return errors.New("expanded parent identity unavailable")
	}
	query := url.Values{"parent_id": {task.ID}, "limit": {"200"}}
	children, _, err := fetchPaginated[api.Task](ctx, "/tasks", query, true, true)
	if err != nil {
		return err
	}
	seen := map[string]bool{task.ID: true}
	for _, child := range children {
		if !apptasks.ValidTaskID(child.ID) || seen[child.ID] {
			return errors.New("expanded children missing or duplicate identity")
		}
		seen[child.ID] = true
		for _, field := range []string{"checked", "is_deleted"} {
			if value, ok := child.ResponseFact(field).Bool(); ok && value {
				return errors.New("expanded child contradicts active collection")
			}
		}
		if parent, ok := child.ResponseFact("parent_id").Text(); ok && parent != task.ID {
			return errors.New("expanded child returned contradictory parent")
		}
		if child.ResponseFact("parent_id").State == api.ResponseNull || child.ResponseFact("parent_id").State == api.ResponseInvalid {
			return errors.New("expanded child returned no parent")
		}
	}
	if err := sorting.apply(children); err != nil {
		return err
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		all := append([]api.Task{task}, children...)
		if err := writeTaskResponseIssues(ctx, all); err != nil {
			return err
		}
		values := make([]any, 0, len(children))
		for _, child := range children {
			values = append(values, taskResourceValue(ctx, child))
		}
		return writeStructuredValue(ctx, map[string]any{"task": taskResourceValue(ctx, task), "children": values, "children_complete": true})
	}
	if err := writeTaskView(ctx, task, full); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Stdout, "\nDirect active children: %d (all pages fetched)\n", len(children))
	return writeTaskList(ctx, children, "", false)
}
