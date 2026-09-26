package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/authorization"
)

// SetTransport changes transport for embedding/testing without exposing an unguarded client.
func (c *Client) SetTransport(transport http.RoundTripper) { c.http.Transport = transport }

// dispatch is the sole Todoist resource HTTP boundary. Unknown operations require writes.
func (c *Client) dispatch(req *http.Request, path string) (*http.Response, error) {
	if err := c.authorizeRequest(req, path); err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	var denied *authorization.Error
	if errors.As(err, &denied) {
		return nil, denied
	}
	return resp, err
}

func (c *Client) authorizeRequest(req *http.Request, path string) error {
	if err := c.authorization.CheckCredential(); err != nil {
		return err
	}
	if !isReadRequest(req, path) {
		return c.authorization.CheckMutation()
	}
	return nil
}

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	// Keep net/http's default limit while checking every redirected request.
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}
	path := ""
	if req.URL.Scheme == base.Scheme && req.URL.Host == base.Host && strings.HasPrefix(req.URL.Path, base.Path) {
		path = strings.TrimPrefix(req.URL.Path, base.Path)
	}
	return c.authorizeRequest(req, path)
}

func isReadRequest(req *http.Request, path string) bool {
	if req.URL.Query().Has("commands") {
		return false
	}
	if req.Method == http.MethodGet {
		switch path {
		case "/tasks", "/tasks/filter", "/tasks/completed/by_completion_date", "/tasks/completed/by_due_date", "/tasks/completed/stats", "/projects", "/projects/archived", "/sections", "/labels", "/comments", "/filters", "/activities":
			return true
		}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && parts[1] != "" {
			switch parts[0] {
			case "tasks", "projects", "sections", "labels", "comments", "filters":
				return true
			}
		}
		return len(parts) == 3 && parts[0] == "projects" && parts[1] != "" && parts[2] == "collaborators"
	}
	if req.Method != http.MethodPost || path != "/sync" || req.GetBody == nil {
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
	var fields map[string]json.RawMessage
	switch req.Header.Get("Content-Type") {
	case "application/x-www-form-urlencoded":
		form, err := url.ParseQuery(string(data))
		if err != nil {
			return false
		}
		fields = map[string]json.RawMessage{}
		for key, values := range form {
			if len(values) != 1 {
				return false
			}
			fields[key] = json.RawMessage(values[0])
		}
	case "application/json":
		if json.Unmarshal(data, &fields) != nil {
			return false
		}
	default:
		return false
	}
	for key := range fields {
		switch key {
		case "sync_token", "resource_types", "commands":
		default:
			return false
		}
	}
	if commands, ok := fields["commands"]; ok {
		var entries []json.RawMessage
		if json.Unmarshal(commands, &entries) != nil || entries == nil || len(entries) != 0 {
			return false
		}
	}
	var resources []string
	if json.Unmarshal(fields["resource_types"], &resources) != nil || len(resources) == 0 {
		return false
	}
	for _, resource := range resources {
		switch resource {
		case "workspaces", "user", "user_settings", "live_notifications", "reminders":
		default:
			return false
		}
	}
	return true
}
