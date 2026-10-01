package cli

import (
	"sort"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func workspaceCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printWorkspaceHelp(ctx.Stdout)
		return nil
	}
	switch canonicalSubcommand(args[0], map[string]string{"ls": "list"}) {
	case "list":
		return workspaceList(ctx, args[1:])
	default:
		return unknownCommand("workspace", args[0])
	}
}

func workspaceList(ctx *Context, args []string) error {
	fs := newFlagSet("workspace list")
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printWorkspaceHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	workspaces, reqID, err := ctx.Client.SyncWorkspaces(reqCtx)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Name < workspaces[j].Name
	})
	return writeWorkspaceList(ctx, workspaces)
}

func writeWorkspaceList(ctx *Context, workspaces []api.Workspace) error {
	if ctx.Mode == output.ModeIDsOnly {
		return writeIDs(ctx, workspaces, func(item api.Workspace) string { return item.ID }, "")
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, workspaces)
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSON(ctx.Stdout, workspaces)
	}
	rows := make([][]string, 0, len(workspaces))
	for _, w := range workspaces {
		rows = append(rows, []string{w.ID, w.Name, w.Role, w.Plan})
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Name", "Role", "Plan"}, rows)
}
