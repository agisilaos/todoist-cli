package cli

import (
	"bytes"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	appnotifications "github.com/agisilaos/todoist-cli/internal/app/notifications"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestResourceIDsOnly(t *testing.T) {
	writers := []struct {
		name      string
		paginated bool
		write     func(*Context, []string, string) error
	}{
		{name: "task", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Task, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeTaskList(ctx, items, cursor, false)
		}},
		{name: "project", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Project, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeProjectList(ctx, items, cursor)
		}},
		{name: "collaborator", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Collaborator, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeProjectCollaborators(ctx, items, cursor)
		}},
		{name: "section", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Section, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeSectionList(ctx, items, cursor)
		}},
		{name: "label", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Label, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeLabelList(ctx, items, cursor)
		}},
		{name: "comment", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Comment, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeCommentList(ctx, items, cursor)
		}},
		{name: "filter", paginated: false, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Filter, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeFilterList(ctx, items)
		}},
		{name: "workspace", paginated: false, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Workspace, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeWorkspaceList(ctx, items)
		}},
		{name: "reminder", paginated: false, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Reminder, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeReminderList(ctx, items)
		}},
		{name: "activity", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.ActivityEvent, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeActivityList(ctx, items, cursor)
		}},
		{name: "notification", paginated: true, write: func(ctx *Context, ids []string, cursor string) error {
			items := make([]api.Notification, len(ids))
			for i, id := range ids {
				items[i].ID = id
			}
			return writeNotificationList(ctx, appnotifications.ListResult{Items: items, HasMore: cursor != "", Offset: 20, Limit: 10})
		}},
	}
	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			cases := []struct {
				name    string
				ids     []string
				cursor  string
				want    string
				invalid bool
			}{
				{"rows", []string{"opaque-Z123456789", "a2", "a2"}, "", "opaque-Z123456789\na2\na2\n", false},
				{"empty", nil, "", "", false},
				{"page", []string{"a1"}, "next-token", "a1\n", false},
				{"empty_page", nil, "next-token", "", false},
				{"missing_id", []string{"valid", ""}, "", "", true},
				{"newline", []string{"valid", "bad\nid"}, "", "", true},
				{"space", []string{"valid", "bad id"}, "", "", true},
				{"unicode_space", []string{"valid", "bad\u00a0id"}, "", "", true},
				{"control", []string{"valid", "bad\x00id"}, "", "", true},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					ctx := &Context{Stdout: &stdout, Stderr: &stderr, Mode: output.ModeIDsOnly}
					err := writer.write(ctx, tc.ids, tc.cursor)
					if (err != nil) != tc.invalid || stdout.String() != tc.want {
						t.Fatalf("err=%v stdout=%q want=%q", err, stdout.String(), tc.want)
					}
					wantNotice := ""
					if writer.paginated && tc.cursor != "" {
						wantNotice = "More available. Use --cursor \"next-token\"\n"
						if writer.name == "notification" {
							wantNotice = "More available. Use --offset 30\n"
						}
					}
					if stderr.String() != wantNotice {
						t.Fatalf("stderr=%q want=%q", stderr.String(), wantNotice)
					}
				})
			}
		})
	}
}
