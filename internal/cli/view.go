package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
)

type viewTarget struct {
	Command string
	Args    []string
}

func viewCommand(ctx *Context, args []string) error {
	fs := newFlagSet("view")
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printViewHelp(ctx.Stdout)
		return nil
	}
	if len(fs.Args()) == 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("view requires a Todoist URL")}
	}
	target, err := resolveViewTarget(fs.Arg(0), ctx)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	return dispatchViewTarget(ctx, target)
}

func dispatchViewTarget(ctx *Context, target viewTarget) error {
	switch target.Command {
	case "task":
		return taskCommand(ctx, target.Args)
	case "filter":
		return filterCommand(ctx, target.Args)
	case "today":
		return todayCommand(ctx, target.Args)
	case "upcoming":
		return upcomingCommand(ctx, target.Args)
	case "completed":
		return completedCommand(ctx, target.Args)
	case "settings":
		return settingsCommand(ctx, target.Args)
	case "activity":
		return activityCommand(ctx, target.Args)
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported view target: %s", target.Command)}
	}
}

func resolveViewTarget(raw string, ctx *Context) (viewTarget, error) {
	if parsed, ok := apprefs.ParseTodoistEntityURL(raw); ok {
		switch parsed.Entity {
		case "task":
			return viewTarget{Command: "task", Args: []string{"view", "id:" + parsed.ID}}, nil
		case "project":
			return viewTarget{Command: "task", Args: []string{"list", "--project", "id:" + parsed.ID}}, nil
		case "filter":
			return viewTarget{Command: "filter", Args: []string{"show", "id:" + parsed.ID}}, nil
		case "label":
			name, err := resolveLabelNameByID(ctx, parsed.ID)
			if err != nil {
				return viewTarget{}, err
			}
			return viewTarget{Command: "task", Args: []string{"list", "--label", name}}, nil
		default:
			return viewTarget{}, fmt.Errorf("unsupported Todoist entity URL: %s", parsed.Entity)
		}
	}

	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return viewTarget{}, fmt.Errorf("invalid URL")
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "app.todoist.com" {
		return viewTarget{}, fmt.Errorf("unsupported URL host: %s", u.Hostname())
	}
	switch strings.Trim(u.Path, "/") {
	case "app/inbox":
		return viewTarget{Command: "task", Args: []string{"list"}}, nil
	case "app/today":
		return viewTarget{Command: "today", Args: nil}, nil
	case "app/upcoming":
		return viewTarget{Command: "upcoming", Args: nil}, nil
	case "app/completed":
		return viewTarget{Command: "completed", Args: nil}, nil
	case "app/settings":
		return viewTarget{Command: "settings", Args: []string{"view"}}, nil
	case "app/activity":
		return viewTarget{Command: "activity", Args: nil}, nil
	default:
		return viewTarget{}, fmt.Errorf("unsupported Todoist URL path: %s", u.Path)
	}
}

func resolveLabelNameByID(ctx *Context, id string) (string, error) {
	if err := ensureClient(ctx); err != nil {
		return "", err
	}
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	var labels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	reqID, err := ctx.Client.Get(reqCtx, "/labels", nil, &labels)
	if err != nil {
		return "", err
	}
	setRequestID(ctx, reqID)
	for _, label := range labels {
		if label.ID == id {
			return label.Name, nil
		}
	}
	return "", fmt.Errorf("label %q not found", id)
}

func printViewHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist view <url>

Notes:
  - Opens Todoist web URLs via equivalent CLI commands.
  - Supported entity URLs: task, project, label, filter.
  - Supported page URLs: inbox, today, upcoming, completed, settings, activity.

Examples:
  todoist view https://app.todoist.com/app/task/call-mom-6f3qg8mgqp99mFVJ
  todoist view https://app.todoist.com/app/project/home-2203306141
  todoist view https://app.todoist.com/app/settings
`)
}
