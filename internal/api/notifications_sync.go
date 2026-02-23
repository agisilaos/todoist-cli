package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Notification struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	IsUnread         bool   `json:"is_unread"`
	IsDeleted        bool   `json:"is_deleted"`
	CreatedAt        string `json:"created_at"`
	FromUserID       string `json:"from_uid,omitempty"`
	FromUserName     string `json:"from_user_name,omitempty"`
	ProjectID        string `json:"project_id,omitempty"`
	ProjectName      string `json:"project_name,omitempty"`
	TaskID           string `json:"item_id,omitempty"`
	TaskContent      string `json:"item_content,omitempty"`
	InvitationID     string `json:"invitation_id,omitempty"`
	InvitationSecret string `json:"invitation_secret,omitempty"`
}

func (c *Client) FetchLiveNotifications(ctx context.Context) ([]Notification, string, error) {
	resp, requestID, err := c.syncRequest(ctx, map[string]string{
		"sync_token":     "*",
		"resource_types": `["live_notifications"]`,
	})
	if err != nil {
		return nil, requestID, err
	}
	raw := resp.ExtraData["live_notifications"]
	list, ok := raw.([]any)
	if !ok {
		return []Notification{}, requestID, nil
	}
	notifications := make([]Notification, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		n := Notification{
			ID:               stringifyAny(m["id"]),
			Type:             firstNonEmpty(stringifyAny(m["notification_type"]), stringifyAny(m["type"])),
			IsUnread:         boolAny(m["is_unread"]),
			IsDeleted:        boolAny(m["is_deleted"]),
			CreatedAt:        firstNonEmpty(stringifyAny(m["created_at"]), stringifyAny(m["created"])),
			FromUserID:       stringifyAny(m["from_uid"]),
			ProjectID:        stringifyAny(m["project_id"]),
			ProjectName:      stringifyAny(m["project_name"]),
			TaskID:           stringifyAny(m["item_id"]),
			TaskContent:      stringifyAny(m["item_content"]),
			InvitationID:     stringifyAny(m["invitation_id"]),
			InvitationSecret: stringifyAny(m["invitation_secret"]),
		}
		if fromUser, ok := m["from_user"].(map[string]any); ok {
			n.FromUserName = firstNonEmpty(stringifyAny(fromUser["full_name"]), stringifyAny(fromUser["name"]))
		}
		if n.IsDeleted {
			continue
		}
		notifications = append(notifications, n)
	}
	return notifications, requestID, nil
}

func (c *Client) MarkNotificationsRead(ctx context.Context, ids []string) (string, error) {
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return "", fmt.Errorf("at least one notification id is required")
	}
	payload, err := json.Marshal([]map[string]any{{
		"type": "live_notifications_mark_read",
		"uuid": NewRequestID(),
		"args": map[string]any{"ids": filtered},
	}})
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func (c *Client) MarkNotificationsUnread(ctx context.Context, ids []string) (string, error) {
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return "", fmt.Errorf("at least one notification id is required")
	}
	payload, err := json.Marshal([]map[string]any{{
		"type": "live_notifications_mark_unread",
		"uuid": NewRequestID(),
		"args": map[string]any{"ids": filtered},
	}})
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func (c *Client) MarkAllNotificationsRead(ctx context.Context) (string, error) {
	payload, err := json.Marshal([]map[string]any{{
		"type": "live_notifications_mark_read_all",
		"uuid": NewRequestID(),
		"args": map[string]any{},
	}})
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func stringifyAny(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case float64:
		if vv == float64(int64(vv)) {
			return fmt.Sprintf("%d", int64(vv))
		}
		return fmt.Sprintf("%v", vv)
	default:
		return ""
	}
}

func boolAny(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
