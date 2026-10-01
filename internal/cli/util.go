package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type multiValue []string

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func bindHelpFlag(fs *flag.FlagSet, help *bool) {
	if fs == nil || help == nil {
		return
	}
	fs.BoolVar(help, "help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
}

func (m *multiValue) String() string {
	return strings.Join(*m, ",")
}

func (m *multiValue) Set(value string) error {
	if value == "" {
		return nil
	}
	*m = append(*m, value)
	return nil
}

func requireIDArg(name string, args []string) (string, error) {
	fs := newFlagSet(name)
	var id string
	fs.StringVar(&id, "id", "", "ID")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if id == "" {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id", name)}
	}
	return stripIDPrefix(id), nil
}

func requireEntityIDArg(name, entity string, args []string) (string, error) {
	fs := newFlagSet(name)
	var id string
	fs.StringVar(&id, "id", "", "ID")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if id == "" {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id", name)}
	}
	normalized, directID, err := apprefs.NormalizeEntityRef(id, entity)
	if err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if strings.TrimSpace(normalized) == "" {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id", name)}
	}
	if !directID {
		return strings.TrimSpace(id), nil
	}
	return normalized, nil
}

func requireTaskID(ctx *Context, name string, args []string) (string, error) {
	fs := newFlagSet(name)
	var id string
	fs.StringVar(&id, "id", "", "Task ID")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if id != "" {
		normalized, directID, err := apprefs.NormalizeEntityRef(id, "task")
		if err != nil {
			return "", &CodeError{Code: exitUsage, Err: err}
		}
		if strings.TrimSpace(normalized) == "" {
			return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id or a reference", name)}
		}
		if !directID {
			return strings.TrimSpace(id), nil
		}
		return normalized, nil
	}
	if len(fs.Args()) == 0 {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id or a reference", name)}
	}
	if err := ensureClient(ctx); err != nil {
		return "", err
	}
	ref := strings.Join(fs.Args(), " ")
	svc := apptasks.Service{Resolver: cliTaskResolver{ctx: ctx}}
	resolvedID, err := svc.ResolveTaskTarget(context.Background(), apptasks.ResolveTaskTargetInput{Ref: ref})
	if err != nil {
		return "", err
	}
	return resolvedID, nil
}

// writeStructuredValue emits a single result, preserving the JSON payload in NDJSON.
func writeStructuredValue(ctx *Context, value any, meta output.Meta) error {
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSON(ctx.Stdout, []any{value})
	}
	return output.WriteJSON(ctx.Stdout, value, meta)
}

func writeDryRun(ctx *Context, action string, payload any) error {
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, map[string]any{
			"action":        action,
			"payload":       payload,
			"dry_run":       true,
			"authorization": currentAuthorization(ctx),
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "dry run: %s; %s\n", action, currentAuthorization(ctx).Summary())
	return nil
}

func writeSimpleResult(ctx *Context, status, id string) error {
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, map[string]any{
			"id":     id,
			"status": status,
		}, output.Meta{RequestID: ctx.RequestID})
	}
	fmt.Fprintf(ctx.Stdout, "%s %s\n", status, id)
	return nil
}

func setRequestID(ctx *Context, requestID string) {
	if requestID != "" {
		ctx.RequestID = requestID
		if ctx != nil && ctx.Global.Verbose && ctx.Stderr != nil {
			fmt.Fprintf(ctx.Stderr, "request_id=%s\n", requestID)
		}
	}
}

func ctxRequestIDValue(ctx *Context) string {
	return ctx.RequestID
}

