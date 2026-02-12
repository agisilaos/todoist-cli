package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/output"
)

type multiValue []string

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
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
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

func requireTaskID(ctx *Context, name string, args []string) (string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	fs.StringVar(&id, "id", "", "Task ID")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if id != "" {
		return stripIDPrefix(id), nil
	}
	if len(fs.Args()) == 0 {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id or a reference", name)}
	}
	if err := ensureClient(ctx); err != nil {
		return "", err
	}
	ref := strings.Join(fs.Args(), " ")
	task, err := resolveTaskRef(ctx, ref)
	if err != nil {
		return "", err
	}
	return task.ID, nil
}

func writeDryRun(ctx *Context, action string, payload any) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"action":  action,
			"payload": payload,
			"dry_run": true,
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "dry run: %s\n", action)
	return nil
}

func writeSimpleResult(ctx *Context, status, id string) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
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
	if ctx.Mode == output.ModeJSON {
		enc := json.NewEncoder(ctx.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"error": err.Error(),
			"meta":  meta,
		})
		return
	}
	if meta.RequestID != "" {
		fmt.Fprintf(ctx.Stderr, "error: %s (request_id=%s)\n", err, meta.RequestID)
		return
	}
	fmt.Fprintf(ctx.Stderr, "error: %s\n", err)
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
