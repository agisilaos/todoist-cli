package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	appreminders "github.com/agisilaos/todoist-cli/internal/app/reminders"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func reminderCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printReminderHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":  "list",
		"rm":  "delete",
		"del": "delete",
	})
	switch sub {
	case "list":
		return reminderList(ctx, args[1:])
	case "add":
		return reminderAdd(ctx, args[1:])
	case "update":
		return reminderUpdate(ctx, args[1:])
	case "delete":
		return reminderDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown reminder subcommand: %s", args[0])}
	}
}

func reminderList(ctx *Context, args []string) error {
	fs := newFlagSet("reminder list")
	var task string
	var help bool
	fs.StringVar(&task, "task", "", "Task reference")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printReminderHelp(ctx.Stdout)
		return nil
	}
	if task == "" && len(fs.Args()) > 0 {
		task = strings.Join(fs.Args(), " ")
	}
	if strings.TrimSpace(task) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("reminder list requires --task or positional task reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	taskObj, err := resolveTaskRef(ctx, task)
	if err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	reminders, reqID, err := ctx.Client.FetchReminders(reqCtx)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	filtered := make([]api.Reminder, 0, len(reminders))
	for _, reminder := range reminders {
		if reminder.ItemID == taskObj.ID {
			filtered = append(filtered, reminder)
		}
	}
	return writeReminderList(ctx, filtered)
}

func reminderAdd(ctx *Context, args []string) error {
	fs := newFlagSet("reminder add")
	var task string
	var before string
	var at string
	var help bool
	fs.StringVar(&task, "task", "", "Task reference")
	fs.StringVar(&before, "before", "", "Reminder offset before due (e.g. 30m, 1h)")
	fs.StringVar(&at, "at", "", "Reminder datetime (RFC3339 or YYYY-MM-DD HH:MM)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printReminderHelp(ctx.Stdout)
		return nil
	}
	if task == "" && len(fs.Args()) > 0 {
		task = strings.Join(fs.Args(), " ")
	}
	if strings.TrimSpace(task) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("reminder add requires --task or positional task reference")}
	}
	if err := appreminders.ValidateTimeChoice(before, at); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	taskObj, err := resolveTaskRef(ctx, task)
	if err != nil {
		return err
	}
	input := api.ReminderAddInput{ItemID: taskObj.ID}
	if strings.TrimSpace(before) != "" {
		mins, err := appreminders.ParseBeforeMinutes(before)
		if err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		input.MinuteOffset = mins
	}
	if strings.TrimSpace(at) != "" {
		atDate, err := appreminders.ParseAtDate(at)
		if err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		input.Due = &api.ReminderDue{Date: atDate}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "reminder add", input)
	}
	reqCtx, cancel := requestContext(ctx)
	id, reqID, err := ctx.Client.AddReminder(reqCtx, input)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "created", id)
}

func reminderUpdate(ctx *Context, args []string) error {
	fs := newFlagSet("reminder update")
	var id string
	var before string
	var at string
	var help bool
	fs.StringVar(&id, "id", "", "Reminder ID")
	fs.StringVar(&before, "before", "", "Reminder offset before due (e.g. 30m, 1h)")
	fs.StringVar(&at, "at", "", "Reminder datetime (RFC3339 or YYYY-MM-DD HH:MM)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printReminderHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	id = stripIDPrefix(id)
	if strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("reminder update requires --id or positional id")}
	}
	if err := appreminders.ValidateTimeChoice(before, at); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	input := api.ReminderUpdateInput{ID: id}
	if strings.TrimSpace(before) != "" {
		mins, err := appreminders.ParseBeforeMinutes(before)
		if err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		input.MinuteOffset = mins
	}
	if strings.TrimSpace(at) != "" {
		atDate, err := appreminders.ParseAtDate(at)
		if err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		input.Due = &api.ReminderDue{Date: atDate}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "reminder update", input)
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.UpdateReminder(reqCtx, input)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "updated", id)
}

func reminderDelete(ctx *Context, args []string) error {
	fs := newFlagSet("reminder delete")
	var id string
	var yes bool
	var help bool
	fs.StringVar(&id, "id", "", "Reminder ID")
	fs.BoolVar(&yes, "yes", false, "Skip confirmation")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printReminderHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		id = fs.Arg(0)
	}
	id = stripIDPrefix(id)
	if strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("reminder delete requires --id or positional id")}
	}
	if !yes && !ctx.Global.Force && !ctx.Global.DryRun {
		return &CodeError{Code: exitUsage, Err: errors.New("reminder delete requires --yes (or --force)")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "reminder delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.DeleteReminder(reqCtx, id)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
}

func writeReminderList(ctx *Context, reminders []api.Reminder) error {
	if reminders == nil {
		reminders = []api.Reminder{}
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, reminders, output.Meta{RequestID: ctx.RequestID, Count: len(reminders)})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, reminders)
	}
	rows := make([][]string, 0, len(reminders))
	for _, reminder := range reminders {
		when := ""
		if reminder.MinuteOffset > 0 {
			when = fmt.Sprintf("%dm before due", reminder.MinuteOffset)
		} else if reminder.Due != nil {
			when = reminder.Due.Date
		}
		rows = append(rows, []string{reminder.ID, reminder.ItemID, when})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Task", "When"}, rows)
}
