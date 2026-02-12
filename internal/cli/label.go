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

func labelCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return labelList(ctx, args[1:])
	case "add":
		return labelAdd(ctx, args[1:])
	case "update":
		return labelUpdate(ctx, args[1:])
	case "delete":
		return labelDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown label subcommand: %s", args[0])}
	}
}

func labelList(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("label list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var allLabels []api.Label
	var next string
	for {
		var page api.Paginated[api.Label]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/labels", query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allLabels = append(allLabels, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	return writeLabelList(ctx, allLabels, next)
}

func labelAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("label add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var name string
	var color string
	var order int
	var favorite bool
	var help bool
	fs.StringVar(&name, "name", "", "Label name")
	fs.StringVar(&color, "color", "", "Label color")
	fs.IntVar(&order, "order", 0, "Order")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	if name == "" {
		printLabelHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--name is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{"name": name}
	if color != "" {
		body["color"] = color
	}
	if order > 0 {
		body["order"] = order
	}
	if favorite {
		body["is_favorite"] = true
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "label add", body)
	}
	var label api.Label
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/labels", nil, body, &label, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeLabelList(ctx, []api.Label{label}, "")
}

func labelUpdate(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("label update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var name string
	var color string
	var order int
	var favorite bool
	var unfavorite bool
	var help bool
	fs.StringVar(&id, "id", "", "Label ID")
	fs.StringVar(&name, "name", "", "Label name")
	fs.StringVar(&color, "color", "", "Label color")
	fs.IntVar(&order, "order", 0, "Order")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	fs.BoolVar(&unfavorite, "unfavorite", false, "Unfavorite")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	if id == "" {
		printLabelHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id is required")}
	}
	if favorite && unfavorite {
		printLabelHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--favorite and --unfavorite are mutually exclusive")}
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if color != "" {
		body["color"] = color
	}
	if order > 0 {
		body["order"] = order
	}
	if favorite {
		body["is_favorite"] = true
	}
	if unfavorite {
		body["is_favorite"] = false
	}
	if len(body) == 0 {
		printLabelHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("no fields to update")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "label update", body)
	}
	var label api.Label
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/labels/"+id, nil, body, &label, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeLabelList(ctx, []api.Label{label}, "")
}

func labelDelete(ctx *Context, args []string) error {
	id, err := requireIDArg("label delete", args)
	if err != nil {
		printLabelHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !ctx.Global.Force && !ctx.Global.DryRun {
		ok, err := confirm(ctx, fmt.Sprintf("Delete label %s?", id))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "label delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/labels/"+id, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}

func writeLabelList(ctx *Context, labels []api.Label, cursor string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, labels, output.Meta{RequestID: ctx.RequestID, Count: len(labels), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		items := make([]any, 0, len(labels))
		for _, label := range labels {
			items = append(items, label)
		}
		return output.WriteNDJSON(ctx.Stdout, items)
	}
	rows := make([][]string, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, []string{
			label.ID,
			label.Name,
			label.Color,
			strconv.FormatBool(label.IsFavorite),
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Name", "Color", "Favorite"}, rows)
}
