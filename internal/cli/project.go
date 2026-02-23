package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/api"
	appprojects "github.com/agisilaos/todoist-cli/internal/app/projects"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func projectCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":   "list",
		"show": "view",
		"create": "add",
		"rm":   "delete",
		"del":  "delete",
	})
	switch sub {
	case "list":
		return projectList(ctx, args[1:])
	case "view":
		return projectView(ctx, args[1:])
	case "browse":
		return projectBrowse(ctx, args[1:])
	case "collaborators":
		return projectCollaborators(ctx, args[1:])
	case "add":
		return projectAdd(ctx, args[1:])
	case "update":
		return projectUpdate(ctx, args[1:])
	case "move":
		return projectMove(ctx, args[1:])
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

var openProjectBrowserFn = openOAuthBrowser

func projectView(ctx *Context, args []string) error {
	fs := newFlagSet("project view")
	var id string
	var help bool
	fs.StringVar(&id, "id", "", "Project ID or name")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	if id == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("project view requires --id or a project reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	resolvedID, err := resolveProjectID(ctx, id)
	if err != nil {
		return err
	}
	var project api.Project
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Get(reqCtx, "/projects/"+resolvedID, nil, &project)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, project, output.Meta{RequestID: ctx.RequestID})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, []api.Project{project})
	}
	rows := [][]string{
		{"ID", project.ID},
		{"Name", project.Name},
		{"Description", project.Description},
		{"Parent", project.ParentID},
		{"Workspace", project.WorkspaceID},
		{"View", project.ViewStyle},
		{"Archived", strconv.FormatBool(project.IsArchived)},
		{"Shared", strconv.FormatBool(project.IsShared)},
		{"Favorite", strconv.FormatBool(project.IsFavorite)},
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"Field", "Value"}, rows)
}

func projectCollaborators(ctx *Context, args []string) error {
	fs := newFlagSet("project collaborators")
	var id string
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.StringVar(&id, "id", "", "Project ID or name")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	if id == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("project collaborators requires --id or a project reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	resolvedID, err := resolveProjectID(ctx, id)
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	allCollaborators, next, err := fetchPaginated[api.Collaborator](ctx, "/projects/"+resolvedID+"/collaborators", query, all)
	if err != nil {
		return err
	}
	return writeProjectCollaborators(ctx, allCollaborators, next)
}

func projectList(ctx *Context, args []string) error {
	fs := newFlagSet("project list")
	var archived bool
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.BoolVar(&archived, "archived", false, "List archived projects")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
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
	allProjects, next, err := fetchPaginated[api.Project](ctx, path, query, all)
	if err != nil {
		return err
	}
	return writeProjectList(ctx, allProjects, next)
}

func projectAdd(ctx *Context, args []string) error {
	fs := newFlagSet("project add")
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
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	parentID := ""
	if parent != "" {
		id, err := resolveProjectID(ctx, parent)
		if err != nil {
			return err
		}
		parentID = id
	}
	body, err := appprojects.BuildAddPayload(appprojects.AddInput{
		Name:        name,
		Description: description,
		ParentID:    parentID,
		Color:       color,
		Favorite:    favorite,
		ViewStyle:   viewStyle,
		WorkspaceID: workspace,
	})
	if err != nil {
		printProjectHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
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
	fs := newFlagSet("project update")
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
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	var err error
	id, body, err := appprojects.BuildUpdatePayload(appprojects.UpdateInput{
		ID:          id,
		Name:        name,
		Description: description,
		Color:       color,
		Favorite:    favorite,
		ViewStyle:   viewStyle,
	})
	if err != nil {
		printProjectHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	id, directID, err := apprefs.NormalizeEntityRef(id, "project")
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if !directID || id == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("project update requires --id")}
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
	id, err := requireEntityIDArg("project archive", "project", args)
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
	id, err := requireEntityIDArg("project unarchive", "project", args)
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
	id, err := requireEntityIDArg("project delete", "project", args)
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

func projectBrowse(ctx *Context, args []string) error {
	fs := newFlagSet("project browse")
	var id string
	var help bool
	fs.StringVar(&id, "id", "", "Project ID or name")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	if id == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("project browse requires --id or a project reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	resolvedID, err := resolveProjectID(ctx, id)
	if err != nil {
		return err
	}
	project, err := fetchProjectByID(ctx, resolvedID)
	if err != nil {
		return err
	}
	browseURL, err := appprojects.BuildBrowseURL(appprojects.BrowseURLInput{ID: project.ID, Name: project.Name})
	if err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project browse", map[string]any{"id": project.ID, "url": browseURL})
	}
	if err := openProjectBrowserFn(browseURL); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"id":     project.ID,
			"name":   project.Name,
			"url":    browseURL,
			"opened": true,
		}, output.Meta{RequestID: ctx.RequestID})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, [][]string{{project.ID, project.Name, browseURL}})
	}
	fmt.Fprintf(ctx.Stdout, "opened %s\n", browseURL)
	return nil
}

