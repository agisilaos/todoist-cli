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

func sectionCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
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
	fs := flag.NewFlagSet("section list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var project string
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		query.Set("project_id", id)
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var allSections []api.Section
	var next string
	for {
		var page api.Paginated[api.Section]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/sections", query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allSections = append(allSections, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	return writeSectionList(ctx, allSections, next)
}

func sectionAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("section add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var name string
	var project string
	var help bool
	fs.StringVar(&name, "name", "", "Section name")
	fs.StringVar(&project, "project", "", "Project")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	if name == "" || project == "" {
		printSectionHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--name and --project are required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	projectID, err := resolveProjectID(ctx, project)
	if err != nil {
		return err
	}
	body := map[string]any{"name": name, "project_id": projectID}
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
	fs := flag.NewFlagSet("section update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var name string
	var help bool
	fs.StringVar(&id, "id", "", "Section ID")
	fs.StringVar(&name, "name", "", "Section name")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSectionHelp(ctx.Stdout)
		return nil
	}
	if id == "" || name == "" {
		printSectionHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id and --name are required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{"name": name}
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
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !ctx.Global.Force && !ctx.Global.DryRun {
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
