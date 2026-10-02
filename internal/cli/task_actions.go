package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

type cliTaskResolver struct {
	ctx *Context
}

func (r cliTaskResolver) ResolveTaskRef(_ context.Context, ref string) (api.Task, error) {
	return resolveTaskRef(r.ctx, ref)
}

func asUsageIfGeneric(err error) error {
	if err == nil {
		return nil
	}
	var codeErr *CodeError
	if errors.As(err, &codeErr) {
		return err
	}
	return &CodeError{Code: exitUsage, Err: err}
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
	reqID, err := ctx.Client.Post(reqCtx, taskPath(id)+"/reopen", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "reopened", id)
}

func taskDelete(ctx *Context, args []string) error {
	fs := newFlagSet("task delete")
	var id string
	var yes bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.BoolVar(&yes, "yes", false, "Skip confirmation")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	ref := ""
	if len(fs.Args()) > 0 {
		ref = strings.Join(fs.Args(), " ")
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	svc := apptasks.Service{Resolver: cliTaskResolver{ctx: ctx}}
	resolvedID, err := svc.ResolveTaskTarget(context.Background(), apptasks.ResolveTaskTargetInput{ID: id, Ref: ref})
	if err != nil {
		if strings.TrimSpace(id) == "" && strings.TrimSpace(ref) == "" {
			printTaskHelp(ctx.Stderr)
			return &CodeError{Code: exitUsage, Err: errors.New("task delete requires --id or a reference")}
		}
		printTaskHelp(ctx.Stderr)
		return asUsageIfGeneric(err)
	}
	id = resolvedID
	if !yes {
		return &CodeError{Code: exitUsage, Err: errors.New("task delete requires --yes")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, taskPath(id), nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}
