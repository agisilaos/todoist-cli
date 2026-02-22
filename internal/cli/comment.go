package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/api"
	appcomments "github.com/agisilaos/todoist-cli/internal/app/comments"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func commentCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":  "list",
		"rm":  "delete",
		"del": "delete",
	})
	switch sub {
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
	fs := newFlagSet("comment list")
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
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	if err := appcomments.ValidateList(appcomments.ListInput{TaskID: task, ProjectID: project}); err != nil {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
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
	allComments, next, err := fetchPaginated[api.Comment](ctx, "/comments", query, all)
	if err != nil {
		return err
	}
	return writeCommentList(ctx, allComments, next)
}

func commentAdd(ctx *Context, args []string) error {
	fs := newFlagSet("comment add")
	var content string
	var task string
	var project string
	var help bool
	fs.StringVar(&content, "content", "", "Comment content")
	fs.StringVar(&task, "task", "", "Task ID")
	fs.StringVar(&project, "project", "", "Project ID")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	projectID := ""
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		projectID = id
	}
	body, err := appcomments.BuildAddPayload(appcomments.AddInput{
		Content:   content,
		TaskID:    task,
		ProjectID: projectID,
	})
	if err != nil {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
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
	fs := newFlagSet("comment update")
	var id string
	var content string
	var help bool
	fs.StringVar(&id, "id", "", "Comment ID")
	fs.StringVar(&content, "content", "", "Comment content")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCommentHelp(ctx.Stdout)
		return nil
	}
	commentID, body, err := appcomments.BuildUpdatePayload(appcomments.UpdateInput{
		ID:      id,
		Content: content,
	})
	if err != nil {
		printCommentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "comment update", body)
	}
	var comment api.Comment
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/comments/"+commentID, nil, body, &comment, true)
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
		return output.WriteNDJSONSlice(ctx.Stdout, comments)
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
