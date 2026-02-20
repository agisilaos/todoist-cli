package cli

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type doctorCheck struct {
	Name    string         `json:"name"`
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func doctorCommand(ctx *Context, args []string) error {
	fs := newFlagSet("doctor")
	var help bool
	var strict bool
	bindHelpFlag(fs, &help)
	fs.BoolVar(&strict, "strict", false, "Exit non-zero on warnings")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printDoctorHelp(ctx.Stdout)
		return nil
	}

	checks := runDoctorChecks(ctx)
	warnCount := 0
	failCount := 0
	for _, c := range checks {
		switch c.Status {
		case "warn":
			warnCount++
		case "fail":
			failCount++
		}
	}

	if err := writeDoctorReport(ctx, checks, warnCount, failCount); err != nil {
		return err
	}
	if failCount > 0 || (strict && warnCount > 0) {
		return &CodeError{Code: exitError, Err: fmt.Errorf("doctor checks failed (warn=%d fail=%d)", warnCount, failCount)}
	}
	return nil
}

func runDoctorChecks(ctx *Context) []doctorCheck {
	return []doctorCheck{
		checkConfigFiles(ctx),
		checkCredentials(ctx),
		checkAPIConnectivity(ctx),
		checkPlannerSetup(ctx),
		checkPolicyFile(ctx),
		checkReplayJournal(ctx),
	}
}

func checkConfigFiles(ctx *Context) doctorCheck {
	check := doctorCheck{Name: "config", Status: "ok", Message: "config loaded", Details: map[string]any{}}
	if ctx == nil || ctx.ConfigPath == "" {
		check.Status = "warn"
		check.Message = "config path unavailable"
		return check
	}
	check.Details["path"] = ctx.ConfigPath
	if info, err := os.Stat(ctx.ConfigPath); err == nil {
		check.Details["exists"] = true
		check.Details["mode"] = info.Mode().Perm().String()
	} else if os.IsNotExist(err) {
		check.Status = "warn"
		check.Message = "config file not found (defaults in use)"
		check.Details["exists"] = false
	} else {
		check.Status = "fail"
		check.Message = "cannot stat config file"
		check.Details["error"] = err.Error()
	}
	return check
}

func checkCredentials(ctx *Context) doctorCheck {
	check := doctorCheck{Name: "credentials", Status: "ok", Message: "token available", Details: map[string]any{}}
	if ctx == nil {
		check.Status = "fail"
		check.Message = "missing CLI context"
		return check
	}
	check.Details["profile"] = ctx.Profile
	check.Details["source"] = ctx.TokenSource
	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	check.Details["credentials_path"] = credsPath

	if info, err := os.Stat(credsPath); err == nil {
		check.Details["credentials_exists"] = true
		check.Details["credentials_mode"] = info.Mode().Perm().String()
		if info.Mode().Perm()&0o077 != 0 {
			check.Status = "warn"
			check.Message = "credentials file permissions should be 0600"
		}
	} else if os.IsNotExist(err) {
		check.Details["credentials_exists"] = false
	} else {
		check.Status = "warn"
		check.Message = "cannot read credentials file metadata"
		check.Details["error"] = err.Error()
	}

	if strings.TrimSpace(ctx.Token) == "" {
		check.Status = "warn"
		check.Message = "no token resolved; run `todoist auth login`"
	}
	return check
}

func checkAPIConnectivity(ctx *Context) doctorCheck {
	check := doctorCheck{Name: "api", Status: "warn", Message: "token not configured; connectivity skipped"}
	if ctx == nil || strings.TrimSpace(ctx.Token) == "" {
		return check
	}
	if err := ensureClient(ctx); err != nil {
		check.Status = "fail"
		check.Message = "failed to initialize API client"
		check.Details = map[string]any{"error": err.Error()}
		return check
	}
	query := url.Values{}
	query.Set("limit", "1")
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	var page api.Paginated[api.Project]
	reqID, err := ctx.Client.Get(reqCtx, "/projects", query, &page)
	if err != nil {
		check.Status = "fail"
		check.Message = "API probe failed"
		check.Details = map[string]any{"error": err.Error()}
		return check
	}
	setRequestID(ctx, reqID)
	check.Status = "ok"
	check.Message = "API reachable"
	check.Details = map[string]any{"request_id": reqID}
	return check
}

