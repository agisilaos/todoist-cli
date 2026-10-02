package api

import (
	"context"
	"encoding/json"
	"fmt"
)

type filterSyncItem struct {
	Filter
	IsDeleted bool `json:"is_deleted"`
}

func (c *Client) FetchFilters(ctx context.Context) ([]Filter, string, error) {
	resp, requestID, err := c.syncRequest(ctx, map[string]string{
		"sync_token": "*", "resource_types": `["filters"]`,
	})
	if err != nil {
		return nil, requestID, err
	}
	filters := make([]Filter, 0, len(resp.Filters))
	for _, item := range resp.Filters {
		if !item.IsDeleted {
			filters = append(filters, item.Filter)
		}
	}
	return filters, requestID, nil
}

func (c *Client) AddFilter(ctx context.Context, fields map[string]any) (Filter, string, error) {
	tempID := NewRequestID()
	resp, requestID, err := c.filterCommand(ctx, "filter_add", fields, tempID)
	if err != nil {
		return Filter{}, requestID, err
	}
	id := resp.TempIDMapping[tempID]
	if id == "" {
		return Filter{}, requestID, fmt.Errorf("filter created but sync response omitted its ID; run 'todoist filter list' to confirm")
	}
	filter, err := filterFromSync(resp, id)
	return filter, requestID, err
}

func (c *Client) UpdateFilter(ctx context.Context, id string, fields map[string]any) (Filter, string, error) {
	args := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		args[key] = value
	}
	args["id"] = id
	resp, requestID, err := c.filterCommand(ctx, "filter_update", args, "")
	if err != nil {
		return Filter{}, requestID, err
	}
	filter, err := filterFromSync(resp, id)
	return filter, requestID, err
}

func (c *Client) DeleteFilter(ctx context.Context, id string) (string, error) {
	_, requestID, err := c.filterCommand(ctx, "filter_delete", map[string]any{"id": id}, "")
	return requestID, err
}

func (c *Client) filterCommand(ctx context.Context, kind string, args map[string]any, tempID string) (syncResponse, string, error) {
	uuid := NewRequestID()
	command := map[string]any{"type": kind, "uuid": uuid, "args": args}
	if tempID != "" {
		command["temp_id"] = tempID
	}
	payload, err := json.Marshal([]map[string]any{command})
	if err != nil {
		return syncResponse{}, "", err
	}
	resp, requestID, err := c.syncRequest(ctx, map[string]string{
		"commands": string(payload), "sync_token": "*", "resource_types": `["filters"]`,
	})
	if err != nil {
		return resp, requestID, err
	}
	status, exists := resp.SyncStatus[uuid]
	if !exists {
		return resp, requestID, fmt.Errorf("%s response omitted command acknowledgement; run 'todoist filter list' to confirm", kind)
	}
	if status == "ok" {
		return resp, requestID, nil
	}
	code := 400
	if details, ok := status.(map[string]any); ok {
		if httpCode, ok := details["http_code"].(float64); ok && httpCode >= 400 && httpCode <= 599 {
			code = int(httpCode)
		}
	}
	details, _ := json.Marshal(status)
	return resp, requestID, &APIError{Status: code, RequestID: requestID, Message: kind + ": " + string(details)}
}

func filterFromSync(resp syncResponse, id string) (Filter, error) {
	for _, item := range resp.Filters {
		if item.ID == id && !item.IsDeleted {
			return item.Filter, nil
		}
	}
	return Filter{}, fmt.Errorf("filter mutation succeeded but sync response omitted the result; run 'todoist filter list' to confirm")
}
