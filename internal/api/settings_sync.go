package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type UserSettings struct {
	Timezone              string `json:"timezone"`
	TimeFormat            int    `json:"time_format"`
	DateFormat            int    `json:"date_format"`
	StartDay              int    `json:"start_day"`
	Theme                 int    `json:"theme"`
	AutoReminder          int    `json:"auto_reminder"`
	NextWeek              int    `json:"next_week"`
	StartPage             string `json:"start_page"`
	ReminderPush          bool   `json:"reminder_push"`
	ReminderDesktop       bool   `json:"reminder_desktop"`
	ReminderEmail         bool   `json:"reminder_email"`
	CompletedSoundDesktop bool   `json:"completed_sound_desktop"`
	CompletedSoundMobile  bool   `json:"completed_sound_mobile"`
}

type UpdateUserSettingsInput struct {
	Timezone              *string `json:"timezone,omitempty"`
	TimeFormat            *int    `json:"time_format,omitempty"`
	DateFormat            *int    `json:"date_format,omitempty"`
	StartDay              *int    `json:"start_day,omitempty"`
	Theme                 *int    `json:"theme,omitempty"`
	AutoReminder          *int    `json:"auto_reminder,omitempty"`
	NextWeek              *int    `json:"next_week,omitempty"`
	StartPage             *string `json:"start_page,omitempty"`
	ReminderPush          *bool   `json:"reminder_push,omitempty"`
	ReminderDesktop       *bool   `json:"reminder_desktop,omitempty"`
	ReminderEmail         *bool   `json:"reminder_email,omitempty"`
	CompletedSoundDesktop *bool   `json:"completed_sound_desktop,omitempty"`
	CompletedSoundMobile  *bool   `json:"completed_sound_mobile,omitempty"`
}

func (c *Client) FetchUserSettings(ctx context.Context) (UserSettings, string, error) {
	resp, requestID, err := c.syncRequest(ctx, map[string]string{
		"sync_token":     "*",
		"resource_types": `["user","user_settings"]`,
	})
	if err != nil {
		return UserSettings{}, requestID, err
	}
	user := mapAny(resp.ExtraData["user"])
	settings := mapAny(resp.ExtraData["user_settings"])
	return UserSettings{
		Timezone:              firstNonEmpty(strings.TrimSpace(fmt.Sprintf("%v", user["timezone"])), "UTC"),
		TimeFormat:            intAny(user["time_format"]),
		DateFormat:            intAny(user["date_format"]),
		StartDay:              intAny(user["start_day"]),
		Theme:                 firstNonZero(intAny(user["theme_id"]), intAny(user["theme"])),
		AutoReminder:          intAny(user["auto_reminder"]),
		NextWeek:              intAny(user["next_week"]),
		StartPage:             firstNonEmpty(strings.TrimSpace(fmt.Sprintf("%v", user["start_page"])), "today"),
		ReminderPush:          boolOrDefault(settings["reminder_push"], true),
		ReminderDesktop:       boolOrDefault(settings["reminder_desktop"], true),
		ReminderEmail:         boolOrDefault(settings["reminder_email"], false),
		CompletedSoundDesktop: boolOrDefault(settings["completed_sound_desktop"], true),
		CompletedSoundMobile:  boolOrDefault(settings["completed_sound_mobile"], true),
	}, requestID, nil
}

func (c *Client) UpdateUserSettings(ctx context.Context, in UpdateUserSettingsInput) (string, error) {
	commands := make([]map[string]any, 0, 2)

	userArgs := map[string]any{}
	if in.Timezone != nil {
		userArgs["timezone"] = strings.TrimSpace(*in.Timezone)
	}
	if in.TimeFormat != nil {
		userArgs["time_format"] = *in.TimeFormat
	}
	if in.DateFormat != nil {
		userArgs["date_format"] = *in.DateFormat
	}
	if in.StartDay != nil {
		userArgs["start_day"] = *in.StartDay
	}
	if in.Theme != nil {
		userArgs["theme_id"] = fmt.Sprintf("%d", *in.Theme)
	}
	if in.AutoReminder != nil {
		userArgs["auto_reminder"] = *in.AutoReminder
	}
	if in.NextWeek != nil {
		userArgs["next_week"] = *in.NextWeek
	}
	if in.StartPage != nil {
		userArgs["start_page"] = strings.TrimSpace(*in.StartPage)
	}
	if len(userArgs) > 0 {
		commands = append(commands, map[string]any{
			"type": "user_update",
			"uuid": NewRequestID(),
			"args": userArgs,
		})
	}

	settingsArgs := map[string]any{}
	if in.ReminderPush != nil {
		settingsArgs["reminder_push"] = *in.ReminderPush
	}
	if in.ReminderDesktop != nil {
		settingsArgs["reminder_desktop"] = *in.ReminderDesktop
	}
	if in.ReminderEmail != nil {
		settingsArgs["reminder_email"] = *in.ReminderEmail
	}
	if in.CompletedSoundDesktop != nil {
		settingsArgs["completed_sound_desktop"] = *in.CompletedSoundDesktop
	}
	if in.CompletedSoundMobile != nil {
		settingsArgs["completed_sound_mobile"] = *in.CompletedSoundMobile
	}
	if len(settingsArgs) > 0 {
		commands = append(commands, map[string]any{
			"type": "user_settings_update",
			"uuid": NewRequestID(),
			"args": settingsArgs,
		})
	}

	if len(commands) == 0 {
		return "", fmt.Errorf("no settings to update")
	}
	payload, err := json.Marshal(commands)
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func boolOrDefault(v any, fallback bool) bool {
	switch vv := v.(type) {
	case bool:
		return vv
	default:
		return fallback
	}
}