func checkPlannerSetup(ctx *Context) doctorCheck {
	cmd, source := resolvePlannerCmd(ctx, "", true)
	check := doctorCheck{Name: "planner", Status: "warn", Message: "planner command not configured", Details: map[string]any{"source": source}}
	if strings.TrimSpace(cmd) == "" {
		return check
	}
	check.Details["command"] = cmd
	bin := strings.Fields(cmd)
	if len(bin) == 0 {
		check.Status = "fail"
		check.Message = "planner command is empty"
		return check
	}
	if _, err := exec.LookPath(bin[0]); err != nil {
		check.Status = "warn"
		check.Message = "planner executable not found in PATH"
		check.Details["error"] = err.Error()
		return check
	}
	check.Status = "ok"
	check.Message = "planner command configured"
	return check
}

func checkPolicyFile(ctx *Context) doctorCheck {
	check := doctorCheck{Name: "policy", Status: "warn", Message: "policy file not found", Details: map[string]any{}}
	if ctx == nil || ctx.ConfigPath == "" {
		return check
	}
	path := filepath.Join(filepath.Dir(ctx.ConfigPath), "agent_policy.json")
	check.Details["path"] = path
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return check
		}
		check.Status = "fail"
		check.Message = "cannot stat policy file"
		check.Details["error"] = err.Error()
		return check
	}
	policy, err := loadAgentPolicy(ctx, path)
	if err != nil {
		check.Status = "fail"
		check.Message = "policy parse failed"
		check.Details["error"] = err.Error()
		return check
	}
	check.Status = "ok"
	check.Message = "policy file loaded"
	check.Details["allow_count"] = len(policy.AllowActionTypes)
	check.Details["deny_count"] = len(policy.DenyActionTypes)
	check.Details["max_destructive_actions"] = policy.MaxDestructiveActions
	return check
}

func checkReplayJournal(ctx *Context) doctorCheck {
	check := doctorCheck{Name: "replay", Status: "ok", Message: "replay journal readable", Details: map[string]any{}}
	journal, path, err := loadReplayJournal(ctx)
	check.Details["path"] = path
	if err != nil {
		check.Status = "fail"
		check.Message = "replay journal parse failed"
		check.Details["error"] = err.Error()
		return check
	}
	check.Details["entries"] = len(journal.Applied)
	if path == "" {
		check.Status = "warn"
		check.Message = "replay path unavailable"
	}
	return check
}

func writeDoctorReport(ctx *Context, checks []doctorCheck, warnCount, failCount int) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"checks": checks,
			"summary": map[string]any{
				"ok":    len(checks) - warnCount - failCount,
				"warn":  warnCount,
				"fail":  failCount,
				"total": len(checks),
			},
		}, output.Meta{RequestID: ctxRequestIDValue(ctx)})
	}
	rows := make([][]string, 0, len(checks))
	for _, c := range checks {
		rows = append(rows, []string{c.Name, c.Status, c.Message})
	}
	if err := output.WriteTable(ctx.Stdout, []string{"Check", "Status", "Message"}, rows); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(ctx.Stdout, "Summary: ok=%d warn=%d fail=%d\n", len(checks)-warnCount-failCount, warnCount, failCount)
	return nil
}

func printDoctorHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist doctor [--strict]

Checks:
  - config and credentials file status
  - auth token availability
  - API reachability (when token exists)
  - planner command configuration
  - default agent policy file parse
  - replay journal parse/readability

Flags:
  --strict   Exit non-zero when warnings exist
`)
}
