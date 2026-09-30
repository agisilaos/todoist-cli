package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type profileProblem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// The error carries evidence for its own profile, never the invocation's
// previously selected credential. The renderer must not infer that association.
type profileCommandError struct {
	Code          string
	Message       string
	Profile       string
	Authorization *authorization.Report
}

func (e *profileCommandError) Error() string { return e.Message }

type profileStorageError struct {
	Profile string
	Err     error
}

func (e *profileStorageError) Error() string { return e.Err.Error() }
func (e *profileStorageError) Unwrap() error { return e.Err }

type profileListRow struct {
	Profile       string               `json:"profile"`
	Selected      bool                 `json:"selected"`
	Configured    bool                 `json:"configured"`
	Backend       string               `json:"backend"`
	Accessibility string               `json:"accessibility"`
	Recovery      string               `json:"recovery"`
	Authorization authorization.Report `json:"authorization"`
	Error         *profileProblem      `json:"error,omitempty"`
}

func profileCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printProfileHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return profileListCommand(ctx, args[1:])
	case "current":
		return profileCurrentCommand(ctx, args[1:])
	case "use":
		return profileUseCommand(ctx, args[1:])
	case "remove":
		return profileRemoveCommand(ctx, args[1:])
	default:
		return unknownCommand("profile", args[0])
	}
}

func profileArguments(ctx *Context, operation string, args []string, named bool) (string, bool, error) {
	fs := newFlagSet("profile " + operation)
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return "", false, &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printProfileHelp(ctx.Stdout)
		return "", true, nil
	}
	if !named && len(fs.Args()) != 0 {
		return "", false, &CodeError{Code: exitUsage, Err: fmt.Errorf("profile %s accepts no positional arguments", operation)}
	}
	if named {
		if len(fs.Args()) != 1 || strings.TrimSpace(fs.Args()[0]) == "" || strings.ContainsFunc(fs.Args()[0], unicode.IsControl) {
			return "", false, &CodeError{Code: exitUsage, Err: fmt.Errorf("profile %s requires one credential profile name", operation)}
		}
		return fs.Args()[0], false, nil
	}
	return "", false, nil
}

func profileOperationContext(ctx *Context) context.Context {
	if ctx.OperationContext != nil {
		return ctx.OperationContext
	}
	return context.Background()
}

func profileSelectionSource(ctx *Context) string {
	if ctx.SelectionSource != "" {
		return ctx.SelectionSource
	}
	_, source := resolveProfileSelection(ctx.Global.Profile, ctx.ProjectDefaultProfile, ctx.UserDefaultProfile)
	return source
}

func profileIssue(err error) *profileProblem {
	if err == nil {
		return nil
	}
	var storeErr *credentials.Error
	if errors.As(err, &storeErr) {
		return &profileProblem{Code: string(storeErr.Kind), Message: storeErr.Error()}
	}
	var authErr *authorization.Error
	if errors.As(err, &authErr) {
		return &profileProblem{Code: authErr.Code, Message: authErr.Message}
	}
	return &profileProblem{Code: "CREDENTIAL_STORE_IO", Message: "Credential storage could not complete an I/O operation."}
}

