package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestResolvePlannerCmd(t *testing.T) {
	ctx := &Context{Config: config.Config{PlannerCmd: "echo hi"}}
	cmd, _ := resolvePlannerCmd(ctx, "", false)
	if cmd != "echo hi" {
		t.Fatalf("expected planner cmd from config, got %q", cmd)
	}
}

func TestAgentPlannerShowJSON(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{
		Stdout: &buf,
		Mode:   output.ModeJSON,
		Config: config.Config{PlannerCmd: "foo"},
	}
	if err := agentPlanner(ctx, []string{}); err != nil {
		t.Fatalf("agentPlanner show: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, `"planner_cmd": "foo"`) {
		t.Fatalf("unexpected output: %s", got)
	}
}
