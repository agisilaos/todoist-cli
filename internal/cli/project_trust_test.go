package cli

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestProjectConfigurationCannotChooseCredentialDestinationOrPlanner(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("TODOIST_TOKEN", "synthetic-token")
	t.Setenv("TODOIST_BASE_URL", "")
	t.Setenv("TODOIST_PLANNER_CMD", "")
	t.Setenv("TODOIST_PROFILE", "")
	project := filepath.Join(dir, ".todoist.json")
	if err := writeJSON(project, config.Config{BaseURL: "https://untrusted.invalid", PlannerCmd: "untrusted-command", TableWidth: 91}); err != nil {
		t.Fatal(err)
	}
	for _, configured := range []bool{false, true} {
		name := "default"
		if configured {
			name = "user"
		}
		t.Run(name, func(t *testing.T) {
			user := filepath.Join(dir, name, "config.json")
			cfg := config.Config{}
			wantURL, wantPlanner := "https://api.todoist.com/api/v1", ""
			if configured {
				cfg.BaseURL, cfg.PlannerCmd = "https://trusted.invalid", "trusted-command"
				wantURL, wantPlanner = cfg.BaseURL, cfg.PlannerCmd
			}
			if err := writeJSON(user, cfg); err != nil {
				t.Fatal(err)
			}
			ctx := &Context{Global: GlobalOptions{ConfigPath: user}}
			if err := loadConfig(ctx); err != nil {
				t.Fatal(err)
			}
			if ctx.Config.TableWidth != 91 {
				t.Fatal("ordinary project default lost")
			}
			planner, _ := resolvePlannerCmd(ctx, "", true)
			if planner != wantPlanner {
				t.Fatalf("planner = %q, want %q", planner, wantPlanner)
			}
			ctx.Client.SetTransport(testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != wantURL+"/projects" || req.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Fatalf("unexpected authenticated destination: %s", req.URL)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`[]`))}, nil
			}))
			var out any
			if _, err := ctx.Client.Get(context.Background(), "/projects", nil, &out); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Explicitly selecting the same file is an intentional trust decision.
	ctx := &Context{Global: GlobalOptions{ConfigPath: project}}
	if err := loadConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Config.BaseURL != "https://untrusted.invalid" || ctx.Config.PlannerCmd != "untrusted-command" {
		t.Fatal("explicit config selection lost")
	}
	t.Setenv("TODOIST_BASE_URL", "https://environment.invalid")
	t.Setenv("TODOIST_PLANNER_CMD", "environment-command")
	if err := loadConfig(ctx); err != nil {
		t.Fatal(err)
	}
	planner, _ := resolvePlannerCmd(ctx, "", true)
	if ctx.Config.BaseURL != "https://environment.invalid" || planner != "environment-command" {
		t.Fatal("environment overrides lost")
	}
	ctx.Global.BaseURL = "https://flag.invalid"
	if err := loadConfig(ctx); err != nil {
		t.Fatal(err)
	}
	planner, _ = resolvePlannerCmd(ctx, "flag-command", false)
	if ctx.Config.BaseURL != "https://flag.invalid" || planner != "flag-command" {
		t.Fatal("explicit flag overrides lost")
	}
}
