package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func filterCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printFilterHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":  "list",
		"rm":  "delete",
		"del": "delete",
	})
	switch sub {
	case "list":
		return filterList(ctx, args[1:])
	case "show":
		return filterShow(ctx, args[1:])
	case "add":
		return filterAdd(ctx, args[1:])
	case "update":
		return filterUpdate(ctx, args[1:])
	case "delete":
		return filterDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown filter subcommand: %s", args[0])}
	}
}

func filterList(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("filter list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var help bool
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printFilterHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	var filters []api.Filter
	reqID, err := ctx.Client.Get(reqCtx, "/filters", nil, &filters)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeFilterList(ctx, filters)
}

func filterShow(ctx *Context, args []string) error {
	ref := strings.TrimSpace(strings.Join(args, " "))
	if ref == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("filter show requires a filter reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	filter, err := resolveFilterRef(ctx, ref)
	if err != nil {
		return err
	}
	tasks, _, err := listTasksByFilter(ctx, filter.Query, "", 50, true)
	if err != nil {
		return err
	}
	return writeTaskList(ctx, tasks, "", false)
}

func filterAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("filter add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var name string
	var queryStr string
	var color string
	var favorite bool
	var help bool
	fs.StringVar(&name, "name", "", "Filter name")
	fs.StringVar(&queryStr, "query", "", "Filter query")
	fs.StringVar(&color, "color", "", "Color")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printFilterHelp(ctx.Stdout)
		return nil
	}
	if name == "" || queryStr == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("--name and --query are required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{"name": name, "query": queryStr}
	if color != "" {
		body["color"] = color
	}
	if favorite {
		body["is_favorite"] = true
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "filter add", body)
	}
	var filter api.Filter
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/filters", nil, body, &filter, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeFilterList(ctx, []api.Filter{filter})
}

func filterUpdate(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("filter update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var ref string
	var name string
	var queryStr string
	var color string
	var favorite bool
	var unfavorite bool
	var help bool
	fs.StringVar(&ref, "id", "", "Filter ID or name")
	fs.StringVar(&name, "name", "", "Filter name")
	fs.StringVar(&queryStr, "query", "", "Filter query")
	fs.StringVar(&color, "color", "", "Color")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	fs.BoolVar(&unfavorite, "unfavorite", false, "Unfavorite")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printFilterHelp(ctx.Stdout)
		return nil
	}
	if ref == "" && len(fs.Args()) > 0 {
		ref = strings.Join(fs.Args(), " ")
	}
	if ref == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("filter update requires --id or a filter reference")}
	}
	if favorite && unfavorite {
		return &CodeError{Code: exitUsage, Err: errors.New("--favorite and --unfavorite are mutually exclusive")}
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if queryStr != "" {
		body["query"] = queryStr
	}
	if color != "" {
		body["color"] = color
	}
	if favorite {
		body["is_favorite"] = true
	}
	if unfavorite {
		body["is_favorite"] = false
	}
	if len(body) == 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("no fields to update")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	filter, err := resolveFilterRef(ctx, ref)
	if err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "filter update", map[string]any{"id": filter.ID, "payload": body})
	}
	var out api.Filter
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/filters/"+filter.ID, nil, body, &out, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeFilterList(ctx, []api.Filter{out})
}

func filterDelete(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("filter delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var ref string
	var yes bool
	var help bool
	fs.StringVar(&ref, "id", "", "Filter ID or name")
	fs.BoolVar(&yes, "yes", false, "Skip confirmation")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printFilterHelp(ctx.Stdout)
		return nil
	}
	if ref == "" && len(fs.Args()) > 0 {
		ref = strings.Join(fs.Args(), " ")
	}
	if ref == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("filter delete requires --id or a filter reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	filter, err := resolveFilterRef(ctx, ref)
	if err != nil {
		return err
	}
	if !yes && !ctx.Global.Force {
		return &CodeError{Code: exitUsage, Err: errors.New("filter delete requires --yes")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "filter delete", map[string]any{"id": filter.ID})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/filters/"+filter.ID, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", filter.ID)
}

func resolveFilterRef(ctx *Context, ref string) (api.Filter, error) {
	ref = strings.TrimSpace(stripIDPrefix(ref))
	reqCtx, cancel := requestContext(ctx)
	var filters []api.Filter
	reqID, err := ctx.Client.Get(reqCtx, "/filters", nil, &filters)
	cancel()
	if err != nil {
		return api.Filter{}, err
	}
	setRequestID(ctx, reqID)
	for _, f := range filters {
		if strings.EqualFold(f.ID, ref) || strings.EqualFold(f.Name, ref) {
			return f, nil
		}
	}
	candidates := fuzzyCandidates(ref, filters, func(f api.Filter) string { return f.Name }, func(f api.Filter) string { return f.ID })
	if len(candidates) == 1 {
		for _, f := range filters {
			if f.ID == candidates[0].ID {
				return f, nil
			}
		}
	}
	if len(candidates) > 1 {
		return api.Filter{}, ambiguousMatchCodeError("filter", ref, candidates)
	}
	return api.Filter{}, &CodeError{Code: exitNotFound, Err: fmt.Errorf("filter %q not found", ref)}
}

func writeFilterList(ctx *Context, filters []api.Filter) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, filters, output.Meta{RequestID: ctx.RequestID, Count: len(filters)})
	}
	if ctx.Mode == output.ModeNDJSON {
		items := make([]any, 0, len(filters))
		for _, f := range filters {
			items = append(items, f)
		}
		return output.WriteNDJSON(ctx.Stdout, items)
	}
	rows := make([][]string, 0, len(filters))
	for _, f := range filters {
		fav := ""
		if f.IsFavorite {
			fav = "yes"
		}
		rows = append(rows, []string{f.ID, f.Name, f.Query, f.Color, fav})
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Name", "Query", "Color", "Favorite"}, rows)
}
