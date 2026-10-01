package cli

import (
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func detailFact(task api.Task, path, nullValue string) string {
	fact := task.ResponseFact(path)
	switch fact.State {
	case api.ResponseAbsent:
		return "Not returned"
	case api.ResponseNull:
		return nullValue
	case api.ResponseInvalid:
		return "Unavailable (invalid returned type)"
	}
	if text, ok := fact.Text(); ok {
		if text == "" {
			return "(empty returned value)"
		}
		return text
	}
	if value, ok := fact.Bool(); ok {
		if value {
			return "Yes"
		}
		return "No"
	}
	return string(fact.JSON())
}

func writeDetailTaskFacts(task api.Task, full bool, field func(string, string)) {
	unknownNull := "Unavailable (returned null)"
	if full {
		for _, entry := range []struct{ label, path, nullValue string }{
			{"Sibling order", "child_order", unknownNull},
			{"Order key", "order_key", "None (not migrated)"},
			{"Day order", "day_order", unknownNull},
			{"Owner ID", "user_id", unknownNull},
			{"Creator ID", "added_by_uid", "Unknown"},
			{"Assigner ID", "assigned_by_uid", "None"},
			{"Completer ID", "completed_by_uid", "None"},
			{"Collapsed", "is_collapsed", unknownNull},
			{"Deleted", "is_deleted", unknownNull},
			{"Completion count", "completed_count", unknownNull},
			{"Postponement count", "postponed_count", unknownNull},
			{"Uncompletable (returned)", "is_uncompletable", unknownNull},
		} {
			field(entry.label, detailFact(task, entry.path, entry.nullValue))
		}
		return
	}
	deadline := detailFact(task, "deadline", "No deadline")
	if task.ResponseFact("deadline").State == api.ResponseValue {
		deadline = detailFact(task, "deadline.date", unknownNull)
		language := task.ResponseFact("deadline.lang")
		if language.State != api.ResponseAbsent {
			field("Deadline language", detailFact(task, "deadline.lang", unknownNull))
		}
	}
	field("Deadline", deadline)
	duration := detailFact(task, "duration", "No duration")
	if task.ResponseFact("duration").State == api.ResponseValue {
		amount := detailFact(task, "duration.amount", unknownNull)
		unit := detailFact(task, "duration.unit", unknownNull)
		duration = "Amount: " + amount + "; unit: " + unit
		if number, ok := task.ResponseFact("duration.amount").Int(); ok {
			if label, known := task.ResponseFact("duration.unit").Text(); known && (label == "minute" || label == "day") {
				if number != 1 {
					label += "s"
				}
				duration = strconv.Itoa(number) + " " + label
			}
		}
	}
	field("Duration", duration)
	field("Assignee ID", detailFact(task, "responsible_uid", "Unassigned"))
	language := detailFact(task, "due.lang", unknownNull)
	if task.ResponseFact("due").State == api.ResponseNull {
		language = "None (no due date)"
	}
	field("Due language", language)
	reference := "Not returned (content unavailable)"
	if content, ok := task.ResponseFact("content").Text(); ok {
		reference = "No (title syntax)"
		if strings.HasPrefix(content, "* ") {
			reference = "Yes (title syntax)"
		}
	}
	field("Reference item", reference)
}
