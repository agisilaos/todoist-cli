package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func taskMove(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task move", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var project string
	var section string
	var parent string
	var filter string
	var yes bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&section, "section", "", "Section")
	fs.StringVar(&parent, "parent", "", "Parent")
	fs.StringVar(&filter, "filter", "", "Filter query for bulk move")
	fs.BoolVar(&yes, "yes", false, "Required for bulk move")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if filter == "" && id == "" && len(fs.Args()) > 0 {
		ref := strings.Join(fs.Args(), " ")
		if err := ensureClient(ctx); err != nil {
			return err
		}
		task, err := resolveTaskRef(ctx, ref)
		if err != nil {
			return err
		}
		id = task.ID
	}
	if filter == "" && id == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id is required (or pass a text reference)")}
	}
	if project == "" && section == "" && parent == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("at least one of --project, --section, or --parent is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if filter != "" {
		if id != "" || len(fs.Args()) > 0 {
			return &CodeError{Code: exitUsage, Err: errors.New("--filter cannot be combined with --id or positional task reference")}
		}
		if !yes && !ctx.Global.Force {
			return &CodeError{Code: exitUsage, Err: errors.New("bulk move with --filter requires --yes (or --force)")}
		}
		tasks, _, err := listTasksByFilter(ctx, filter, "", 200, true)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(tasks))
		for _, t := range tasks {
			ids = append(ids, t.ID)
		}
		body, err := buildTaskMovePayload(ctx, "", project, "", section, parent)
		if err != nil {
			return err
		}
		if ctx.Global.DryRun {
			return writeDryRun(ctx, "task move bulk", map[string]any{"filter": filter, "count": len(ids), "ids": ids, "payload": body})
		}
		moved := 0
		failed := 0
		for _, taskID := range ids {
			reqCtx, cancel := requestContext(ctx)
			reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+taskID+"/move", nil, body, nil, true)
			cancel()
			if err != nil {
				failed++
				continue
			}
			setRequestID(ctx, reqID)
			moved++
		}
		if ctx.Mode == output.ModeJSON {
			return output.WriteJSON(ctx.Stdout, map[string]any{
				"filter": filter,
				"moved":  moved,
				"failed": failed,
				"count":  len(ids),
			}, output.Meta{RequestID: ctx.RequestID})
		}
		fmt.Fprintf(ctx.Stdout, "bulk move complete: moved=%d failed=%d total=%d\n", moved, failed, len(ids))
		return nil
	}
	body, err := buildTaskMovePayload(ctx, "", project, "", section, parent)
	if err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task move", body)
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id+"/move", nil, body, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "moved", id)
}

func taskComplete(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task complete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var filter string
	var yes bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.StringVar(&filter, "filter", "", "Filter query for bulk complete")
	fs.BoolVar(&yes, "yes", false, "Required for bulk complete")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if filter != "" {
		if id != "" || len(fs.Args()) > 0 {
			return &CodeError{Code: exitUsage, Err: errors.New("--filter cannot be combined with --id or positional task reference")}
		}
		if !yes && !ctx.Global.Force {
			return &CodeError{Code: exitUsage, Err: errors.New("bulk complete with --filter requires --yes (or --force)")}
		}
		tasks, _, err := listTasksByFilter(ctx, filter, "", 200, true)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(tasks))
		for _, t := range tasks {
			ids = append(ids, t.ID)
		}
		if ctx.Global.DryRun {
			return writeDryRun(ctx, "task complete bulk", map[string]any{"filter": filter, "count": len(ids), "ids": ids})
		}
		completed := 0
		failed := 0
		for _, taskID := range ids {
			reqCtx, cancel := requestContext(ctx)
			reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+taskID+"/close", nil, nil, nil, true)
			cancel()
			if err != nil {
				failed++
				continue
			}
			setRequestID(ctx, reqID)
			completed++
		}
		if ctx.Mode == output.ModeJSON {
			return output.WriteJSON(ctx.Stdout, map[string]any{
				"filter":    filter,
				"completed": completed,
				"failed":    failed,
				"count":     len(ids),
			}, output.Meta{RequestID: ctx.RequestID})
		}
		fmt.Fprintf(ctx.Stdout, "bulk complete done: completed=%d failed=%d total=%d\n", completed, failed, len(ids))
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		ref := strings.Join(fs.Args(), " ")
		task, err := resolveTaskRef(ctx, ref)
		if err != nil {
			return err
		}
		id = task.ID
	}
	if id == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("task complete requires --id or a reference")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task complete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id+"/close", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "completed", id)
}

func taskReopen(ctx *Context, args []string) error {
	id, err := requireTaskID(ctx, "task reopen", args)
	if err != nil {
		printTaskHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task reopen", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id+"/reopen", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "reopened", id)
}

func taskDelete(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var yes bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.BoolVar(&yes, "yes", false, "Skip confirmation")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		if err := ensureClient(ctx); err != nil {
			return err
		}
		ref := strings.Join(fs.Args(), " ")
		task, err := resolveTaskRef(ctx, ref)
		if err != nil {
			return err
		}
		id = task.ID
	}
	if id == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("task delete requires --id or a reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !yes {
		return &CodeError{Code: exitUsage, Err: errors.New("task delete requires --yes")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/tasks/"+id, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}
