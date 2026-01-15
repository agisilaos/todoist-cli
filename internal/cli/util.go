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
	if err := fs.Parse(args); err != nil {
		return "", &CodeError{Code: exitUsage, Err: err}
	}
	if id == "" {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires --id", name)}
	}
	return stripIDPrefix(id), nil
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