func writeError(ctx *Context, err error) {
	if err == nil {
		return
	}
	meta := output.Meta{RequestID: ctxRequestIDValue(ctx)}
	if meta.RequestID == "" {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.RequestID != "" {
			meta.RequestID = apiErr.RequestID
		}
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeIDsOnly {
		details := map[string]any(nil)
		var ambiguousErr *AmbiguousMatchError
		if errors.As(err, &ambiguousErr) {
			details = map[string]any{
				"type":    "ambiguous_match",
				"entity":  ambiguousErr.Entity,
				"input":   ambiguousErr.Input,
				"matches": ambiguousErr.Matches,
			}
		}
		enc := json.NewEncoder(ctx.Stderr)
		if !ctx.Global.QuietJSON {
			enc.SetIndent("", "  ")
		}
		payload := map[string]any{
			"error": safeErrorText(ctx, err),
			"meta":  meta,
		}
		var authorizationErr *authorization.Error
		if errors.As(err, &authorizationErr) {
			payload["code"] = authorizationErr.Code
			details = map[string]any{"profile": ctx.Profile, "source": ctx.TokenSource, "authorization": currentAuthorization(ctx)}
			if authorizationErr.Code == "OAUTH_SCOPE_INVALID" {
				// Scope rejection describes a new grant, not the credential it would replace.
				details = map[string]any{"profile": ctx.Profile}
			}
			if authorizationErr.Reason != "" {
				details["reason"] = authorizationErr.Reason
			}
		}
		var storageErr *credentials.Error
		if errors.As(err, &storageErr) {
			payload["code"] = storageErr.Kind
			details = storageErrorDetails(err)
			var targetErr *profileStorageError
			if errors.As(err, &targetErr) {
				if details == nil {
					details = map[string]any{}
				}
				details["profile"] = targetErr.Profile
				details["operation"] = "remove"
				if storageErr.Kind == credentials.Cleanup || storageErr.Kind == credentials.Recovery {
					details["retry_command"] = credentialCommand(ctx, "", "profile", "remove", targetErr.Profile)
					details["repair_command"] = credentialCommand(ctx, targetErr.Profile, "auth", "repair")
				}
			}
		}
		var profileErr *profileCommandError
		if errors.As(err, &profileErr) {
			payload["code"] = profileErr.Code
			details = map[string]any{"profile": profileErr.Profile}
			if profileErr.Authorization != nil {
				details["authorization"] = profileErr.Authorization
			}
		}
		var oauthErr *oauthError
		if errors.As(err, &oauthErr) {
			payload["code"] = oauthErr.Code
			// This failure concerns the candidate grant, never the active credential.
			details = nil
		}
		if details != nil {
			payload["details"] = details
		}
		_ = enc.Encode(payload)
		return
	}
	if meta.RequestID != "" {
		fmt.Fprintf(ctx.Stderr, "error: %s (request_id=%s)\n", safeErrorText(ctx, err), meta.RequestID)
	} else {
		fmt.Fprintf(ctx.Stderr, "error: %s\n", safeErrorText(ctx, err))
	}
	var unknown *unknownCommandError
	if humanCommandHints(ctx.Global) && errors.As(err, &unknown) {
		printCommandHints(ctx.Stderr, unknown.parent, unknown.input)
	}
	writeRecoveryHints(ctx, err)
	var denied *authorization.Error
	if errors.As(err, &denied) && denied.Code == "READ_ONLY" {
		fmt.Fprintln(ctx.Stderr, "Select a write-capable profile, or log in with --oauth without --read-only. --force cannot override authorization.")
	}
}

func requireNonEmpty(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New(field + " is required")}
	}
	return nil
}

func terminalWidth() int {
	if env := os.Getenv("COLUMNS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil && val > 0 {
			return val
		}
	}
	return 120
}

func tableWidth(ctx *Context) int {
	if ctx != nil && ctx.Config.TableWidth > 0 {
		return ctx.Config.TableWidth
	}
	return terminalWidth()
}

func cleanCell(value string) string {
	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	return strings.TrimSpace(replacer.Replace(value))
}

func truncateString(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func shortID(value string, max int, wide bool) string {
	if wide || value == "" {
		return value
	}
	return truncateString(value, max)
}

func useFuzzy(ctx *Context) bool {
	return ctx != nil && ctx.Fuzzy
}

func useAccessible(ctx *Context) bool {
	return ctx != nil && ctx.Accessible
}

func stripIDPrefix(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "id:") {
		return strings.TrimSpace(value[3:])
	}
	return value
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func canonicalSubcommand(input string, aliases map[string]string) string {
	if aliases == nil {
		return input
	}
	if resolved, ok := aliases[input]; ok {
		return resolved
	}
	return input
}

type boolFlag interface {
	IsBoolFlag() bool
}

func parseFlagSetInterspersed(fs *flag.FlagSet, args []string) error {
	normalized, err := normalizeInterspersedArgs(fs, args)
	if err != nil {
		return err
	}
	return fs.Parse(normalized)
}

func normalizeInterspersedArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	flagArgs := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name, hasValue := splitFlagName(arg)
		if name == "" {
			positional = append(positional, arg)
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			return nil, fmt.Errorf("flag provided but not defined: %s", arg)
		}
		flagArgs = append(flagArgs, arg)
		if hasValue || isFlagBool(f.Value) {
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag needs an argument: --%s", name)
		}
		i++
		flagArgs = append(flagArgs, args[i])
	}
	return append(flagArgs, positional...), nil
}

func splitFlagName(arg string) (string, bool) {
	if strings.HasPrefix(arg, "--") {
		name := strings.TrimPrefix(arg, "--")
		if name == "" {
			return "", false
		}
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			return name[:idx], true
		}
		return name, false
	}
	if strings.HasPrefix(arg, "-") {
		name := strings.TrimPrefix(arg, "-")
		if name == "" {
			return "", false
		}
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			return name[:idx], true
		}
		return name, false
	}
	return "", false
}

func isFlagBool(v flag.Value) bool {
	bf, ok := v.(boolFlag)
	return ok && bf.IsBoolFlag()
}

func safeErrorText(ctx *Context, err error) string {
	text := err.Error()
	if ctx != nil && ctx.Token != "" {
		text = strings.ReplaceAll(text, ctx.Token, "[REDACTED]")
	}
	return text
}
