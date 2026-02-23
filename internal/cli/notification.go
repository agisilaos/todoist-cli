package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	appnotifications "github.com/agisilaos/todoist-cli/internal/app/notifications"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func notificationCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printNotificationHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls": "list",
	})
	switch sub {
	case "list":
		return notificationList(ctx, args[1:])
	case "view":
		return notificationView(ctx, args[1:])
	case "accept":
		return notificationAccept(ctx, args[1:])
	case "reject":
		return notificationReject(ctx, args[1:])
	case "read":
		return notificationRead(ctx, args[1:])
	case "unread":
		return notificationUnread(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown notification subcommand: %s", args[0])}
	}
}

func notificationList(ctx *Context, args []string) error {
	fs := newFlagSet("notification list")
	var typeFilter string
	var unread bool
	var read bool
	var limit int
	var offset int
	var help bool
	fs.StringVar(&typeFilter, "type", "", "Filter by notification type (comma-separated)")
	fs.BoolVar(&unread, "unread", false, "Only unread notifications")
	fs.BoolVar(&read, "read", false, "Only read notifications")
	fs.IntVar(&limit, "limit", 10, "Max notifications to show")
	fs.IntVar(&offset, "offset", 0, "Skip first N notifications")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printNotificationHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	items, reqID, err := ctx.Client.FetchLiveNotifications(reqCtx)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	out, err := appnotifications.List(items, appnotifications.ListInput{
		Type:   typeFilter,
		Unread: unread,
		Read:   read,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	return writeNotificationList(ctx, out)
}

func notificationView(ctx *Context, args []string) error {
	fs := newFlagSet("notification view")
	var id string
	var help bool
	fs.StringVar(&id, "id", "", "Notification ID")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printNotificationHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	id = stripIDPrefix(id)
	if strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("notification view requires --id or positional id")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	n, err := findNotificationByID(ctx, id)
	if err != nil {
		return err
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, n, output.Meta{RequestID: ctx.RequestID})
	}
	rows := [][]string{
		{"ID", n.ID},
		{"Type", n.Type},
		{"Status", map[bool]string{true: "unread", false: "read"}[n.IsUnread]},
		{"Created", n.CreatedAt},
		{"Project", n.ProjectName},
		{"Task", n.TaskContent},
		{"From", n.FromUserName},
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"Field", "Value"}, rows)
}

func notificationAccept(ctx *Context, args []string) error {
	return notificationInvitationAction(ctx, args, "accept")
}

func notificationReject(ctx *Context, args []string) error {
	return notificationInvitationAction(ctx, args, "reject")
}

func notificationInvitationAction(ctx *Context, args []string, action string) error {
	fs := newFlagSet("notification " + action)
	var id string
	var help bool
	fs.StringVar(&id, "id", "", "Notification ID")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printNotificationHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	id = stripIDPrefix(id)
	if strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("notification %s requires --id or positional id", action)}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	n, err := findNotificationByID(ctx, id)
	if err != nil {
		return err
	}
	if n.Type != "share_invitation_sent" {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("notification %s only supports share_invitation_sent (got %s)", action, n.Type)}
	}
	if strings.TrimSpace(n.InvitationID) == "" || strings.TrimSpace(n.InvitationSecret) == "" {
		return &CodeError{Code: exitError, Err: errors.New("notification is missing invitation_id/invitation_secret")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "notification "+action, map[string]any{"id": id, "invitation_id": n.InvitationID})
	}
	reqCtx, cancel := requestContext(ctx)
	var reqID string
	switch action {
	case "accept":
		reqID, err = ctx.Client.AcceptInvitation(reqCtx, n.InvitationID, n.InvitationSecret)
	case "reject":
		reqID, err = ctx.Client.RejectInvitation(reqCtx, n.InvitationID, n.InvitationSecret)
	default:
		cancel()
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported invitation action: %s", action)}
	}
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, action+"ed", id)
}

func findNotificationByID(ctx *Context, id string) (api.Notification, error) {
	reqCtx, cancel := requestContext(ctx)
	items, reqID, err := ctx.Client.FetchLiveNotifications(reqCtx)
	cancel()
	if err != nil {
		return api.Notification{}, err
	}
	setRequestID(ctx, reqID)
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return api.Notification{}, &CodeError{Code: exitNotFound, Err: fmt.Errorf("notification %q not found", id)}
}

func notificationRead(ctx *Context, args []string) error {
	fs := newFlagSet("notification read")
	var id string
	var all bool
	var yes bool
	var help bool
	fs.StringVar(&id, "id", "", "Notification ID")
	fs.BoolVar(&all, "all", false, "Mark all notifications as read")
	fs.BoolVar(&yes, "yes", false, "Confirm --all operation")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printNotificationHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	id = stripIDPrefix(id)
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if all {
		if !yes && !ctx.Global.Force && !ctx.Global.DryRun {
			return &CodeError{Code: exitUsage, Err: errors.New("notification read --all requires --yes (or --force)")}
		}
		if ctx.Global.DryRun {
			return writeDryRun(ctx, "notification read all", map[string]any{"all": true})
		}
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.MarkAllNotificationsRead(reqCtx)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		return writeSimpleResult(ctx, "read", "all")
	}
	if strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("notification read requires id or --all")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "notification read", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.MarkNotificationsRead(reqCtx, []string{id})
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "read", id)
}

func notificationUnread(ctx *Context, args []string) error {
	fs := newFlagSet("notification unread")
	var id string
	var help bool
	fs.StringVar(&id, "id", "", "Notification ID")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printNotificationHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	id = stripIDPrefix(id)
	if strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("notification unread requires --id or positional id")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "notification unread", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.MarkNotificationsUnread(reqCtx, []string{id})
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "unread", id)
}

func writeNotificationList(ctx *Context, out appnotifications.ListResult) error {
	items := out.Items
	if items == nil {
		items = []api.Notification{}
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, items, output.Meta{RequestID: ctx.RequestID, Count: len(items), Cursor: nextOffsetCursor(out)})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, items)
	}
	if len(items) == 0 {
		if ctx.Mode == output.ModeHuman {
			fmt.Fprintln(ctx.Stdout, "No notifications.")
		}
		return nil
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		status := "read"
		if item.IsUnread {
			status = "unread"
		}
		rows = append(rows, []string{item.ID, item.Type, status, item.CreatedAt})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	if err := output.WriteTable(ctx.Stdout, []string{"ID", "Type", "Status", "Created"}, rows); err != nil {
		return err
	}
	if out.HasMore {
		fmt.Fprintf(ctx.Stdout, "\nMore available. Use --offset %d\n", out.Offset+out.Limit)
	}
	return nil
}

func nextOffsetCursor(out appnotifications.ListResult) string {
	if !out.HasMore {
		return ""
	}
	return strconv.Itoa(out.Offset + out.Limit)
}
