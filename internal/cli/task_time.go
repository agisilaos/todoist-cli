package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

func taskReschedule(ctx *Context, args []string) error {
	fs := newFlagSet("task reschedule")
	var id, date, instant, local string
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.StringVar(&date, "due-date", "", "Date retaining existing clock")
	fs.StringVar(&instant, "due-datetime", "", "Exact RFC3339 instant for fixed-zone tasks")
	fs.StringVar(&local, "due-local-datetime", "", "Floating local datetime")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	count := 0
	idPresent := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "id" {
			idPresent = true
		}
		if f.Name == "due-date" || f.Name == "due-datetime" || f.Name == "due-local-datetime" {
			count++
		}
	})
	if idPresent && strings.TrimSpace(id) == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("--id cannot be empty")}
	}
	if count != 1 || date == "" && instant == "" && local == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("reschedule requires exactly one nonempty due target")}
	}
	for _, field := range []struct{ value, layout string }{{date, "2006-01-02"}, {instant, time.RFC3339Nano}, {local, "2006-01-02T15:04:05"}} {
		if field.value != "" {
			var err error
			if field.layout == time.RFC3339Nano {
				_, err = apptasks.ParseInstant(field.value)
			} else if field.layout == "2006-01-02T15:04:05" {
				_, err = apptasks.ParseLocalDatetime(field.value)
			} else {
				_, err = time.Parse(field.layout, field.value)
			}
			if err != nil {
				return &CodeError{Code: exitUsage, Err: err}
			}
		}
	}
	if id != "" && len(fs.Args()) > 0 || id == "" && len(fs.Args()) == 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("reschedule requires one task target")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	id, task, err := resolveEditingTarget(ctx, id, strings.Join(fs.Args(), " "), true)
	if err != nil {
		return err
	}
	due, err := apptasks.RescheduleDue(*task, date, instant, local)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("cannot preserve rescheduling semantics: %w", err)}
	}
	body := map[string]any{"id": id, "due": due}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task reschedule", body)
	}
	raw, _, err := nativeTaskCommand(ctx, "item_update", body)
	if err != nil {
		return err
	}
	return writeReturnedTask(ctx, raw, id, "task_reschedule", false)
}