func profileListCommand(ctx *Context, args []string) error {
	if _, help, err := profileArguments(ctx, "list", args, false); err != nil || help {
		return err
	}
	store := profileStore(ctx)
	names, err := store.List(profileOperationContext(ctx))
	if err != nil {
		return err
	}
	rows := make([]profileListRow, 0, len(names))
	invalid := false
	for _, name := range names {
		info, inspectErr := store.Inspect(profileOperationContext(ctx), name)
		report := authorization.Resolve(info.Authorization, "credentials", info.Configured)
		row := profileListRow{Profile: name, Selected: name == ctx.Profile, Configured: info.Configured, Backend: info.Backend, Accessibility: "unchecked", Recovery: info.Recovery, Authorization: report}
		if inspectErr == nil {
			inspectErr = report.CheckCredential()
		}
		row.Error = profileIssue(inspectErr)
		invalid = invalid || row.Error != nil
		rows = append(rows, row)
	}
	payload := map[string]any{"profiles": rows, "selected_profile": ctx.Profile, "selection_source": profileSelectionSource(ctx), "environment_token_active": os.Getenv("TODOIST_TOKEN") != ""}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		if err := writeStructuredValue(ctx, payload, output.Meta{}); err != nil {
			return err
		}
	} else {
		if len(rows) == 0 {
			fmt.Fprintln(ctx.Stdout, "No stored credential profiles. Create one with `todoist --profile NAME auth login`.")
		}
		for _, row := range rows {
			marker := ""
			if row.Selected {
				marker = " (selected)"
			}
			fmt.Fprintf(ctx.Stdout, "%q%s: configured=%t; backend=%s; accessibility unchecked; %s\n", row.Profile, marker, row.Configured, row.Backend, row.Authorization.Summary())
			if row.Recovery != "" {
				fmt.Fprintf(ctx.Stdout, "  recovery: %s; run `%s`\n", row.Recovery, credentialCommand(ctx, row.Profile, "auth", "repair"))
			}
			if row.Error != nil {
				fmt.Fprintf(ctx.Stdout, "  %s: %s\n", row.Error.Code, row.Error.Message)
			}
		}
		if os.Getenv("TODOIST_TOKEN") != "" {
			fmt.Fprintln(ctx.Stdout, "TODOIST_TOKEN overrides the selected stored profile; listed authorization describes each stored credential.")
		}
	}
	if invalid {
		return &CodeError{Code: exitAuth, Err: &profileCommandError{Code: "PROFILE_INSPECTION_FAILED", Message: "One or more stored profiles could not be inspected; see their reported errors."}}
	}
	return nil
}

func profileCurrentCommand(ctx *Context, args []string) error {
	if _, help, err := profileArguments(ctx, "current", args, false); err != nil || help {
		return err
	}
	environment := os.Getenv("TODOIST_TOKEN") != ""
	info := credentials.Info{Accessibility: "unchecked"}
	var inspectErr error
	if !environment {
		info, inspectErr = profileStore(ctx).Inspect(profileOperationContext(ctx), ctx.Profile)
	}
	source, backend := "", info.Backend
	configured := info.Configured
	if configured {
		source = "credentials"
	}
	if environment {
		source, backend, configured = "env", "environment", true
	}
	report := authorization.Resolve(info.Authorization, source, configured)
	var problem *profileProblem
	var resultErr error
	if inspectErr != nil {
		problem, resultErr = profileIssue(inspectErr), inspectErr
	} else if authErr := report.CheckCredential(); authErr != nil {
		problem = profileIssue(authErr)
		resultErr = &CodeError{Code: exitAuth, Err: &profileCommandError{Code: problem.Code, Message: problem.Message, Profile: ctx.Profile, Authorization: &report}}
	} else if !configured {
		problem = &profileProblem{Code: "PROFILE_NOT_FOUND", Message: "The selected credential profile has no active stored credential; log in again or select another profile."}
		resultErr = &CodeError{Code: exitNotFound, Err: &profileCommandError{Code: problem.Code, Message: problem.Message, Profile: ctx.Profile}}
	}
	payload := map[string]any{"selected_profile": ctx.Profile, "selection_source": profileSelectionSource(ctx), "profile_active": !environment && configured && resultErr == nil, "configured": configured, "source": source, "backend": backend, "accessibility": "unchecked", "recovery": info.Recovery, "authorization": report}
	if problem != nil {
		payload["error"] = problem
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		if err := writeStructuredValue(ctx, payload, output.Meta{}); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(ctx.Stdout, "Selected credential profile: %q (source: %s).\n", ctx.Profile, profileSelectionSource(ctx))
		if environment {
			fmt.Fprintln(ctx.Stdout, "Active credential: TODOIST_TOKEN; the selected profile is inactive and its metadata was not inspected.")
		} else {
			fmt.Fprintf(ctx.Stdout, "Stored credential: configured=%t; backend=%s; accessibility unchecked.\n", configured, backend)
		}
		fmt.Fprintln(ctx.Stdout, "Authorization: "+report.Summary())
		if info.Recovery != "" {
			fmt.Fprintf(ctx.Stdout, "Recovery: %s; run `%s`.\n", info.Recovery, credentialCommand(ctx, ctx.Profile, "auth", "repair"))
		}
		if problem != nil {
			fmt.Fprintf(ctx.Stdout, "%s: %s\n", problem.Code, problem.Message)
		}
	}
	return resultErr
}

