package cli

import (
	"errors"
	"fmt"
	"strings"

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
	startPageName := resolveStartPageName(ctx, settings.StartPage)
	return writeSettings(ctx, settings, startPageName)
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

func writeSettings(ctx *Context, settings api.UserSettings, startPageName string) error {
	if ctx.Mode == output.ModeJSON {
		view := map[string]any{
			"timezone":                settings.Timezone,
			"time_format":             appsettings.TimeFormatLabel(settings.TimeFormat),
			"date_format":             appsettings.DateFormatLabel(settings.DateFormat),
			"start_day":               appsettings.DayName(settings.StartDay),
			"theme":                   appsettings.ThemeName(settings.Theme),
			"theme_label":             appsettings.ThemeLabel(settings.Theme),
			"auto_reminder":           settings.AutoReminder,
			"auto_reminder_label":     appsettings.AutoReminderLabel(settings.AutoReminder),
			"next_week":               appsettings.DayName(settings.NextWeek),
			"start_page":              settings.StartPage,
			"start_page_name":         startPageName,
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
		{"Time format", appsettings.TimeFormatLabel(settings.TimeFormat)},
		{"Date format", appsettings.DateFormatLabel(settings.DateFormat)},
		{"Start day", appsettings.DayLabel(settings.StartDay)},
		{"Theme", appsettings.ThemeLabel(settings.Theme)},
		{"Auto reminder", appsettings.AutoReminderLabel(settings.AutoReminder)},
		{"Next week", appsettings.DayLabel(settings.NextWeek)},
		{"Start page", withResolvedName(settings.StartPage, startPageName)},
		{"Reminder push", boolOnOff(settings.ReminderPush)},
		{"Reminder desktop", boolOnOff(settings.ReminderDesktop)},
		{"Reminder email", boolOnOff(settings.ReminderEmail)},
		{"Completed sound desktop", boolOnOff(settings.CompletedSoundDesktop)},
		{"Completed sound mobile", boolOnOff(settings.CompletedSoundMobile)},
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	fmt.Fprintln(ctx.Stdout, "General")
	if err := output.WriteTable(ctx.Stdout, []string{"Setting", "Value"}, rows[:8]); err != nil {
		return err
	}
	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, "Notifications")
	return output.WriteTable(ctx.Stdout, []string{"Setting", "Value"}, rows[8:])
}

func boolOnOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func withResolvedName(raw, name string) string {
	if strings.TrimSpace(name) == "" {
		return raw
	}
	return fmt.Sprintf("%s (%s)", raw, name)
}

func resolveStartPageName(ctx *Context, startPage string) string {
	refType, refID := parseStartPageRef(startPage)
	if refType == "" || refID == "" {
		return ""
	}
	switch refType {
	case "project":
		var project api.Project
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Get(reqCtx, "/projects/"+refID, nil, &project)
		cancel()
		if err == nil {
			return strings.TrimSpace(project.Name)
		}
	case "label":
		var label api.Label
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Get(reqCtx, "/labels/"+refID, nil, &label)
		cancel()
		if err == nil {
			return strings.TrimSpace(label.Name)
		}
	case "filter":
		filters, _, err := listAllFilters(ctx)
		if err != nil {
			return ""
		}
		for _, filter := range filters {
			if filter.ID == refID {
				return strings.TrimSpace(filter.Name)
			}
		}
	}
	return ""
}

func parseStartPageRef(startPage string) (string, string) {
	pieces := strings.SplitN(strings.TrimSpace(startPage), "?", 2)
	if len(pieces) != 2 {
		return "", ""
	}
	refType := strings.ToLower(strings.TrimSpace(pieces[0]))
	if refType != "project" && refType != "label" && refType != "filter" {
		return "", ""
	}
	for _, pair := range strings.Split(pieces[1], "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if strings.TrimSpace(kv[0]) == "id" {
			return refType, strings.TrimSpace(kv[1])
		}
	}
	return "", ""
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
