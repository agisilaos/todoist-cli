package settings

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type Theme struct {
	ID    int
	Name  string
	Label string
	Pro   bool
}

var themes = []Theme{
	{ID: 0, Name: "todoist", Label: "Todoist", Pro: false},
	{ID: 11, Name: "dark", Label: "Dark", Pro: false},
	{ID: 2, Name: "moonstone", Label: "Moonstone", Pro: false},
	{ID: 3, Name: "tangerine", Label: "Tangerine", Pro: false},
	{ID: 5, Name: "kale", Label: "Kale", Pro: true},
	{ID: 6, Name: "blueberry", Label: "Blueberry", Pro: true},
	{ID: 8, Name: "lavender", Label: "Lavender", Pro: true},
	{ID: 12, Name: "raspberry", Label: "Raspberry", Pro: true},
}

var dayMap = map[string]int{
	"monday": 1, "mon": 1,
	"tuesday": 2, "tue": 2,
	"wednesday": 3, "wed": 3,
	"thursday": 4, "thu": 4,
	"friday": 5, "fri": 5,
	"saturday": 6, "sat": 6,
	"sunday": 7, "sun": 7,
}

var timeFormatMap = map[string]int{
	"12":  1,
	"12h": 1,
	"24":  0,
	"24h": 0,
}

var timeFormatDisplay = map[int]string{
	0: "24h",
	1: "12h",
}

var dateFormatMap = map[string]int{
	"us":         1,
	"mm-dd-yyyy": 1,
	"mdy":        1,
	"intl":       0,
	"dd-mm-yyyy": 0,
	"dmy":        0,
}

var dateFormatDisplay = map[int]string{
	0: "DD-MM-YYYY",
	1: "MM-DD-YYYY",
}

type UpdateFlags struct {
	Timezone              string
	TimeFormat            string
	DateFormat            string
	StartDay              string
	Theme                 string
	AutoReminder          string
	NextWeek              string
	StartPage             string
	ReminderPush          string
	ReminderDesktop       string
	ReminderEmail         string
	CompletedSoundDesktop string
	CompletedSoundMobile  string
}

func Themes() []Theme {
	out := make([]Theme, len(themes))
	copy(out, themes)
	return out
}

func ParseUpdateInput(flags UpdateFlags) (api.UpdateUserSettingsInput, error) {
	var out api.UpdateUserSettingsInput
	if value := strings.TrimSpace(flags.Timezone); value != "" {
		out.Timezone = &value
	}
	if value := strings.TrimSpace(flags.TimeFormat); value != "" {
		n, ok := timeFormatMap[strings.ToLower(value)]
		if !ok {
			return out, fmt.Errorf("invalid time format: %q", flags.TimeFormat)
		}
		out.TimeFormat = &n
	}
	if value := strings.TrimSpace(flags.DateFormat); value != "" {
		n, ok := dateFormatMap[strings.ToLower(value)]
		if !ok {
			return out, fmt.Errorf("invalid date format: %q", flags.DateFormat)
		}
		out.DateFormat = &n
	}
	if value := strings.TrimSpace(flags.StartDay); value != "" {
		n, ok := dayMap[strings.ToLower(value)]
		if !ok {
			return out, fmt.Errorf("invalid start day: %q", flags.StartDay)
		}
		out.StartDay = &n
	}
	if value := strings.TrimSpace(flags.Theme); value != "" {
		themeID, err := parseThemeID(value)
		if err != nil {
			return out, err
		}
		out.Theme = &themeID
	}
	if value := strings.TrimSpace(flags.AutoReminder); value != "" {
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes < 0 {
			return out, errors.New("auto reminder must be a non-negative integer")
		}
		out.AutoReminder = &minutes
	}
	if value := strings.TrimSpace(flags.NextWeek); value != "" {
		n, ok := dayMap[strings.ToLower(value)]
		if !ok {
			return out, fmt.Errorf("invalid next week day: %q", flags.NextWeek)
		}
		out.NextWeek = &n
	}
	if value := strings.TrimSpace(flags.StartPage); value != "" {
		out.StartPage = &value
	}
	if value := strings.TrimSpace(flags.ReminderPush); value != "" {
		b, err := parseBool(value)
		if err != nil {
			return out, fmt.Errorf("invalid reminder-push value: %q", flags.ReminderPush)
		}
		out.ReminderPush = &b
	}
	if value := strings.TrimSpace(flags.ReminderDesktop); value != "" {
		b, err := parseBool(value)
		if err != nil {
			return out, fmt.Errorf("invalid reminder-desktop value: %q", flags.ReminderDesktop)
		}
		out.ReminderDesktop = &b
	}
	if value := strings.TrimSpace(flags.ReminderEmail); value != "" {
		b, err := parseBool(value)
		if err != nil {
			return out, fmt.Errorf("invalid reminder-email value: %q", flags.ReminderEmail)
		}
		out.ReminderEmail = &b
	}
	if value := strings.TrimSpace(flags.CompletedSoundDesktop); value != "" {
		b, err := parseBool(value)
		if err != nil {
			return out, fmt.Errorf("invalid completed-sound-desktop value: %q", flags.CompletedSoundDesktop)
		}
		out.CompletedSoundDesktop = &b
	}
	if value := strings.TrimSpace(flags.CompletedSoundMobile); value != "" {
		b, err := parseBool(value)
		if err != nil {
			return out, fmt.Errorf("invalid completed-sound-mobile value: %q", flags.CompletedSoundMobile)
		}
		out.CompletedSoundMobile = &b
	}
	return out, nil
}

