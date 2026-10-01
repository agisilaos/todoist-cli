package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type taskActionOptions struct {
	id, project, section, parent, filter    string
	yes, forever, clearParent, clearSection bool
}
type plannedTaskAction struct {
	id        string
	before    *api.Task
	body      map[string]any
	native    string
	unchanged bool
}
type taskBatchTarget struct {
	ID         string             `json:"id"`
	Outcome    api.TaskWriteState `json:"outcome"`
	Dispatched bool               `json:"dispatched"`
	RequestID  string             `json:"request_id,omitempty"`
	Error      string             `json:"error,omitempty"`
}

func (o *taskActionOptions) bind(fs *flag.FlagSet, move bool) {
	fs.StringVar(&o.id, "id", "", "Task ID")
	fs.StringVar(&o.filter, "filter", "", "Complete filtered selection")
	fs.BoolVar(&o.yes, "yes", false, "Confirm filtered batch")
	if move {
		fs.StringVar(&o.project, "project", "", "Project destination or section scope")
		fs.StringVar(&o.section, "section", "", "Section destination")
		fs.StringVar(&o.parent, "parent", "", "Parent destination")
		fs.BoolVar(&o.clearParent, "clear-parent", false, "Detach to current effective section/project")
		fs.BoolVar(&o.clearSection, "clear-section", false, "Clear current section")
	} else {
		fs.BoolVar(&o.forever, "forever", false, "Permanently complete recurring task and subtasks")
	}
}

