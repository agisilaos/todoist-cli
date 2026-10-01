package cli

import (
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type taskDueEvidence struct {
	kind     string
	value    time.Time
	zone     *time.Location
	timezone api.ResponseFact
}

var taskLocalPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?$`)

var taskInstantPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

func parseTaskInstant(value string) (time.Time, error) {
	if !taskInstantPattern.MatchString(value) {
		return time.Time{}, errors.New("invalid RFC3339 instant or unsupported precision")
	}
	return time.Parse(time.RFC3339Nano, value)
}
func parseDueValue(value string) (string, time.Time, error) {
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return "date", t, nil
	}
	if t, err := parseTaskInstant(value); err == nil {
		return "fixed", t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", value); err == nil && taskLocalPattern.MatchString(value) {
		return "floating", t, nil
	}
	return "", time.Time{}, errors.New("invalid returned due date")
}
func taskDueFacts(task api.Task, preserve bool) (taskDueEvidence, error) {
	var out taskDueEvidence
	value, ok := task.ResponseFact("due.date").Text()
	if !ok || value == "" {
		return out, errors.New("primary due date unavailable")
	}
	kind, t, err := parseDueValue(value)
	if err != nil {
		return out, err
	}
	out.timezone = task.ResponseFact("due.timezone")
	if out.timezone.State == api.ResponseInvalid {
		return out, errors.New("malformed returned timezone")
	}
	if zone, ok := out.timezone.Text(); ok {
		if zone == "" || zone == "Local" {
			return out, errors.New("empty returned timezone")
		}
		out.zone, err = time.LoadLocation(zone)
		if err != nil {
			return out, errors.New("unknown returned timezone")
		}
	}
	secondary := task.ResponseFact("due.datetime")
	if secondary.State == api.ResponseInvalid {
		return out, errors.New("malformed compatibility datetime")
	}
	if text, ok := secondary.Text(); ok {
		otherKind, other, err := parseDueValue(text)
		if err != nil || otherKind == "date" {
			return out, errors.New("invalid compatibility datetime")
		}
		localOther := other
		if otherKind == "fixed" && out.zone != nil {
			localOther = other.In(out.zone)
		}
		if kind == "date" {
			if localOther.Format("2006-01-02") != t.Format("2006-01-02") {
				return out, errors.New("contradictory due calendar date and datetime")
			}
			kind, t = otherKind, other
		} else if kind != otherKind || !t.Equal(other) {
			return out, errors.New("contradictory due date and datetime")
		}
	}
	if preserve {
		if kind == "fixed" && out.zone == nil || kind != "fixed" && out.timezone.State != api.ResponseNull {
			return out, errors.New("returned timezone cannot establish time character")
		}
	} else if kind != "fixed" && out.zone != nil {
		return out, errors.New("contradictory due timezone")
	}
	if kind == "fixed" && out.zone != nil {
		t = t.In(out.zone)
	}
	out.kind, out.value = kind, t
	return out, nil
}
func rescheduleDue(task api.Task, date, instant, local string) (map[string]any, error) {
	evidence, err := taskDueFacts(task, true)
	if err != nil {
		return nil, err
	}
	recurring, ok := task.ResponseFact("due.is_recurring").Bool()
	if !ok {
		return nil, errors.New("recurrence not returned; cannot promise preservation")
	}
	due := map[string]any{"is_recurring": recurring, "timezone": nil}
	if evidence.kind == "fixed" {
		zone, _ := evidence.timezone.Text()
		due["timezone"] = zone
	}
	if language, ok := task.ResponseFact("due.lang").Text(); ok && strings.TrimSpace(language) != "" {
		due["lang"] = language
	}
	if recurring {
		expression, ok := task.ResponseFact("due.string").Text()
		if !ok || strings.TrimSpace(expression) == "" || due["lang"] == nil {
			return nil, errors.New("recurring expression or language unavailable")
		}
		due["string"] = expression
	}
	target := evidence.value
	switch {
	case date != "":
		day, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, err
		}
		wall := time.Date(day.Year(), day.Month(), day.Day(), target.Hour(), target.Minute(), target.Second(), target.Nanosecond(), time.UTC)
		if evidence.kind == "date" {
			target = day
		} else if evidence.kind == "floating" {
			target = wall
		} else {
			target, err = uniqueWallInstant(wall, evidence.zone)
			if err != nil {
				return nil, err
			}
		}
	case instant != "":
		if evidence.kind != "fixed" {
			return nil, errors.New("RFC3339 target requires an existing fixed-zone due time")
		}
		target, err = parseTaskInstant(instant)
		if err != nil {
			return nil, err
		}
	case local != "":
		if evidence.kind != "floating" {
			return nil, errors.New("local datetime target requires an existing floating due time")
		}
		if !taskLocalPattern.MatchString(local) {
			return nil, errors.New("invalid floating datetime or unsupported precision")
		}
		target, err = time.Parse("2006-01-02T15:04:05", local)
		if err != nil {
			return nil, err
		}
	}
	switch evidence.kind {
	case "date":
		due["date"] = target.Format("2006-01-02")
	case "floating":
		due["date"] = target.Format("2006-01-02T15:04:05.999999999")
	case "fixed":
		due["date"] = target.UTC().Format(time.RFC3339Nano)
	}
	return due, nil
}

// Enumerate actual zone offsets around the target, then validate each candidate.
// This detects folds and gaps instead of accepting time.Date's normalization.
func uniqueWallInstant(wall time.Time, zone *time.Location) (time.Time, error) {
	offsets := map[int]bool{}
	for h := -72; h <= 72; h++ {
		_, offset := wall.Add(time.Duration(h) * time.Hour).In(zone).Zone()
		offsets[offset] = true
	}
	var matches []time.Time
	for offset := range offsets {
		instant := wall.Add(-time.Duration(offset) * time.Second)
		if instant.In(zone).Format("2006-01-02T15:04:05.999999999") == wall.Format("2006-01-02T15:04:05.999999999") {
			matches = append(matches, instant)
		}
	}
	if len(matches) != 1 {
		return time.Time{}, errors.New("target local clock is in a DST gap or fold; use an explicit RFC3339 instant")
	}
	return matches[0], nil
}
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
			if field.layout == "2006-01-02T15:04:05" && !taskLocalPattern.MatchString(field.value) {
				return &CodeError{Code: exitUsage, Err: errors.New("invalid floating datetime or unsupported precision")}
			}
			var err error
			if field.layout == time.RFC3339Nano {
				_, err = parseTaskInstant(field.value)
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
	due, err := rescheduleDue(*task, date, instant, local)
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
