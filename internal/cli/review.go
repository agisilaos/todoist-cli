package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
	appreview "github.com/agisilaos/todoist-cli/internal/app/review"
)

var errReviewCancelled = errors.New("review cancelled")

type reviewInput struct {
	lines chan string
	done  chan error
	ctx   context.Context
}

func newReviewInput(ctx context.Context, r io.Reader) *reviewInput {
	input := &reviewInput{lines: make(chan string), done: make(chan error, 1), ctx: ctx}
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			select {
			case input.lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = io.EOF
		}
		input.done <- err
	}()
	return input
}
func (in *reviewInput) ask(ctx *Context, prompt string) (string, error) {
	fmt.Fprint(ctx.Stderr, prompt)
	if in.ctx.Err() != nil {
		return "", errReviewCancelled
	}
	select {
	case <-in.ctx.Done():
		return "", errReviewCancelled
	case err := <-in.done:
		return "", fmt.Errorf("review input ended: %w", err)
	case line := <-in.lines:
		line = strings.TrimSpace(line)
		if line == "cancel" || line == "quit" {
			return "", errReviewCancelled
		}
		return line, nil
	}
}

func reviewCommand(ctx *Context, args []string) error {
	fs := newFlagSet("review")
	var filter, out string
	var help bool
	fs.StringVar(&filter, "filter", "overdue | today", "Todoist filter")
	fs.StringVar(&out, "out", "", "Save finished review plan")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printReviewHelp(ctx.Stdout)
		return nil
	}
	if len(fs.Args()) > 0 || strings.TrimSpace(filter) == "" || out == "-" {
		return &CodeError{Code: exitUsage, Err: errors.New("review accepts --filter and --out <file>; no positional arguments")}
	}
	if ctx.Global.NoInput || !isTTYReader(ctx.Stdin) {
		return &CodeError{Code: exitUsage, Err: errors.New("review requires terminal input; use task list --filter and agent plan/apply for noninteractive work")}
	}
	if ctx.Global.Force {
		return &CodeError{Code: exitUsage, Err: errors.New("review requires explicit confirmation; remove --force")}
	}
	operation, stop := signal.NotifyContext(operationContext(ctx), os.Interrupt)
	defer stop()
	prior := ctx.OperationContext
	ctx.OperationContext = operation
	defer func() { ctx.OperationContext = prior }()
	if err := ensureClient(ctx); err != nil {
		return err
	}
	return runReview(ctx, filter, out, newReviewInput(operation, ctx.Stdin))
}

