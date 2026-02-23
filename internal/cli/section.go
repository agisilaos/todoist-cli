package cli

import (
	"fmt"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/api"
	appsections "github.com/agisilaos/todoist-cli/internal/app/sections"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func sectionCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":  "list",
		"rm":  "delete",
		"del": "delete",
	})
	switch sub {
	case "list":
		return sectionList(ctx, args[1:])
	case "add":
		return sectionAdd(ctx, args[1:])
	case "update":
		return sectionUpdate(ctx, args[1:])
	case "delete":
		return sectionDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown section subcommand: %s", args[0])}
	}
}

func sectionList(ctx *Context, args []string) error {
	fs := newFlagSet("section list")
	var project string
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	resolvedProjectID := ""
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		resolvedProjectID = id
	}
	query := appsections.BuildListQuery(appsections.ListInput{
		Limit:     limit,
		Cursor:    cursor,
		ProjectID: resolvedProjectID,
	})
	allSections, next, err := fetchPaginated[api.Section](ctx, "/sections", query, all)
	if err != nil {
		return err
	}
	return writeSectionList(ctx, allSections, next)
}

func sectionAdd(ctx *Context, args []string) error {
	fs := newFlagSet("section add")
	var name string
	var project string
	var help bool
	fs.StringVar(&name, "name", "", "Section name")
	fs.StringVar(&project, "project", "", "Project")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	projectID, err := resolveProjectID(ctx, project)
	if err != nil {
		return err
	}
	body, err := appsections.BuildAddPayload(appsections.AddInput{Name: name, ProjectID: projectID})
	if err != nil {
		printSectionHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "section add", body)
	}
	var section api.Section
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/sections", nil, body, &section, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSectionList(ctx, []api.Section{section}, "")
}

func sectionUpdate(ctx *Context, args []string) error {
	fs := newFlagSet("section update")
	var id string
	var name string
	var help bool
	fs.StringVar(&id, "id", "", "Section ID")
	fs.StringVar(&name, "name", "", "Section name")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	id, body, err := appsections.BuildUpdatePayload(appsections.UpdateInput{ID: id, Name: name})
	if err != nil {
		printSectionHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "section update", body)
	}
	var section api.Section
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/sections/"+id, nil, body, &section, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSectionList(ctx, []api.Section{section}, "")
}

func sectionDelete(ctx *Context, args []string) error {
	id, err := requireIDArg("section delete", args)
	if err != nil {
		printSectionHelp(ctx.Stderr)
		return err
	}
	id, requiresConfirm, err := appsections.BuildDeletePlan(appsections.DeleteInput{
		ID:     id,
		Force:  ctx.Global.Force,
		DryRun: ctx.Global.DryRun,
	})
	if err != nil {
		printSectionHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if requiresConfirm {
		ok, err := confirm(ctx, fmt.Sprintf("Delete section %s?", id))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "section delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/sections/"+id, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}

func writeSectionList(ctx *Context, sections []api.Section, cursor string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, sections, output.Meta{RequestID: ctx.RequestID, Count: len(sections), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, sections)
	}
	rows := make([][]string, 0, len(sections))
	for _, section := range sections {
		rows = append(rows, []string{
			section.ID,
			section.Name,
			section.ProjectID,
			strconv.FormatBool(section.IsArchived),
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Name", "Project", "Archived"}, rows)
}