func taskMove(ctx *Context, args []string) error     { return taskAction(ctx, args, true) }
func taskComplete(ctx *Context, args []string) error { return taskAction(ctx, args, false) }
func taskAction(ctx *Context, args []string, move bool) error {
	operation := "task complete"
	if move {
		operation = "task move"
	}
	fs := newFlagSet(operation)
	var o taskActionOptions
	var help bool
	o.bind(fs, move)
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	fail := func(text string) error { return &CodeError{Code: exitUsage, Err: errors.New(text)} }
	present := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { present[f.Name] = true })
	for name, value := range map[string]string{"id": o.id, "project": o.project, "section": o.section, "parent": o.parent, "filter": o.filter} {
		if present[name] && strings.TrimSpace(value) == "" {
			return fail("--" + name + " cannot be empty")
		}
	}
	if present["parent"] {
		id, err := normalizeTaskInputID(o.parent)
		if err != nil {
			return fail("invalid --parent: " + err.Error())
		}
		o.parent = id
	}
	ref := strings.Join(fs.Args(), " ")
	if o.id != "" && ref != "" || o.filter != "" && (o.id != "" || ref != "") {
		return fail("task selectors cannot be combined")
	}
	if o.id == "" && ref == "" && o.filter == "" {
		return fail("task target required")
	}
	if move {
		clear := o.clearParent || o.clearSection
		destination := o.project != "" || o.section != "" || o.parent != ""
		if !clear && !destination {
			return fail("move destination or clear flag required")
		}
		if clear && destination {
			return fail("hierarchy clear flags cannot accompany destination setters")
		}
		if o.parent != "" && (o.project != "" || o.section != "") {
			return fail("parent cannot accompany project or section destination")
		}
	}
	if o.filter != "" && !o.yes && !ctx.Global.Force {
		return fail("filtered batch requires --yes (or --force)")
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	var targets []api.Task
	var before *api.Task
	var err error
	if o.filter != "" {
		targets, _, err = listTasksByFilter(ctx, o.filter, "", 200, true, true)
		if err != nil {
			return err
		}
	} else {
		o.id, before, err = resolveEditingTarget(ctx, o.id, ref, o.clearParent || o.clearSection)
		if err != nil {
			return err
		}
		task := api.Task{ID: o.id}
		if before != nil {
			task = *before
		}
		targets = []api.Task{task}
	}
	ancestry, err := newTaskAncestry(ctx, targets)
	if err != nil {
		if o.filter != "" {
			return writeBatchPreflightFailure(ctx, o.filter, targets, operation, err)
		}
		return err
	}
	var destination map[string]any
	if move && !o.clearParent && !o.clearSection {
		destination, err = buildTaskMovePayload(ctx, "", o.project, "", o.section, o.parent)
		if err != nil {
			if o.filter != "" {
				return writeBatchPreflightFailure(ctx, o.filter, targets, operation, err)
			}
			return err
		}
		if value, ok := destination["parent_id"].(string); ok {
			o.parent = value
		}
	}
	if o.filter != "" || o.parent != "" {
		if err := ancestry.checkSelection(targets, o.parent); err != nil {
			if o.filter != "" {
				return writeBatchPreflightFailure(ctx, o.filter, targets, operation, err)
			}
			return err
		}
	}
	plans := make([]plannedTaskAction, 0, len(targets))
	var preflight error
	for _, task := range targets {
		plan := plannedTaskAction{id: task.ID, before: before, body: destination}
		if o.filter != "" {
			copy := task
			plan.before = &copy
		}
		if o.clearParent || o.clearSection {
			plan.body, plan.unchanged, err = ancestry.clearDestination(task.ID, o.clearParent, o.clearSection)
			if err != nil {
				preflight = err
			}
			plan.native = "item_move"
		}
		if o.forever {
			plan.native = "item_complete"
			plan.body = map[string]any{}
		}
		plans = append(plans, plan)
	}
	if preflight != nil {
		if o.filter != "" {
			return writeBatchPreflightFailure(ctx, o.filter, targets, operation, preflight)
		}
		return preflight
	}
	if ctx.Global.DryRun {
		if o.filter != "" {
			ids := make([]string, 0, len(plans))
			payloads := make([]any, 0, len(plans))
			for _, plan := range plans {
				ids = append(ids, plan.id)
				payloads = append(payloads, map[string]any{"id": plan.id, "payload": plan.body, "native": plan.native, "unchanged": plan.unchanged})
			}
			return writeDryRun(ctx, operation+" bulk", map[string]any{"filter": o.filter, "count": len(ids), "ids": ids, "payload": destination, "targets": payloads, "forever": o.forever})
		}
		payload := plans[0].body
		if payload == nil {
			payload = map[string]any{"id": o.id}
		}
		if o.forever {
			payload = map[string]any{"id": o.id, "forever": true}
		}
		return writeTaskActionPreview(ctx, strings.TrimPrefix(operation, "task "), o.id, before, payload)
	}
	for _, plan := range plans {
		if !plan.unchanged {
			if err := currentAuthorization(ctx).CheckMutation(); err != nil {
				return err
			}
			break
		}
	}
	if o.filter == "" {
		plan := plans[0]
		if plan.unchanged {
			return writeTaskAcknowledgement(ctx, plan.id, strings.ReplaceAll(operation, " ", "_"), "unchanged")
		}
		raw, _, err := dispatchTaskAction(ctx, plan, move)
		if err != nil {
			return err
		}
		if move {
			return writeTaskMoveResult(ctx, plan.id, plan.before, plan.body, raw)
		}
		if o.forever && taskActionHuman(ctx) {
			fmt.Fprintf(ctx.Stdout, "Permanent completion accepted\nID: %s\n", captureText(plan.id))
			return nil
		}
		return writeTaskCompletionResult(ctx, plan.id, plan.before)
	}
	results := make([]taskBatchTarget, 0, len(plans))
	stop := false
	for _, plan := range plans {
		target := taskBatchTarget{ID: plan.id, Outcome: "unattempted"}
		if plan.unchanged {
			target.Outcome = api.TaskWriteAccepted
		} else if !stop {
			_, requestID, err := dispatchTaskAction(ctx, plan, move)
			target.RequestID = requestID
			target.Outcome = api.TaskWriteOutcome(err)
			target.Dispatched = target.Outcome != api.TaskWriteNotDispatched
			if err != nil {
				target.Error = safeErrorText(ctx, err)
				if target.Outcome == api.TaskWriteNotDispatched {
					target.Outcome = api.TaskWriteRejected
				}
				if target.Outcome == api.TaskWriteUncertain || target.Outcome == api.TaskWriteAccepted {
					stop = true
				}
			}
		}
		results = append(results, target)
	}
	return writeTaskBatch(ctx, o.filter, operation, results, nil)
}
func dispatchTaskAction(ctx *Context, plan plannedTaskAction, move bool) ([]byte, string, error) {
	if plan.native != "" {
		body := map[string]any{"id": plan.id}
		for key, value := range plan.body {
			body[key] = value
		}
		return nativeTaskCommand(ctx, plan.native, body)
	}
	if move {
		return postTaskResource(ctx, taskPath(plan.id)+"/move", plan.body)
	}
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	requestID, err := ctx.Client.Post(reqCtx, taskPath(plan.id)+"/close", nil, nil, nil, true)
	setRequestID(ctx, requestID)
	return nil, requestID, err
}
func writeBatchPreflightFailure(ctx *Context, filter string, tasks []api.Task, operation string, err error) error {
	targets := make([]taskBatchTarget, 0, len(tasks))
	for _, task := range tasks {
		targets = append(targets, taskBatchTarget{ID: task.ID, Outcome: api.TaskWriteRejected, Error: "preflight rejected batch: " + safeErrorText(ctx, err)})
	}
	return writeTaskBatch(ctx, filter, operation, targets, err)
}
func writeTaskBatch(ctx *Context, filter, operation string, targets []taskBatchTarget, preflight error) error {
	counts := map[api.TaskWriteState]int{"accepted": 0, "rejected": 0, "uncertain": 0, "unattempted": 0}
	dispatched, unchanged := 0, 0
	for _, target := range targets {
		counts[target.Outcome]++
		if target.Dispatched {
			dispatched++
		} else if target.Outcome == api.TaskWriteAccepted {
			unchanged++
		}
	}
	result := map[string]any{"filter": filter, "operation": strings.ReplaceAll(operation, " ", "_"), "count": len(targets), "targets": targets, "dispatched": dispatched, "unchanged": unchanged, "accepted": counts["accepted"], "rejected": counts["rejected"], "uncertain": counts["uncertain"], "unattempted": counts["unattempted"], "failed": counts["rejected"] + counts["uncertain"]}
	if operation == "task move" {
		result["moved"] = counts["accepted"] - unchanged
	} else {
		result["completed"] = counts["accepted"]
	}
	if preflight != nil {
		result["preflight_error"] = safeErrorText(ctx, preflight)
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		if err := writeStructuredValue(ctx, result); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(ctx.Stdout, "%s batch: accepted=%d rejected=%d uncertain=%d unattempted=%d dispatched=%d unchanged=%d\n", operation, counts["accepted"], counts["rejected"], counts["uncertain"], counts["unattempted"], dispatched, unchanged)
		for _, target := range targets {
			fmt.Fprintf(ctx.Stdout, "ID: %s · %s · dispatched=%t", captureText(target.ID), target.Outcome, target.Dispatched)
			if target.RequestID != "" {
				fmt.Fprintf(ctx.Stdout, " · request_id=%s", captureText(target.RequestID))
			}
			if target.Error != "" {
				fmt.Fprintf(ctx.Stdout, " · %s", captureText(target.Error))
			}
			fmt.Fprintln(ctx.Stdout)
		}
	}
	if preflight != nil || counts["accepted"] != len(targets) {
		return &CodeError{Code: exitError, Err: errors.New("task batch incomplete; inspect target accounting before resubmitting")}
	}
	return nil
}