func selectReviewTasks(ctx *Context, filter string) ([]appreview.Snapshot, error) {
	tasks := []appreview.Snapshot{}
	ids := map[string]bool{}
	cursors := map[string]bool{}
	cursor := ""
	for {
		req, cancel := requestContext(ctx)
		query := url.Values{"query": {filter}, "limit": {"200"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var page api.Paginated[appreview.Snapshot]
		_, err := ctx.Client.Get(req, "/tasks/filter", query, &page)
		cancel()
		if err != nil {
			return nil, err
		}
		for _, task := range page.Results {
			id := appreview.Text(task, "id")
			if id == "" {
				return nil, errors.New("review received a task without an ID")
			}
			if !ids[id] {
				ids[id] = true
				tasks = append(tasks, appreview.SnapshotOf(task))
			}
		}
		if page.NextCursor == "" {
			break
		}
		if cursors[page.NextCursor] {
			return nil, errors.New("review pagination repeated a cursor; no review started")
		}
		cursors[page.NextCursor] = true
		cursor = page.NextCursor
	}
	appreview.Sort(tasks)
	return tasks, nil
}

func runReview(ctx *Context, filter, out string, input *reviewInput) error {
	plan := Plan{Version: 1, Instruction: "Daily review: " + filter, CreatedAt: ctx.Now().UTC().Format(time.RFC3339), ConfirmToken: api.NewRequestID(), Actions: []Action{}, Review: &coreagent.Review{Version: 1, Filter: filter, Tasks: []coreagent.ReviewTask{}}}
	if err := validateReviewOutputPath(ctx, out); err != nil {
		return err
	}
	tasks, err := selectReviewTasks(ctx, filter)
	if err != nil {
		if ctx.OperationContext != nil && ctx.OperationContext.Err() != nil && errors.Is(err, context.Canceled) {
			return writeReviewReport(ctx, plan, nil, "cancelled", "", errReviewCancelled)
		}
		return err
	}
	for _, task := range tasks {
		plan.Review.Tasks = append(plan.Review.Tasks, coreagent.ReviewTask{ID: appreview.Text(task, "id"), Content: appreview.Text(task, "content"), Snapshot: task, Actions: []int{}})
	}
	finish := func(phase string, cause error) error {
		if reportErr := writeReviewReport(ctx, plan, nil, phase, "", cause); reportErr != nil {
			return reportErr
		}
		if errors.Is(cause, errReviewCancelled) {
			return nil
		}
		return cause
	}
	if len(tasks) == 0 {
		return finish("empty", nil)
	}
	// Names are context only; unavailable lookups leave explicit IDs visible.
	if _, err := listAllProjects(ctx); err != nil {
		fmt.Fprintln(ctx.Stderr, "Project names unavailable; showing IDs.")
	}
	if _, err := listAllSections(ctx, ""); err != nil {
		fmt.Fprintln(ctx.Stderr, "Section names unavailable; showing IDs.")
	}
	fmt.Fprintf(ctx.Stderr, "Review set: %d tasks. Nothing changes until final confirmation. Type cancel at any prompt.\nAuthorization: %s\n", len(tasks), currentAuthorization(ctx).Summary())
	for i := range plan.Review.Tasks {
		task := &plan.Review.Tasks[i]
		fmt.Fprintf(ctx.Stderr, "\nTask %d of %d\n", i+1, len(tasks))
		showReviewTask(ctx, task.Snapshot)
		for {
			choice, err := input.ask(ctx, "Disposition [keep/change/complete/skip]: ")
			if err != nil {
				return finish("cancelled", err)
			}
			switch choice {
			case "keep", "skip":
				task.Disposition = choice
			case "complete":
				fmt.Fprintln(ctx.Stderr, "Complete this occurrence for recurring tasks; ordinary completion also completes subtasks.")
				task.Disposition = choice
				task.Actions = append(task.Actions, len(plan.Actions))
				plan.Actions = append(plan.Actions, Action{Type: "task_complete", TaskID: task.ID})
			case "change":
				actions, err := editReviewTask(ctx, input, *task)
				if err != nil {
					return finish("cancelled", err)
				}
				if len(actions) == 0 {
					fmt.Fprintln(ctx.Stderr, "No edits selected; choose keep or skip.")
					continue
				}
				task.Disposition = choice
				for _, a := range actions {
					task.Actions = append(task.Actions, len(plan.Actions))
					plan.Actions = append(plan.Actions, a)
				}
			default:
				fmt.Fprintln(ctx.Stderr, "Choose keep, change, complete, or skip.")
				continue
			}
			break
		}
	}
	plan.Summary = summarizeActions(plan.Actions)
	if err := validatePlan(plan, 1, true); err != nil {
		return finish("preview", err)
	}
	policy, err := loadAgentPolicy(ctx, "")
	if err != nil {
		return finish("preview", err)
	}
	if err := enforceAgentPolicy(plan, policy); err != nil {
		return finish("preview", err)
	}
	showReviewPreview(ctx, plan)
	if out != "" {
		if err := saveReviewPlan(out, plan); err != nil {
			return finish("preview", err)
		}
		fmt.Fprintf(ctx.Stderr, "Saved plan: %s\n", out)
	}
	if ctx.Global.DryRun || !currentAuthorization(ctx).WriteCapable || len(plan.Actions) == 0 {
		return writeReviewReport(ctx, plan, nil, "preview", out, nil)
	}
	for {
		answer, err := input.ask(ctx, "Apply this plan? [yes/no]: ")
		if err != nil {
			return finish("cancelled", err)
		}
		if answer == "no" || answer == "n" {
			return finish("cancelled", nil)
		}
		if answer != "yes" {
			fmt.Fprintln(ctx.Stderr, "Type yes to apply, or no to cancel.")
			continue
		}
		break
	}
	if out == "" {
		out = filepath.Join(filepath.Dir(ctx.ConfigPath), "review-plans", plan.ConfirmToken+".json")
		if err := saveReviewPlan(out, plan); err != nil {
			return finish("preview", err)
		}
	}
	fmt.Fprintf(ctx.Stderr, "Recovery plan: %s\nRetry definite failures with: todoist agent apply --plan %q --confirm %q\nUse the same --config, profile/account and base URL. Inspect uncertain outcomes before starting a fresh review.\n", out, out, plan.ConfirmToken)
	return applyReviewAndReport(ctx, plan, out, "fail", "review")
}

func saveReviewPlan(path string, plan Plan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Never replace an existing recovery plan, including through a symlink.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	return closeErr
}

func showReviewTask(ctx *Context, task appreview.Snapshot) {
	for _, key := range []string{"id", "content", "description", "project_id", "section_id", "parent_id", "labels", "priority", "assignee_id", "responsible_uid", "due", "deadline", "duration"} {
		value := task[key]
		if len(value) == 0 {
			value = json.RawMessage("null")
		}
		fmt.Fprintf(ctx.Stderr, "%s: %s\n", key, value)
	}
	if cache := ctx.lookupCache; cache != nil {
		for _, p := range cache.projects {
			if p.ID == appreview.Text(task, "project_id") {
				fmt.Fprintf(ctx.Stderr, "Project name: %q\n", p.Name)
			}
		}
		for _, sections := range cache.sectionsByProject {
			for _, section := range sections {
				if section.ID == appreview.Text(task, "section_id") {
					fmt.Fprintf(ctx.Stderr, "Section name: %q\n", section.Name)
				}
			}
		}
	}
	fmt.Fprintln(ctx.Stderr, "Due dates without a time are all-day; a time without a timezone is floating local time. Priority uses API 4 (p1, highest) through 1 (p4).")
}

func showReviewPreview(ctx *Context, plan Plan) {
	fmt.Fprintln(ctx.Stderr, "\nREVIEW PLAN — proposed only; no mutations yet.")
	for _, task := range plan.Review.Tasks {
		fmt.Fprintf(ctx.Stderr, "%s %q: %s\n", task.ID, task.Content, task.Disposition)
		if len(task.Actions) > 0 {
			fmt.Fprintln(ctx.Stderr, "Before:")
			showReviewTask(ctx, task.Snapshot)
			for _, idx := range task.Actions {
				data, _ := json.Marshal(plan.Actions[idx])
				fmt.Fprintf(ctx.Stderr, "Proposed: %s\n", data)
			}
		}
	}
	fmt.Fprintln(ctx.Stderr, "Natural-language due values are unresolved until Todoist applies them. Recurring due edits may change recurrence. Application is not transactional; only one applying process is supported.")
}

func printReviewHelp(out io.Writer) {
	fmt.Fprint(out, `Usage:
  todoist review [--filter <query>] [--out <file>]

Review all overdue or today tasks across projects, or an explicit Todoist filter.
Fetch all pages before prompting; freeze and deduplicate the set, ordered by due,
priority, then ID. This is not an atomic service snapshot.

Choose keep (intentionally unchanged), change, complete, or skip (defer).
Change opens a numbered field menu; choose done when finished. Moves are supported.
Completion advances recurring tasks; ordinary completion also completes subtasks.
The final preview shows current values and proposed actions before asking yes/no.
Type cancel at any prompt. EOF fails safely; Ctrl-C cancels before application.

--out <file>  Save finished agent-compatible plan without overwriting a file.
--dry-run     Review and preview only; no Todoist mutations.
--json        Final review report on stdout; prompts/previews on stderr.
--ndjson      One final review report record, with the same JSON payload.

Read-only credentials may review and save plans, but cannot apply mutations.
--no-input and piped stdin are rejected; use task list and agent plan/apply.
--force is rejected. Blank input never selects a disposition or confirms changes.
No session resume or backtracking. Each change can edit fields then move the task.
Before applying, a recovery plan is saved under the config directory/review-plans.
Retry a definite failure with agent apply --plan <file> --confirm <token>, using
this binary, the same config and account. Recorded actions are skipped; pending
tasks are checked against their original or checkpointed state. Changed/missing
tasks block application. An uncertain write requires inspection and a fresh review.
Only one applying process is supported; no transactional/exactly-once guarantee.
Do not edit saved recovery plans or use older binaries to apply review plans.
`)
}

// Keep the single input reader authoritative: reference resolution must never prompt.
func editReviewTask(ctx *Context, input *reviewInput, task coreagent.ReviewTask) ([]Action, error) {
	update := Action{Type: "task_update", TaskID: task.ID}
	move := Action{Type: "task_move", TaskID: task.ID}
	edited := false
	moved := false
	resolver := *ctx
	resolver.Global.NoInput = true
	fields := []string{"content", "description", "labels", "priority", "due", "due-date", "due-datetime", "due-lang", "duration", "deadline", "assignee", "project", "section", "parent"}
	for {
		fmt.Fprintln(ctx.Stderr, "Edit fields (name or number); done finishes. Empty values leave the field unchanged; clearing fields is not supported.")
		for i, f := range fields {
			fmt.Fprintf(ctx.Stderr, "%d %s\n", i+1, f)
		}
		field, err := input.ask(ctx, "Field [done]: ")
		if err != nil {
			return nil, err
		}
		if field == "done" {
			break
		}
		if n, err := strconv.Atoi(field); err == nil && n >= 1 && n <= len(fields) {
			field = fields[n-1]
		}
		valid := false
		for _, f := range fields {
			if field == f {
				valid = true
			}
		}
		if !valid {
			fmt.Fprintln(ctx.Stderr, "Choose a listed field or done.")
			continue
		}
		if field == "due" || field == "due-date" || field == "due-datetime" {
			fmt.Fprintln(ctx.Stderr, "Due changes may replace recurrence. Natural language resolves only during application.")
		}
		value, err := input.ask(ctx, field+" value: ")
		if err != nil {
			return nil, err
		}
		if value == "" {
			continue
		}
		candidate := update
		destination := move
		switch field {
		case "content":
			candidate.Content = value
		case "description":
			candidate.Description = value
		case "labels":
			candidate.Labels = nil
			for _, label := range strings.Split(value, ",") {
				label = strings.TrimSpace(label)
				if label != "" {
					candidate.Labels = append(candidate.Labels, label)
				}
			}
			if len(candidate.Labels) == 0 {
				err = errors.New("enter comma-separated label names")
			}
		case "priority":
			if len(value) == 2 && value[0] == 'p' && value[1] >= '1' && value[1] <= '4' {
				candidate.Priority = 5 - int(value[1]-'0')
			} else {
				candidate.Priority, err = strconv.Atoi(value)
			}
			if candidate.Priority < 1 || candidate.Priority > 4 {
				err = errors.New("priority must be p1..p4 or API 1..4")
			}
		case "due":
			candidate.Due = value
			candidate.DueDate = ""
			candidate.DueDatetime = ""
		case "due-date":
			_, err = time.Parse("2006-01-02", value)
			candidate.DueDate = value
			candidate.Due = ""
			candidate.DueDatetime = ""
		case "due-datetime":
			_, err = time.Parse(time.RFC3339, value)
			candidate.DueDatetime = value
			candidate.Due = ""
			candidate.DueDate = ""
		case "due-lang":
			candidate.DueLang = value
		case "duration":
			parts := strings.Fields(value)
			candidate.Duration, err = strconv.Atoi(parts[0])
			candidate.DurationUnit = "minute"
			if len(parts) > 1 {
				candidate.DurationUnit = parts[1]
			}
			if len(parts) > 2 || candidate.Duration < 1 || (candidate.DurationUnit != "minute" && candidate.DurationUnit != "day") {
				err = errors.New("duration must be a positive integer followed by minute or day")
			}
		case "deadline":
			_, err = time.Parse("2006-01-02", value)
			candidate.Deadline = value
		case "assignee":
			candidate.Assignee, err = resolveAssigneeSelector(&resolver, "", value, appreview.Text(task.Snapshot, "project_id"), task.ID)
		case "project":
			destination = Action{Type: "task_move", TaskID: task.ID}
			destination.ProjectID, err = resolveReviewDestination(&resolver, "project", value)
		case "section":
			destination = Action{Type: "task_move", TaskID: task.ID}
			destination.SectionID, err = resolveReviewDestination(&resolver, "section", value)
		case "parent":
			parent, lookupErr := resolveTaskRef(&resolver, value)
			err = lookupErr
			destination = Action{Type: "task_move", TaskID: task.ID, Parent: parent.ID}
			if err == nil && parent.ID == task.ID {
				err = errors.New("a task cannot be its own parent")
			}
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "Invalid value: %v\n", err)
			continue
		}
		switch field {
		case "project", "section", "parent":
			move = destination
			moved = true
			fmt.Fprintln(ctx.Stderr, "Move destination selected (replaces any earlier move destination).")
		default:
			update = candidate
			edited = true
		}
	}
	actions := []Action{}
	if edited {
		actions = append(actions, update)
	}
	if moved {
		actions = append(actions, move)
	}
	return actions, nil
}

func resolveReviewDestination(ctx *Context, kind, value string) (string, error) {
	normalized, direct, err := apprefs.NormalizeEntityRef(value, kind)
	if err != nil {
		return "", err
	}
	if direct {
		return normalized, nil
	}
	matches := []string{}
	if kind == "project" {
		projects, err := listAllProjects(ctx)
		if err != nil {
			return "", err
		}
		for _, p := range projects {
			if p.ID == normalized || strings.EqualFold(p.Name, normalized) {
				matches = append(matches, p.ID)
			}
		}
	} else {
		sections, err := listAllSections(ctx, "")
		if err != nil {
			return "", err
		}
		for _, section := range sections {
			if section.ID == normalized || strings.EqualFold(section.Name, normalized) {
				matches = append(matches, section.ID)
			}
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("%s reference %q has %d matches; use a listed name or id:<id>", kind, value, len(matches))
	}
	return matches[0], nil
}

func validateReviewOutputPath(ctx *Context, path string) error {
	if path == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	configAbs, err := filepath.Abs(ctx.ConfigPath)
	if err != nil {
		return err
	}
	parent, configParent := filepath.Dir(abs), filepath.Dir(configAbs)
	same := parent == configParent
	if a, err := os.Stat(parent); err == nil {
		if b, err := os.Stat(configParent); err == nil {
			same = same || os.SameFile(a, b)
		}
	}
	if same {
		for _, reserved := range []string{filepath.Base(configAbs), "credentials.json", "agent_replay.json", "last_plan.json", "agent_policy.json"} {
			if strings.EqualFold(filepath.Base(abs), reserved) {
				return &CodeError{Code: exitUsage, Err: fmt.Errorf("review output conflicts with reserved state file %s; choose a separate plan file", path)}
			}
		}
	}
	return nil
}