var persistProfileSelection = func(ctx context.Context, path, profile string) error {
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	unlock, err := (credentials.Disk{}).Lock(bounded, filepath.Join(filepath.Dir(path), ".todoist-config.lock"))
	if err != nil {
		return &config.SelectionError{}
	}
	defer unlock()
	return config.SetDefaultProfile(path, profile, nil)
}

func profileUseCommand(ctx *Context, args []string) error {
	name, help, err := profileArguments(ctx, "use", args, true)
	if err != nil || help {
		return err
	}
	info, err := profileStore(ctx).Inspect(profileOperationContext(ctx), name)
	if err != nil {
		return err
	}
	if !info.Configured {
		return &CodeError{Code: exitNotFound, Err: &profileCommandError{Code: "PROFILE_NOT_FOUND", Message: "The requested credential profile has no active stored credential; log in to it before selecting it.", Profile: name}}
	}
	report := authorization.Resolve(info.Authorization, "credentials", true)
	if err := report.CheckCredential(); err != nil {
		problem := profileIssue(err)
		return &CodeError{Code: exitAuth, Err: &profileCommandError{Code: problem.Code, Message: problem.Message, Profile: name, Authorization: &report}}
	}
	if err := persistProfileSelection(profileOperationContext(ctx), ctx.ConfigPath, name); err != nil {
		return &CodeError{Code: exitError, Err: err}
	}
	selected, source := resolveProfileSelection(ctx.Global.Profile, ctx.ProjectDefaultProfile, name)
	shadowed := source != "user"
	payload := map[string]any{"profile": name, "saved": true, "config_path": ctx.ConfigPath, "selected_profile": selected, "selection_source": source, "shadowed": shadowed, "environment_token_active": os.Getenv("TODOIST_TOKEN") != ""}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, payload, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Saved default credential profile %q in %s.\n", name, ctx.ConfigPath)
	if shadowed {
		fmt.Fprintf(ctx.Stdout, "This invocation selects %q from %s; that setting overrides the saved user default.\n", selected, source)
	}
	if os.Getenv("TODOIST_TOKEN") != "" {
		fmt.Fprintln(ctx.Stdout, "TODOIST_TOKEN remains the active credential and overrides stored profile selection.")
	}
	return nil
}

func profileRemoveCommand(ctx *Context, args []string) error {
	name, help, err := profileArguments(ctx, "remove", args, true)
	if err != nil || help {
		return err
	}
	if err := profileStore(ctx).Delete(profileOperationContext(ctx), name); err != nil {
		return &profileStorageError{Profile: name, Err: err}
	}
	payload := map[string]any{"profile": name, "removed": true, "selected_profile": ctx.Profile, "selection_source": profileSelectionSource(ctx), "selection_retained": true, "environment_token_active": os.Getenv("TODOIST_TOKEN") != ""}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, payload, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Removed stored credential profile %q. Saved defaults were retained; no other credential was selected.\n", name)
	if name == ctx.Profile {
		fmt.Fprintln(ctx.Stdout, "The selected profile now has no stored credential. Select another profile explicitly with `todoist profile use NAME`, or log in again.")
	}
	if os.Getenv("TODOIST_TOKEN") != "" {
		fmt.Fprintln(ctx.Stdout, "TODOIST_TOKEN remains active; profile removal does not unset it or revoke a Todoist grant.")
	}
	return nil
}
