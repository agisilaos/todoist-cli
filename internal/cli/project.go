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

func projectCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return projectList(ctx, args[1:])
	case "add":
		return projectAdd(ctx, args[1:])
	case "update":
		return projectUpdate(ctx, args[1:])
	case "archive":
		return projectArchive(ctx, args[1:])
	case "unarchive":
		return projectUnarchive(ctx, args[1:])
	case "delete":
		return projectDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown project subcommand: %s", args[0])}
	}
}

func projectList(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("project list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var archived bool
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.BoolVar(&archived, "archived", false, "List archived projects")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	path := "/projects"
	if archived {
		path = "/projects/archived"
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var allProjects []api.Project
	var next string
	for {
		var page api.Paginated[api.Project]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, path, query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allProjects = append(allProjects, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	return writeProjectList(ctx, allProjects, next)
}

func projectAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("project add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var name string
	var description string
	var parent string
	var color string
	var favorite bool
	var viewStyle string
	var workspace string
	var help bool
	fs.StringVar(&name, "name", "", "Project name")
	fs.StringVar(&description, "description", "", "Description")
	fs.StringVar(&parent, "parent", "", "Parent project")
	fs.StringVar(&color, "color", "", "Color")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	fs.StringVar(&viewStyle, "view", "", "View style")
	fs.StringVar(&workspace, "workspace", "", "Workspace ID")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if name == "" {
		printProjectHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--name is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{"name": name}
	if description != "" {
		body["description"] = description
	}
	if parent != "" {
		id, err := resolveProjectID(ctx, parent)
		if err != nil {
			return err
		}
		body["parent_id"] = id
	}
	if color != "" {
		body["color"] = color
	}
	if favorite {
		body["is_favorite"] = true
	}
	if viewStyle != "" {
		body["view_style"] = viewStyle
	}
	if workspace != "" {
		body["workspace_id"] = workspace
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project add", body)
	}
	var project api.Project
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/projects", nil, body, &project, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeProjectList(ctx, []api.Project{project}, "")
}

func projectUpdate(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("project update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var name string
	var description string
	var color string
	var favorite bool
	var viewStyle string
	var help bool
	fs.StringVar(&id, "id", "", "Project ID")
	fs.StringVar(&name, "name", "", "Project name")
	fs.StringVar(&description, "description", "", "Description")
	fs.StringVar(&color, "color", "", "Color")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	fs.StringVar(&viewStyle, "view", "", "View style")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if id == "" {
		printProjectHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id is required")}
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if color != "" {
		body["color"] = color
	}
	if favorite {
		body["is_favorite"] = true
	}
	if viewStyle != "" {
		body["view_style"] = viewStyle
	}
	if len(body) == 0 {
		printProjectHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("no fields to update")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project update", body)
	}
	var project api.Project
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/projects/"+id, nil, body, &project, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeProjectList(ctx, []api.Project{project}, "")
}

func projectArchive(ctx *Context, args []string) error {
	id, err := requireIDArg("project archive", args)
	if err != nil {
		printProjectHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !ctx.Global.Force && !ctx.Global.DryRun {
		ok, err := confirm(ctx, fmt.Sprintf("Archive project %s?", id))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project archive", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/projects/"+id+"/archive", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "archived", id)
}

func projectUnarchive(ctx *Context, args []string) error {
	id, err := requireIDArg("project unarchive", args)
	if err != nil {
		printProjectHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project unarchive", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/projects/"+id+"/unarchive", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "unarchived", id)
}

func projectDelete(ctx *Context, args []string) error {
	id, err := requireIDArg("project delete", args)
	if err != nil {
		printProjectHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !ctx.Global.Force && !ctx.Global.DryRun {
		ok, err := confirm(ctx, fmt.Sprintf("Delete project %s?", id))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/projects/"+id, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}

func writeProjectList(ctx *Context, projects []api.Project, cursor string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, projects, output.Meta{RequestID: ctx.RequestID, Count: len(projects), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		items := make([]any, 0, len(projects))
		for _, project := range projects {
			items = append(items, project)
		}
		return output.WriteNDJSON(ctx.Stdout, items)
	}
	rows := make([][]string, 0, len(projects))
	for _, project := range projects {
		rows = append(rows, []string{
			project.ID,
			project.Name,
			project.ParentID,
			strconv.FormatBool(project.IsArchived),
			strconv.FormatBool(project.IsShared),
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Name", "Parent", "Archived", "Shared"}, rows)
}
