package tasks

import (
	"errors"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type TimeCharacter string

const (
	DateOnly  TimeCharacter = "date"
	Floating  TimeCharacter = "floating"
	FixedZone TimeCharacter = "fixed"
)

type DueEvidence struct {
	Character TimeCharacter
	Value     time.Time
	zone      *time.Location
	timezone  api.ResponseFact
}

var taskLocalPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?$`)

var taskInstantPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

func ParseInstant(value string) (time.Time, error) {
	if !taskInstantPattern.MatchString(value) {
		return time.Time{}, errors.New("invalid RFC3339 instant or unsupported precision")
	}
	return time.Parse(time.RFC3339Nano, value)
}
func parseDueValue(value string) (TimeCharacter, time.Time, error) {
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return DateOnly, t, nil
	}
	if t, err := ParseInstant(value); err == nil {
		return FixedZone, t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", value); err == nil && taskLocalPattern.MatchString(value) {
		return Floating, t, nil
	}
	return "", time.Time{}, errors.New("invalid returned due date")
}
func DueFacts(task api.Task, preserve bool) (DueEvidence, error) {
	var out DueEvidence
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
		if err != nil || otherKind == DateOnly {
			return out, errors.New("invalid compatibility datetime")
		}
		localOther := other
		if otherKind == FixedZone && out.zone != nil {
			localOther = other.In(out.zone)
		}
		if kind == DateOnly {
			if localOther.Format("2006-01-02") != t.Format("2006-01-02") {
				return out, errors.New("contradictory due calendar date and datetime")
			}
			kind, t = otherKind, other
		} else if kind != otherKind || !t.Equal(other) {
			return out, errors.New("contradictory due date and datetime")
		}
	}
	if preserve {
		if kind == FixedZone && out.zone == nil || kind != FixedZone && out.timezone.State != api.ResponseNull {
			return out, errors.New("returned timezone cannot establish time character")
		}
	} else if kind != FixedZone && out.zone != nil {
		return out, errors.New("contradictory due timezone")
	}
	if kind == FixedZone && out.zone != nil {
		t = t.In(out.zone)
	}
	out.Character, out.Value = kind, t
	return out, nil
}
func RescheduleDue(task api.Task, date, instant, local string) (map[string]any, error) {
	evidence, err := DueFacts(task, true)
	if err != nil {
		return nil, err
	}
	recurring, ok := task.ResponseFact("due.is_recurring").Bool()
	if !ok {
		return nil, errors.New("recurrence not returned; cannot promise preservation")
	}
	due := map[string]any{"is_recurring": recurring, "timezone": nil}
	if evidence.Character == FixedZone {
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
	target := evidence.Value
	switch {
	case date != "":
		day, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, err
		}
		wall := time.Date(day.Year(), day.Month(), day.Day(), target.Hour(), target.Minute(), target.Second(), target.Nanosecond(), time.UTC)
		if evidence.Character == DateOnly {
			target = day
		} else if evidence.Character == Floating {
			target = wall
		} else {
			target, err = uniqueWallInstant(wall, evidence.zone)
			if err != nil {
				return nil, err
			}
		}
	case instant != "":
		if evidence.Character != FixedZone {
			return nil, errors.New("RFC3339 target requires an existing fixed-zone due time")
		}
		target, err = ParseInstant(instant)
		if err != nil {
			return nil, err
		}
	case local != "":
		if evidence.Character != Floating {
			return nil, errors.New("local datetime target requires an existing floating due time")
		}
		target, err = ParseLocalDatetime(local)
		if err != nil {
			return nil, err
		}
	}
	switch evidence.Character {
	case DateOnly:
		due["date"] = target.Format("2006-01-02")
	case Floating:
		due["date"] = target.Format("2006-01-02T15:04:05.999999999")
	case FixedZone:
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

func ParseLocalDatetime(value string) (time.Time, error) {
	if !taskLocalPattern.MatchString(value) {
		return time.Time{}, errors.New("invalid floating datetime or unsupported precision")
	}
	return time.Parse("2006-01-02T15:04:05", value)
}
