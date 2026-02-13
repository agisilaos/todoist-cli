package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type APIError struct {
	Status    int
	Message   string
	RequestID string
}

const maxRetries = 2

var waitForRetry = func(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("api error: status %d", e.Status)
	}
	return fmt.Sprintf("api error: status %d: %s", e.Status, e.Message)
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "https://api.todoist.com/api/v1"
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) (string, error) {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, out, false)
}

func (c *Client) Post(ctx context.Context, path string, query url.Values, body any, out any, includeRequestID bool) (string, error) {
	return c.doJSON(ctx, http.MethodPost, path, query, body, out, includeRequestID)
}

func (c *Client) Delete(ctx context.Context, path string, query url.Values) (string, error) {
	return c.doJSON(ctx, http.MethodDelete, path, query, nil, nil, true)
}

func (c *Client) QuickAdd(ctx context.Context, text string) (Task, string, error) {
	var task Task
	reqID, err := c.doJSON(ctx, http.MethodPost, "/tasks/quick", nil, map[string]any{"text": text}, &task, true)
	if err != nil {
		return Task{}, reqID, err
	}
	return task, reqID, nil
}

func (c *Client) SyncWorkspaces(ctx context.Context) ([]Workspace, string, error) {
	fullURL, err := c.buildURL("/sync", nil)
	if err != nil {
		return nil, "", err
	}
	requestID := NewRequestID()
	form := url.Values{}
	form.Set("sync_token", "*")
	form.Set("resource_types", `["workspaces"]`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, requestID, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Request-Id", requestID)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, requestID, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if resp.StatusCode >= 400 {
		return nil, requestID, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(data)), RequestID: requestID}
	}
	var payload struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, requestID, fmt.Errorf("decode sync response: %w", err)
	}
	return payload.Workspaces, requestID, nil
}

func (c *Client) SyncCurrentUserID(ctx context.Context) (string, string, error) {
	fullURL, err := c.buildURL("/sync", nil)
	if err != nil {
		return "", "", err
	}
	requestID := NewRequestID()
	form := url.Values{}
	form.Set("sync_token", "*")
	form.Set("resource_types", `["user"]`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", requestID, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Request-Id", requestID)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", requestID, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if resp.StatusCode >= 400 {
		return "", requestID, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(data)), RequestID: requestID}
	}
	var payload struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", requestID, fmt.Errorf("decode sync user response: %w", err)
	}
	if payload.User == nil {
		return "", requestID, fmt.Errorf("sync user response missing user")
	}
	if id, ok := payload.User["id"].(string); ok && strings.TrimSpace(id) != "" {
		return id, requestID, nil
	}
	if idf, ok := payload.User["id"].(float64); ok {
		return strconv.FormatInt(int64(idf), 10), requestID, nil
	}
	return "", requestID, fmt.Errorf("sync user response missing user id")
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any, includeRequestID bool) (string, error) {
	fullURL, err := c.buildURL(path, query)
	if err != nil {
		return "", err
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("encode body: %w", err)
		}
	}
	requestID := ""
	if includeRequestID {
		requestID = NewRequestID()
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var buf io.Reader
		if payload != nil {
			buf = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, buf)
		if err != nil {
			return requestID, err
		}
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			if shouldRetryTransport(method, includeRequestID, err) && attempt < maxRetries {
				if err := waitForRetry(ctx, retryDelay(attempt, "")); err != nil {
					return requestID, err
				}
				continue
			}
			return requestID, err
		}

		if resp.StatusCode >= 400 {
			msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
			_ = resp.Body.Close()
			if shouldRetryStatus(method, includeRequestID, resp.StatusCode) && attempt < maxRetries {
				if err := waitForRetry(ctx, retryDelay(attempt, resp.Header.Get("Retry-After"))); err != nil {
					return requestID, err
				}
				continue
			}
			return requestID, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(msg)), RequestID: requestID}
		}

		if out == nil {
			_ = resp.Body.Close()
			return requestID, nil
		}
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return requestID, err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return requestID, nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return requestID, fmt.Errorf("decode response: %w", err)
		}
		return requestID, nil
	}
	return requestID, errors.New("exhausted retries")
}

func (c *Client) buildURL(path string, query url.Values) (string, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return "", err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func shouldRetryStatus(method string, includeRequestID bool, status int) bool {
	if !isRetrySafe(method, includeRequestID) {
		return false
	}
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return status >= 500
	}
}

func shouldRetryTransport(method string, includeRequestID bool, err error) bool {
	return isRetrySafe(method, includeRequestID) && err != nil
}

func isRetrySafe(method string, includeRequestID bool) bool {
	return method == http.MethodGet || includeRequestID
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs >= 0 {
		delay := time.Duration(secs) * time.Second
		if delay > 3*time.Second {
			return 3 * time.Second
		}
		return delay
	}
	delay := 200 * time.Millisecond
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	if delay > 1200*time.Millisecond {
		return 1200 * time.Millisecond
	}
	return delay
}
