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
	"strings"
)

// TaskWriteError retains uncertainty at the HTTP boundary, independently of
// optional resource decoding. Other resources keep their existing retry policy.
type TaskWriteError struct {
	Outcome   string
	RequestID string
	Err       error
}

func (e *TaskWriteError) Error() string { return fmt.Sprintf("task write %s: %v", e.Outcome, e.Err) }
func (e *TaskWriteError) Unwrap() error { return e.Err }
func TaskWriteOutcome(err error) string {
	if err == nil {
		return "accepted"
	}
	var write *TaskWriteError
	if errors.As(err, &write) {
		return write.Outcome
	}
	return "not_dispatched"
}
func taskWriteFailure(err error, status int, requestID string) error {
	outcome := "uncertain"
	if status >= 400 && status < 500 && status != http.StatusRequestTimeout {
		outcome = "rejected"
	}
	return &TaskWriteError{Outcome: outcome, RequestID: requestID, Err: err}
}
func taskWritePath(method, path string) bool {
	return method != http.MethodGet && (path == "/tasks" || strings.HasPrefix(path, "/tasks/"))
}
func (c *Client) taskWriteRequest(req *http.Request) bool {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return false
	}
	path := strings.TrimPrefix(req.URL.Path, base.Path)
	if taskWritePath(req.Method, path) {
		return true
	}
	if path != "/sync" || req.Method != http.MethodPost || req.GetBody == nil {
		return false
	}
	body, err := req.GetBody()
	if err != nil {
		return false
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return false
	}
	var commands json.RawMessage
	if req.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		form, err := url.ParseQuery(string(data))
		if err != nil {
			return false
		}
		commands = []byte(form.Get("commands"))
	} else {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		commands = fields["commands"]
	}
	var entries []struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(commands, &entries) != nil {
		return false
	}
	for _, command := range entries {
		if strings.HasPrefix(command.Type, "item_") {
			return true
		}
	}
	return false
}

// TaskCommand dispatches one native task command. Only the command's exact
// acknowledgement is required; items are optional, individually decoded data.
func (c *Client) TaskCommand(ctx context.Context, kind string, args map[string]any) ([]byte, string, error) {
	uuid, requestID := NewRequestID(), NewRequestID()
	commands, err := json.Marshal([]any{map[string]any{"type": kind, "uuid": uuid, "args": args}})
	if err != nil {
		return nil, requestID, err
	}
	fullURL, err := c.buildURL("/sync", nil)
	if err != nil {
		return nil, requestID, err
	}
	form := url.Values{"sync_token": {"*"}, "resource_types": {`[]`}, "commands": {string(commands)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, requestID, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Request-Id", requestID)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.dispatch(req, "/sync")
	if err != nil {
		return nil, requestID, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, requestID, taskWriteFailure(&APIError{Status: resp.StatusCode, RequestID: requestID}, resp.StatusCode, requestID)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil || len(data) > 4*1024*1024 {
		return nil, requestID, taskWriteFailure(errors.New("required command acknowledgement unavailable"), 0, requestID)
	}
	envelope, err := acknowledgementEnvelope(data)
	if err != nil {
		return nil, requestID, taskWriteFailure(errors.New("invalid required command acknowledgement"), 0, requestID)
	}
	statuses, err := uniqueJSONObject(envelope["sync_status"])
	if err != nil {
		return nil, requestID, taskWriteFailure(errors.New("missing required command acknowledgement"), 0, requestID)
	}
	status := statuses[uuid]
	var value string
	if json.Unmarshal(status, &value) != nil || value != "ok" {
		outcome := "uncertain"
		if rejectedTaskAcknowledgement(status) {
			outcome = "rejected"
		}
		cause := error(errors.New("required command acknowledgement did not establish acceptance"))
		if outcome == "rejected" {
			fields, _ := uniqueJSONObject(status)
			var message, tag string
			var code int
			_ = json.Unmarshal(fields["error"], &message)
			_ = json.Unmarshal(fields["error_tag"], &tag)
			_ = json.Unmarshal(fields["http_code"], &code)
			if code != 0 {
				cause = &APIError{Status: code, Message: strings.TrimSpace(message + " " + tag), RequestID: requestID}
			} else {
				cause = fmt.Errorf("task command rejected: %s", status)
			}
		}
		return nil, requestID, &TaskWriteError{Outcome: outcome, RequestID: requestID, Err: cause}
	}
	id, _ := args["id"].(string)
	var items []json.RawMessage
	if json.Unmarshal(envelope["items"], &items) != nil {
		return nil, requestID, nil
	}
	var result []byte
	for _, item := range items {
		var identity struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(item, &identity) == nil && identity.ID == id {
			if result != nil {
				return nil, requestID, nil
			}
			result = item
		}
	}
	return result, requestID, nil
}
func rejectedTaskAcknowledgement(raw json.RawMessage) bool {
	fields, err := uniqueJSONObject(raw)
	if err != nil {
		return false
	}
	if extra, ok := fields["error_extra"]; ok {
		var value map[string]json.RawMessage
		if json.Unmarshal(extra, &value) != nil || value == nil {
			return false
		}
	}
	recognized := false
	for _, key := range []string{"error", "error_tag"} {
		if raw, ok := fields[key]; ok {
			var value string
			if json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value) == "" {
				return false
			}
			recognized = true
		}
	}
	for _, key := range []string{"error_code", "http_code"} {
		if raw, ok := fields[key]; ok {
			var value int
			if json.Unmarshal(raw, &value) != nil {
				return false
			}
			if key == "http_code" {
				if value < 400 || value > 599 {
					return false
				}
			} else if value <= 0 {
				return false
			}
			recognized = true
		}
	}
	return recognized
}

// Required acknowledgements must be unambiguous; encoding/json maps otherwise
// silently let the last duplicate key override contradictory earlier evidence.
func uniqueJSONObject(data []byte) (map[string]json.RawMessage, error) {
	return decodeAcknowledgementObject(data, false)
}
func acknowledgementEnvelope(data []byte) (map[string]json.RawMessage, error) {
	return decodeAcknowledgementObject(data, true)
}
func decodeAcknowledgementObject(data []byte, optionalEnvelope bool) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("expected object")
	}
	fields := map[string]json.RawMessage{}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("invalid object key")
		}
		duplicate := seen[key]
		if duplicate && (!optionalEnvelope || key == "sync_status") {
			return nil, errors.New("duplicate required object key")
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		seen[key] = true
		if duplicate {
			fields[key] = nil
		} else {
			fields[key] = value
		}
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return fields, nil
}
