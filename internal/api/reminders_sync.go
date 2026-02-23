package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ReminderDue struct {
	Date string `json:"date"`
}

type Reminder struct {
	ID           string       `json:"id"`
	ItemID       string       `json:"item_id"`
	Type         string       `json:"type"`
	Due          *ReminderDue `json:"due,omitempty"`
	MinuteOffset int          `json:"minute_offset,omitempty"`
	IsDeleted    bool         `json:"is_deleted"`
}

type reminderSyncResponse struct {
	Reminders     []Reminder        `json:"reminders"`
	TempIDMapping map[string]string `json:"temp_id_mapping"`
	SyncStatus    map[string]any    `json:"sync_status"`
	Error         string            `json:"error"`
	ErrorTag      string            `json:"error_tag"`
	FullSync      bool              `json:"full_sync"`
	ExtraData     map[string]any    `json:"-"`
}

type ReminderAddInput struct {
	TempID       string
	ItemID       string
	MinuteOffset int
	Due          *ReminderDue
}

type ReminderUpdateInput struct {
	ID           string
	MinuteOffset int
	Due          *ReminderDue
}

func (c *Client) FetchReminders(ctx context.Context) ([]Reminder, string, error) {
	resp, requestID, err := c.syncRequest(ctx, map[string]string{
		"sync_token":     "*",
		"resource_types": `["reminders"]`,
	})
	if err != nil {
		return nil, requestID, err
	}
	out := make([]Reminder, 0, len(resp.Reminders))
	for _, reminder := range resp.Reminders {
		if reminder.IsDeleted {
			continue
		}
		out = append(out, reminder)
	}
	return out, requestID, nil
}

func (c *Client) AddReminder(ctx context.Context, in ReminderAddInput) (string, string, error) {
	if strings.TrimSpace(in.ItemID) == "" {
		return "", "", fmt.Errorf("item_id is required")
	}
	tempID := strings.TrimSpace(in.TempID)
	if tempID == "" {
		tempID = NewRequestID()
	}
	args := map[string]any{
		"item_id": in.ItemID,
		"type":    "absolute",
	}
	if in.MinuteOffset > 0 {
		args["minute_offset"] = in.MinuteOffset
	}
	if in.Due != nil && strings.TrimSpace(in.Due.Date) != "" {
		args["due"] = map[string]any{"date": in.Due.Date}
	}
	commands := []map[string]any{{
		"type":    "reminder_add",
		"uuid":    NewRequestID(),
		"temp_id": tempID,
		"args":    args,
	}}
	payload, err := json.Marshal(commands)
	if err != nil {
		return "", "", err
	}
	resp, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	if err != nil {
		return "", requestID, err
	}
	if mapped := strings.TrimSpace(resp.TempIDMapping[tempID]); mapped != "" {
		return mapped, requestID, nil
	}
	return tempID, requestID, nil
}

func (c *Client) UpdateReminder(ctx context.Context, in ReminderUpdateInput) (string, error) {
	if strings.TrimSpace(in.ID) == "" {
		return "", fmt.Errorf("id is required")
	}
	args := map[string]any{"id": in.ID}
	if in.MinuteOffset > 0 {
		args["minute_offset"] = in.MinuteOffset
	}
	if in.Due != nil && strings.TrimSpace(in.Due.Date) != "" {
		args["due"] = map[string]any{"date": in.Due.Date}
	}
	payload, err := json.Marshal([]map[string]any{{
		"type": "reminder_update",
		"uuid": NewRequestID(),
		"args": args,
	}})
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func (c *Client) DeleteReminder(ctx context.Context, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("id is required")
	}
	payload, err := json.Marshal([]map[string]any{{
		"type": "reminder_delete",
		"uuid": NewRequestID(),
		"args": map[string]any{"id": id},
	}})
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func (c *Client) syncRequest(ctx context.Context, formValues map[string]string) (reminderSyncResponse, string, error) {
	fullURL, err := c.buildURL("/sync", nil)
	if err != nil {
		return reminderSyncResponse{}, "", err
	}
	requestID := NewRequestID()
	form := url.Values{}
	for key, value := range formValues {
		form.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return reminderSyncResponse{}, requestID, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Request-Id", requestID)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return reminderSyncResponse{}, requestID, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode >= 400 {
		return reminderSyncResponse{}, requestID, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(data)), RequestID: requestID}
	}
	var payload reminderSyncResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return reminderSyncResponse{}, requestID, fmt.Errorf("decode sync response: %w", err)
	}
	if payload.Error != "" {
		msg := payload.Error
		if payload.ErrorTag != "" {
			msg = payload.ErrorTag + ": " + payload.Error
		}
		return reminderSyncResponse{}, requestID, &APIError{Status: 400, Message: msg, RequestID: requestID}
	}
	return payload, requestID, nil
}
