package cli

import (
	"fmt"
)

func agentExamples(ctx *Context) error {
	fmt.Fprint(ctx.Stdout, `Examples:
  # Weekly triage: plan, review, then apply with confirm token
  todoist agent plan "Triage inbox and plan my week" --out plan.json
  todoist agent apply --plan plan.json --confirm "$(jq -r .confirm_token plan.json)"

  # Preview a weekly reading shuffle (use planner to pick 3 items)
  todoist agent run --instruction "Move 3 articles from Learning to Today" --force --dry-run

  # Print a weekly preview schedule for Saturday at 09:00 (inspect before installing)
  todoist agent schedule print --weekly "sat 09:00" --instruction "Move 3 articles from Learning to Today" --force --dry-run > weekly-preview.plist

  # --force satisfies confirmation; --dry-run previews without CLI mutations.
  # Unattended application requires deliberate --force use without --dry-run.
  # Use --policy to constrain permitted actions, or review a saved plan first.
`)
	return nil
}
