package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func commentCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return commentList(ctx, args[1:])
	case "add":
		return commentAdd(ctx, args[1:])
	case "update":
		return commentUpdate(ctx, args[1:])
	case "delete":
		return commentDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown comment subcommand: %s", args[0])}
	}
}

func commentList(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("comment list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var task string
	var project string
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.StringVar(&task, "task", "", "Task ID")
	fs.StringVar(&project, "project", "", "Project ID")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	if task == "" && project == "" {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--task or --project is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	query := url.Values{}
	if task != "" {
		query.Set("task_id", task)
	}
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		query.Set("project_id", id)
	}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var allComments []api.Comment
	var next string
	for {
		var page api.Paginated[api.Comment]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/comments", query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allComments = append(allComments, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	return writeCommentList(ctx, allComments, next)
}

func commentAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("comment add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var content string
	var task string
	var project string
	var help bool
	fs.StringVar(&content, "content", "", "Comment content")
	fs.StringVar(&task, "task", "", "Task ID")
	fs.StringVar(&project, "project", "", "Project ID")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	if content == "" {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--content is required")}
	}
	if task == "" && project == "" {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--task or --project is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{"content": content}
	if task != "" {
		body["task_id"] = task
	}
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		body["project_id"] = id
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "comment add", body)
	}
	var comment api.Comment
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/comments", nil, body, &comment, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeCommentList(ctx, []api.Comment{comment}, "")
}

func commentUpdate(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("comment update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var content string
	var help bool
	fs.StringVar(&id, "id", "", "Comment ID")
	fs.StringVar(&content, "content", "", "Comment content")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	if id == "" || content == "" {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id and --content are required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{"content": content}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "comment update", body)
	}
	var comment api.Comment
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/comments/"+id, nil, body, &comment, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeCommentList(ctx, []api.Comment{comment}, "")
}

func commentDelete(ctx *Context, args []string) error {
	id, err := requireIDArg("comment delete", args)
	if err != nil {
		printCommentHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !ctx.Global.Force && !ctx.Global.DryRun {
		ok, err := confirm(ctx, fmt.Sprintf("Delete comment %s?", id))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "comment delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/comments/"+id, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}

func writeCommentList(ctx *Context, comments []api.Comment, cursor string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, comments, output.Meta{RequestID: ctx.RequestID, Count: len(comments), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		items := make([]any, 0, len(comments))
		for _, comment := range comments {
			items = append(items, comment)
		}
		return output.WriteNDJSON(ctx.Stdout, items)
	}
	rows := make([][]string, 0, len(comments))
	for _, comment := range comments {
		rows = append(rows, []string{
			comment.ID,
			comment.Content,
			comment.PostedAt,
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Content", "Posted"}, rows)
}