func HasUpdate(input api.UpdateUserSettingsInput) bool {
	return input.Timezone != nil ||
		input.TimeFormat != nil ||
		input.DateFormat != nil ||
		input.StartDay != nil ||
		input.Theme != nil ||
		input.AutoReminder != nil ||
		input.NextWeek != nil ||
		input.StartPage != nil ||
		input.ReminderPush != nil ||
		input.ReminderDesktop != nil ||
		input.ReminderEmail != nil ||
		input.CompletedSoundDesktop != nil ||
		input.CompletedSoundMobile != nil
}

func DayName(day int) string {
	for name, value := range dayMap {
		if len(name) == 3 {
			continue
		}
		if value == day {
			return name
		}
	}
	return strconv.Itoa(day)
}

func DayLabel(day int) string {
	name := DayName(day)
	if len(name) == 0 {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func ThemeName(themeID int) string {
	for _, theme := range themes {
		if theme.ID == themeID {
			return theme.Name
		}
	}
	return strconv.Itoa(themeID)
}

func ThemeLabel(themeID int) string {
	for _, theme := range themes {
		if theme.ID == themeID {
			if theme.Pro {
				return theme.Label + " (Pro)"
			}
			return theme.Label
		}
	}
	return strconv.Itoa(themeID)
}

func TimeFormatLabel(value int) string {
	if out, ok := timeFormatDisplay[value]; ok {
		return out
	}
	return strconv.Itoa(value)
}

func DateFormatLabel(value int) string {
	if out, ok := dateFormatDisplay[value]; ok {
		return out
	}
	return strconv.Itoa(value)
}

func AutoReminderLabel(minutes int) string {
	if minutes <= 0 {
		return "none"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	hours := minutes / 60
	rest := minutes % 60
	if rest == 0 {
		return fmt.Sprintf("%d hr", hours)
	}
	return fmt.Sprintf("%d hr %d min", hours, rest)
}

func parseThemeID(value string) (int, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, theme := range themes {
		if theme.Name == normalized {
			return theme.ID, nil
		}
	}
	return 0, fmt.Errorf("invalid theme: %q", value)
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "on", "yes", "1":
		return true, nil
	case "false", "off", "no", "0":
		return false, nil
	default:
		return false, errors.New("invalid bool")
	}
}
