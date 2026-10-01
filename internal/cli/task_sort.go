package cli

import (
	"errors"
	"flag"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type taskSortOptions struct {
	key, direction string
	flags          *flag.FlagSet
}

func (s *taskSortOptions) bind(fs *flag.FlagSet) {
	s.flags = fs
	fs.StringVar(&s.key, "sort", "", "due|deadline|priority|added|updated|completed|content|order|none")
	fs.StringVar(&s.direction, "sort-order", "", "asc|desc")
}
func (s taskSortOptions) validate() error {
	fail := func(text string) error { return &CodeError{Code: exitUsage, Err: errors.New(text)} }
	if s.flags != nil {
		var empty string
		s.flags.Visit(func(f *flag.Flag) {
			if (f.Name == "sort" || f.Name == "sort-order") && f.Value.String() == "" {
				empty = f.Name
			}
		})
		if empty != "" {
			return fail("--" + empty + " cannot be empty")
		}
	}
	if s.direction != "" && s.direction != "asc" && s.direction != "desc" {
		return fail("--sort-order must be asc or desc")
	}
	if s.key == "" {
		if s.direction != "" {
			return fail("--sort-order requires --sort")
		}
		return nil
	}
	switch s.key {
	case "due", "deadline", "priority", "added", "updated", "completed", "content", "order":
	case "none":
		if s.direction != "" {
			return fail("--sort none cannot have --sort-order")
		}
	default:
		return fail("unknown task sort key")
	}
	return nil
}

type taskSortKey struct {
	available bool
	text      string
	instant   time.Time
	number    *big.Rat
}

func explicitTaskSortKey(task api.Task, key string) taskSortKey {
	result := taskSortKey{}
	switch key {
	case "due":
		evidence, err := taskDueFacts(task, false)
		if err != nil {
			return result
		}
		result.text = evidence.value.Format("2006-01-02")
		if evidence.kind != "date" {
			result.text += "T" + evidence.value.Format("15:04:05.999999999")
		}
		result.available = true
	case "deadline":
		value, ok := task.ResponseFact("deadline.date").Text()
		if !ok {
			return result
		}
		t, err := time.Parse("2006-01-02", value)
		if err == nil {
			result.available = true
			result.text = t.Format("2006-01-02")
		}
	case "added", "updated", "completed":
		value, ok := task.ResponseFact(key + "_at").Text()
		if !ok {
			return result
		}
		t, err := parseTaskInstant(value)
		if err == nil {
			result.available = true
			result.instant = t
		}
	case "content":
		result.text, result.available = task.ResponseFact("content").Text()
	case "priority", "order":
		field := key
		if key == "order" {
			field = "child_order"
		}
		fact := task.ResponseFact(field)
		if fact.State != api.ResponseValue {
			return result
		}
		result.number = new(big.Rat)
		_, result.available = result.number.SetString(string(fact.JSON()))
	}
	return result
}
func (s taskSortOptions) apply(tasks []api.Task) error {
	if s.key == "" || s.key == "none" {
		return nil
	}

	for _, task := range tasks {
		if !usableTaskID(task.ID) {
			return errors.New("sorting requires returned task IDs")
		}
	}
	direction := s.direction
	if direction == "" {
		direction = "asc"
		if s.key == "priority" || s.key == "added" || s.key == "updated" || s.key == "completed" {
			direction = "desc"
		}
	}
	// Compute once per position so duplicate IDs with different returned facts keep
	// their own keys; stable sorting retains truly identical duplicate records.
	type entry struct {
		task api.Task
		key  taskSortKey
	}
	entries := make([]entry, len(tasks))
	for i, task := range tasks {
		entries[i] = entry{task, explicitTaskSortKey(task, s.key)}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].key, entries[j].key
		if a.available != b.available {
			return a.available
		}
		comparison := 0
		if a.available {
			if a.number != nil {
				comparison = a.number.Cmp(b.number)
			} else if !a.instant.IsZero() || !b.instant.IsZero() {
				if a.instant.Before(b.instant) {
					comparison = -1
				} else if a.instant.After(b.instant) {
					comparison = 1
				}
			} else {
				comparison = strings.Compare(a.text, b.text)
			}
		}
		if comparison == 0 {
			return entries[i].task.ID < entries[j].task.ID
		}
		if direction == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
	for i, entry := range entries {
		tasks[i] = entry.task
	}
	return nil
}
func firstTaskSort(options []taskSortOptions) taskSortOptions {
	if len(options) > 0 {
		return options[0]
	}
	return taskSortOptions{}
}
