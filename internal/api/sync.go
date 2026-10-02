package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type syncResponse struct {
	Workspaces    []Workspace       `json:"workspaces"`
	User          map[string]any    `json:"user"`
	Filters       []filterSyncItem  `json:"filters"`
	Reminders     []Reminder        `json:"reminders"`
	TempIDMapping map[string]string `json:"temp_id_mapping"`
	SyncStatus    map[string]any    `json:"sync_status"`
	Error         string            `json:"error"`
	ErrorTag      string            `json:"error_tag"`
	FullSync      bool              `json:"full_sync"`
	ExtraData     map[string]any    `json:"-"`
}

func (c *Client) SyncWorkspaces(ctx context.Context) ([]Workspace, string, error) {
	payload, requestID, err := c.syncRequest(ctx, map[string]string{
		"sync_token": "*", "resource_types": `["workspaces"]`,
	})
	if err != nil {
		return nil, requestID, err
	}
	return payload.Workspaces, requestID, nil
}

func (c *Client) SyncCurrentUserID(ctx context.Context) (string, string, error) {
	payload, requestID, err := c.syncRequest(ctx, map[string]string{
		"sync_token": "*", "resource_types": `["user"]`,
	})
	if err != nil {
		return "", requestID, err
	}
	if payload.User == nil {
		return "", requestID, fmt.Errorf("sync user response missing user")
	}
	if id, ok := payload.User["id"].(string); ok && strings.TrimSpace(id) != "" {
		return id, requestID, nil
	}
	if id, ok := payload.User["id"].(float64); ok {
		return strconv.FormatInt(int64(id), 10), requestID, nil
	}
	return "", requestID, fmt.Errorf("sync user response missing user id")
}

func (c *Client) syncRequest(ctx context.Context, formValues map[string]string) (syncResponse, string, error) {
	fullURL, err := c.buildURL("/sync", nil)
	if err != nil {
		return syncResponse{}, "", err
	}
	requestID := NewRequestID()
	form := url.Values{}
	for key, value := range formValues {
		form.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return syncResponse{}, requestID, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Request-Id", requestID)
	var data responseBytes
	_, err = c.doRequest(req, "/sync", &data, syncRetrySafe(req, formValues))
	if err != nil {
		return syncResponse{}, requestID, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return syncResponse{}, requestID, fmt.Errorf("decode sync response: %w", err)
	}
	var payload syncResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return syncResponse{}, requestID, fmt.Errorf("decode sync response: %w", err)
	}
	payload.ExtraData = raw
	if payload.Error != "" {
		msg := payload.Error
		if payload.ErrorTag != "" {
			msg = payload.ErrorTag + ": " + payload.Error
		}
		return syncResponse{}, requestID, &APIError{Status: 400, Message: msg, RequestID: requestID}
	}
	return payload, requestID, nil
}

// Sync reads are retry-safe; mutations require a stable UUID for every command.
// A request header alone does not establish Sync command idempotency.
func syncRetrySafe(req *http.Request, form map[string]string) bool {
	if isReadRequest(req, "/sync") {
		return true
	}
	for key := range form {
		switch key {
		case "commands", "sync_token", "resource_types":
		default:
			return false
		}
	}
	var commands []struct {
		UUID string `json:"uuid"`
	}
	if json.Unmarshal([]byte(form["commands"]), &commands) != nil || len(commands) == 0 {
		return false
	}
	for _, command := range commands {
		if strings.TrimSpace(command.UUID) == "" {
			return false
		}
	}
	return true
}
