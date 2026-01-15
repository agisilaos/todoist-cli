package cli

import (
	"fmt"
)

func agentExamples(ctx *Context) error {
	fmt.Fprint(ctx.Stdout, `Examples:
  # Weekly triage: plan, review, then apply with confirm token
  todoist agent plan "Triage inbox and plan my week" --out plan.json
  todoist agent apply --plan plan.json --confirm $(jq -r .confirm_token plan.json)

  # Weekly reading shuffle (use planner to pick 3 items)
  todoist agent run --instruction "Move 3 articles from Learning to Today"

  # Schedule a weekly run on Saturday at 09:00 (macOS launchd)
  todoist agent schedule print --weekly "sat 09:00" --instruction "Move 3 articles from Learning to Today" > ~/Library/LaunchAgents/com.todoist.agent.weekly.plist
`)
	return nil
}
