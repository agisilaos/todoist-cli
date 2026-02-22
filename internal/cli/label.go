package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/api"
	applabels "github.com/agisilaos/todoist-cli/internal/app/labels"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func labelCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":  "list",
		"rm":  "delete",
		"del": "delete",
	})
	switch sub {
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
	fs := newFlagSet("label list")
	var cursor string
	var limit int
	var all bool
	var help bool
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	query := applabels.BuildListQuery(applabels.ListInput{Limit: limit, Cursor: cursor})
	allLabels, next, err := fetchPaginated[api.Label](ctx, "/labels", query, all)
	if err != nil {
		return err
	}
	return writeLabelList(ctx, allLabels, next)
}

func labelAdd(ctx *Context, args []string) error {
	fs := newFlagSet("label add")
	var name string
	var color string
	var order int
	var favorite bool
	var help bool
	fs.StringVar(&name, "name", "", "Label name")
	fs.StringVar(&color, "color", "", "Label color")
	fs.IntVar(&order, "order", 0, "Order")
	fs.BoolVar(&favorite, "favorite", false, "Favorite")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	body, err := applabels.BuildAddPayload(applabels.AddInput{
		Name:     name,
		Color:    color,
		Order:    order,
		Favorite: favorite,
	})
	if err != nil {
		printLabelHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
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
	fs := newFlagSet("label update")
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
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printLabelHelp(ctx.Stdout)
		return nil
	}
	id, body, err := applabels.BuildUpdatePayload(applabels.UpdateInput{
		ID:         id,
		Name:       name,
		Color:      color,
		Order:      order,
		Favorite:   favorite,
		Unfavorite: unfavorite,
	})
	if err != nil {
		printLabelHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: err}
	}
	id, directID, err := apprefs.NormalizeEntityRef(id, "label")
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if !directID || id == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("label update requires --id")}
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
	id, err := requireEntityIDArg("label delete", "label", args)
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
		return output.WriteNDJSONSlice(ctx.Stdout, labels)
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
