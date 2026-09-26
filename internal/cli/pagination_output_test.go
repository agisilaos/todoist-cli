package cli

import (
	"bytes"
	"encoding/json"
	"github.com/agisilaos/todoist-cli/internal/api"
	appnotifications "github.com/agisilaos/todoist-cli/internal/app/notifications"
	"github.com/agisilaos/todoist-cli/internal/output"
	"strings"
	"testing"
)

func TestStructuredListsExposePaginationOnStderr(t *testing.T) {
	writers := map[string]func(*Context, string) error{
		"tasks":         func(c *Context, s string) error { return writeTaskList(c, []api.Task{}, s, false) },
		"projects":      func(c *Context, s string) error { return writeProjectList(c, []api.Project{}, s) },
		"sections":      func(c *Context, s string) error { return writeSectionList(c, []api.Section{}, s) },
		"labels":        func(c *Context, s string) error { return writeLabelList(c, []api.Label{}, s) },
		"comments":      func(c *Context, s string) error { return writeCommentList(c, []api.Comment{}, s) },
		"collaborators": func(c *Context, s string) error { return writeProjectCollaborators(c, []api.Collaborator{}, s) },
		"activity":      func(c *Context, s string) error { return writeActivityList(c, []api.ActivityEvent{}, s) },
		"notifications": func(c *Context, s string) error {
			return writeNotificationList(c, appnotifications.ListResult{HasMore: s != "", Limit: 20, Offset: 0})
		},
	}
	for name, write := range writers {
		for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON} {
			for _, cursor := range []string{"", "next\npage"} {
				t.Run(name+"/"+string(mode)+"/"+cursor, func(t *testing.T) {
					var out, errOut bytes.Buffer
					c := &Context{Stdout: &out, Stderr: &errOut, Mode: mode}
					if err := write(c, cursor); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(out.String(), "More available") {
						t.Fatal("pagination contaminated stdout")
					}
					if mode == output.ModeJSON && !json.Valid(out.Bytes()) {
						t.Fatal("invalid JSON payload")
					}
					if cursor == "" {
						if errOut.Len() != 0 {
							t.Fatal("exhausted list advertised a page")
						}
					} else if strings.Count(errOut.String(), "\n") != 1 || !strings.Contains(errOut.String(), "More available. Use --") {
						t.Fatal("missing or unsafe continuation notice")
					}
				})
			}
		}
	}
}
