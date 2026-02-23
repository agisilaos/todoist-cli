package cli

import (
	"errors"
	"fmt"

	"github.com/agisilaos/todoist-cli/internal/api"
	appsettings "github.com/agisilaos/todoist-cli/internal/app/settings"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func settingsCommand(ctx *Context, args []string) error {
	if len(args) == 0 {
		return settingsView(ctx, nil)
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printSettingsHelp(ctx.Stdout)
		return nil
	}
	switch canonicalSubcommand(args[0], nil) {
	case "view":
		return settingsView(ctx, args[1:])
	case "update":
		return settingsUpdate(ctx, args[1:])
	case "themes":
		return settingsThemes(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown settings subcommand: %s", args[0])}
	}
}

func settingsView(ctx *Context, args []string) error {
	fs := newFlagSet("settings view")
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSettingsHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	settings, reqID, err := ctx.Client.FetchUserSettings(reqCtx)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSettings(ctx, settings)
}

func settingsUpdate(ctx *Context, args []string) error {
	fs := newFlagSet("settings update")
	flags := appsettings.UpdateFlags{}
	var help bool
	fs.StringVar(&flags.Timezone, "timezone", "", "Timezone (for example UTC, Europe/London)")
	fs.StringVar(&flags.TimeFormat, "time-format", "", "Time format: 12 or 24")
	fs.StringVar(&flags.DateFormat, "date-format", "", "Date format: us or intl")
	fs.StringVar(&flags.StartDay, "start-day", "", "Week start day")
	fs.StringVar(&flags.Theme, "theme", "", "Theme name")
	fs.StringVar(&flags.AutoReminder, "auto-reminder", "", "Default reminder minutes")
	fs.StringVar(&flags.NextWeek, "next-week", "", "\"Next week\" day")
	fs.StringVar(&flags.StartPage, "start-page", "", "Start page")
	fs.StringVar(&flags.ReminderPush, "reminder-push", "", "Push reminders on/off")
	fs.StringVar(&flags.ReminderDesktop, "reminder-desktop", "", "Desktop reminders on/off")
	fs.StringVar(&flags.ReminderEmail, "reminder-email", "", "Email reminders on/off")
	fs.StringVar(&flags.CompletedSoundDesktop, "completed-sound-desktop", "", "Desktop completion sound on/off")
	fs.StringVar(&flags.CompletedSoundMobile, "completed-sound-mobile", "", "Mobile completion sound on/off")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSettingsHelp(ctx.Stdout)
		return nil
	}
	input, err := appsettings.ParseUpdateInput(flags)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if !appsettings.HasUpdate(input) {
		return &CodeError{Code: exitUsage, Err: errors.New("settings update requires at least one flag")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "settings update", input)
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.UpdateUserSettings(reqCtx, input)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "updated", "settings")
}

func settingsThemes(ctx *Context, args []string) error {
	fs := newFlagSet("settings themes")
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printSettingsHelp(ctx.Stdout)
		return nil
	}
	themes := appsettings.Themes()
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, themes, output.Meta{})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, themes)
	}
	rows := make([][]string, 0, len(themes))
	for _, theme := range themes {
		pro := "no"
		if theme.Pro {
			pro = "yes"
		}
		rows = append(rows, []string{theme.Name, theme.Label, pro})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"Name", "Label", "Pro"}, rows)
}

func writeSettings(ctx *Context, settings api.UserSettings) error {
	if ctx.Mode == output.ModeJSON {
		view := map[string]any{
			"timezone":                settings.Timezone,
			"time_format":             settings.TimeFormat,
			"date_format":             settings.DateFormat,
			"start_day":               appsettings.DayName(settings.StartDay),
			"theme":                   appsettings.ThemeName(settings.Theme),
			"auto_reminder":           settings.AutoReminder,
			"next_week":               appsettings.DayName(settings.NextWeek),
			"start_page":              settings.StartPage,
			"reminder_push":           settings.ReminderPush,
			"reminder_desktop":        settings.ReminderDesktop,
			"reminder_email":          settings.ReminderEmail,
			"completed_sound_desktop": settings.CompletedSoundDesktop,
			"completed_sound_mobile":  settings.CompletedSoundMobile,
		}
		return output.WriteJSON(ctx.Stdout, view, output.Meta{RequestID: ctx.RequestID})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, []api.UserSettings{settings})
	}
	rows := [][]string{
		{"Timezone", settings.Timezone},
		{"Time format", fmt.Sprintf("%d", settings.TimeFormat)},
		{"Date format", fmt.Sprintf("%d", settings.DateFormat)},
		{"Start day", appsettings.DayName(settings.StartDay)},
		{"Theme", appsettings.ThemeName(settings.Theme)},
		{"Auto reminder", fmt.Sprintf("%d", settings.AutoReminder)},
		{"Next week", appsettings.DayName(settings.NextWeek)},
		{"Start page", settings.StartPage},
		{"Reminder push", boolOnOff(settings.ReminderPush)},
		{"Reminder desktop", boolOnOff(settings.ReminderDesktop)},
		{"Reminder email", boolOnOff(settings.ReminderEmail)},
		{"Completed sound desktop", boolOnOff(settings.CompletedSoundDesktop)},
		{"Completed sound mobile", boolOnOff(settings.CompletedSoundMobile)},
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"Setting", "Value"}, rows)
}

func boolOnOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func printSettingsHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist settings
  todoist settings view
  todoist settings update [flags]
  todoist settings themes

Update flags:
  --timezone <tz>
  --time-format <12|24>
  --date-format <us|intl>
  --start-day <day>
  --theme <name>
  --auto-reminder <minutes>
  --next-week <day>
  --start-page <page>
  --reminder-push <on|off>
  --reminder-desktop <on|off>
  --reminder-email <on|off>
  --completed-sound-desktop <on|off>
  --completed-sound-mobile <on|off>

Examples:
  todoist settings
  todoist settings view --json
  todoist settings update --timezone UTC --time-format 24
  todoist settings themes
`)
}