func projectMove(ctx *Context, args []string) error {
	fs := newFlagSet("project move")
	var ref string
	var toWorkspace string
	var toPersonal bool
	var visibility string
	var yes bool
	var help bool
	fs.StringVar(&ref, "id", "", "Project ID or name")
	fs.StringVar(&toWorkspace, "to-workspace", "", "Target workspace")
	fs.BoolVar(&toPersonal, "to-personal", false, "Move project to personal")
	fs.StringVar(&visibility, "visibility", "", "Workspace visibility (restricted|team|public)")
	fs.BoolVar(&yes, "yes", false, "Confirm move")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProjectHelp(ctx.Stdout)
		return nil
	}
	if ref == "" && len(fs.Args()) > 0 {
		ref = fs.Arg(0)
	}
	plan, err := appprojects.BuildMovePlan(appprojects.MoveInput{
		Ref:         ref,
		ToWorkspace: toWorkspace,
		ToPersonal:  toPersonal,
		Visibility:  visibility,
	})
	if err != nil {
		printProjectHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	projectID, err := resolveProjectID(ctx, plan.Ref)
	if err != nil {
		return err
	}
	project, err := fetchProjectByID(ctx, projectID)
	if err != nil {
		return err
	}
	payload := map[string]any{"project_id": project.ID}
	if plan.ToWorkspace != "" {
		workspaceID, err := resolveWorkspaceID(ctx, plan.ToWorkspace)
		if err != nil {
			return err
		}
		payload["workspace_id"] = workspaceID
		if plan.Visibility != "" {
			payload["access"] = map[string]any{"visibility": plan.Visibility}
		}
		if ctx.Global.DryRun {
			return writeDryRun(ctx, "project move", payload)
		}
		if !yes && !ctx.Global.Force {
			fmt.Fprintf(ctx.Stdout, "Would move %q to workspace %q\n", project.Name, plan.ToWorkspace)
			fmt.Fprintln(ctx.Stdout, "Use --yes to confirm.")
			return nil
		}
		reqCtx, cancel := requestContext(ctx)
		moved, reqID, err := ctx.Client.MoveProjectToWorkspace(reqCtx, api.MoveProjectToWorkspaceInput{
			ProjectID:   project.ID,
			WorkspaceID: workspaceID,
			Visibility:  plan.Visibility,
		})
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		return writeProjectList(ctx, []api.Project{moved}, "")
	}
	if project.WorkspaceID == "" {
		return &CodeError{Code: exitConflict, Err: fmt.Errorf("project %q is already personal", project.Name)}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "project move", map[string]any{
			"project_id": project.ID,
			"personal":   true,
		})
	}
	if !yes && !ctx.Global.Force {
		fmt.Fprintf(ctx.Stdout, "Would move %q to personal\n", project.Name)
		fmt.Fprintln(ctx.Stdout, "Use --yes to confirm.")
		return nil
	}
	reqCtx, cancel := requestContext(ctx)
	moved, reqID, err := ctx.Client.MoveProjectToPersonal(reqCtx, project.ID)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeProjectList(ctx, []api.Project{moved}, "")
}

func fetchProjectByID(ctx *Context, id string) (api.Project, error) {
	var project api.Project
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Get(reqCtx, "/projects/"+id, nil, &project)
	cancel()
	if err != nil {
		return api.Project{}, err
	}
	setRequestID(ctx, reqID)
	return project, nil
}


func writeProjectList(ctx *Context, projects []api.Project, cursor string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, projects, output.Meta{RequestID: ctx.RequestID, Count: len(projects), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, projects)
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

func writeProjectCollaborators(ctx *Context, collaborators []api.Collaborator, cursor string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, collaborators, output.Meta{RequestID: ctx.RequestID, Count: len(collaborators), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		items := make([]any, 0, len(collaborators))
		for _, c := range collaborators {
			items = append(items, c)
		}
		return output.WriteNDJSON(ctx.Stdout, items)
	}
	rows := make([][]string, 0, len(collaborators))
	for _, c := range collaborators {
		rows = append(rows, []string{c.ID, c.Name, c.Email})
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Name", "Email"}, rows)
}
