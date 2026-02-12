package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

const syncBaseURL = "https://api.todoist.com/sync/v9"

type APIError struct {
	Status  int
	Message string
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
	form := url.Values{}
	form.Set("text", text)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, syncBaseURL+"/quick/add", strings.NewReader(form.Encode()))
	if err != nil {
		return Task{}, "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	requestID := NewRequestID()
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Task{}, requestID, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return Task{}, requestID, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(msg))}
	}
	var payload struct {
		Item Task `json:"item"`
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Task{}, requestID, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Task{}, requestID, fmt.Errorf("empty response")
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return Task{}, requestID, fmt.Errorf("decode response: %w", err)
	}
	return payload.Item, requestID, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any, includeRequestID bool) (string, error) {
	fullURL, err := c.buildURL(path, query)
	if err != nil {
		return "", err
	}
	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("encode body: %w", err)
		}
		buf = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, buf)
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	requestID := ""
	if includeRequestID {
		requestID = NewRequestID()
		if requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return requestID, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return requestID, &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(msg))}
	}
	if out == nil {
		return requestID, nil
	}
	data, err := io.ReadAll(resp.Body)
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
