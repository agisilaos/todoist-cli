package cli

import (
	"fmt"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	appactivities "github.com/agisilaos/todoist-cli/internal/app/activities"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func activityCommand(ctx *Context, args []string) error {
	fs := newFlagSet("activity")
	var since string
	var until string
	var typ string
	var event string
	var project string
	var by string
	var limit int
	var cursor string
	var all bool
	var help bool
	fs.StringVar(&since, "since", "", "Start date (YYYY-MM-DD)")
	fs.StringVar(&until, "until", "", "End date (YYYY-MM-DD)")
	fs.StringVar(&typ, "type", "", "Object type: task, comment, project")
	fs.StringVar(&event, "event", "", "Event type")
	fs.StringVar(&project, "project", "", "Project reference")
	fs.StringVar(&by, "by", "", "Initiator ID or 'me'")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printActivityHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}

	var projectID string
	if strings.TrimSpace(project) != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		projectID = id
	}
	if strings.EqualFold(strings.TrimSpace(by), "me") {
		reqCtx, cancel := requestContext(ctx)
		userID, reqID, err := ctx.Client.SyncCurrentUserID(reqCtx)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		by = userID
	}

	in, err := appactivities.NormalizeListInput(appactivities.ListInput{
		Since:     since,
		Until:     until,
		Type:      typ,
		Event:     event,
		ProjectID: projectID,
		By:        by,
		Limit:     limit,
		Cursor:    cursor,
		All:       all,
	})
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	events, next, err := fetchPaginated[api.ActivityEvent](ctx, "/activities", appactivities.BuildQuery(in), in.All)
	if err != nil {
		return err
	}
	return writeActivityList(ctx, events, next)
}

func writeActivityList(ctx *Context, events []api.ActivityEvent, cursor string) error {
	if events == nil {
		events = []api.ActivityEvent{}
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, events, output.Meta{RequestID: ctx.RequestID, Count: len(events), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, events)
	}
	if len(events) == 0 {
		fmt.Fprintln(ctx.Stdout, "No activity found.")
		return nil
	}
	projectNames := projectNameMap(ctx)
	rows := make([][]string, 0, len(events))
	for _, event := range events {
		project := event.ParentProjectID
		if name, ok := projectNames[event.ParentProjectID]; ok {
			project = name
		}
		rows = append(rows, []string{
			event.EventDate,
			event.EventType,
			normalizeActivityObjectType(event.ObjectType),
			project,
			activityContent(event),
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"Date", "Event", "Type", "Project", "Content"}, rows)
}

func normalizeActivityObjectType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "item":
		return "task"
	case "note":
		return "comment"
	default:
		return strings.TrimSpace(v)
	}
}

func activityContent(event api.ActivityEvent) string {
	if event.ExtraData == nil {
		if event.ObjectID != "" {
			return "id:" + event.ObjectID
		}
		return ""
	}
	for _, key := range []string{"content", "name", "last_content", "last_name"} {
		if value, ok := event.ExtraData[key]; ok {
			if text := strings.TrimSpace(fmt.Sprintf("%v", value)); text != "" {
				return text
			}
		}
	}
	if event.ObjectID != "" {
		return "id:" + event.ObjectID
	}
	return ""
}

func printActivityHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist activity [--since <date>] [--until <date>] [--type task|comment|project] [--event <type>] [--project <id|name>] [--by <id|me>] [--limit <n>] [--cursor <cursor>] [--all]

Notes:
  - Queries Todoist activity logs endpoint with cursor pagination.
  - --type task maps to object_type=item; comment maps to object_type=note.

Examples:
  todoist activity
  todoist activity --since 2026-02-01 --until 2026-02-23
  todoist activity --type task --event completed --project Home
  todoist activity --by me --limit 20
`)
}
