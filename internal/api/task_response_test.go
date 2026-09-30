package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskResponseFactsPreserveMachineSerialization(t *testing.T) {
	var task Task
	if err := json.Unmarshal([]byte(`{"id":"123","due":{"date":"2026-09-28T09:00:00Z","string":"every day","timezone":"Europe/Berlin","is_recurring":true}}`), &task); err != nil {
		t.Fatal(err)
	}
	if !task.DueReturned || task.Due.Timezone == nil || *task.Due.Timezone != "Europe/Berlin" || task.Due.IsRecurring == nil || !*task.Due.IsRecurring {
		t.Fatalf("lost response facts: %+v", task)
	}
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"123","content":"","description":"","project_id":"","section_id":"","parent_id":"","labels":null,"priority":0,"checked":false,"due":{"date":"2026-09-28T09:00:00Z","string":"every day"},"added_at":"","completed_at":"","updated_at":"","note_count":0}`
	if string(encoded) != want {
		t.Fatalf("machine contract changed: %s", encoded)
	}
	for _, tc := range []struct {
		input   string
		present bool
	}{
		{`{"due":null}`, true}, {`{}`, false},
	} {
		if err := json.Unmarshal([]byte(tc.input), &task); err != nil {
			t.Fatal(err)
		}
		if task.DueReturned != tc.present || task.Due != nil {
			t.Fatalf("stale or missing facts for %s: %+v", tc.input, task)
		}
	}
}

func TestTaskResponseOptionalFactsCannotFailCreation(t *testing.T) {
	for _, fields := range []string{`"timezone":42,"is_recurring":"yes"`, `"timezone":null,"is_recurring":null`, `"other":true`} {
		var task Task
		if err := json.Unmarshal([]byte(`{"due":{"date":"2026-09-28",`+fields+`}}`), &task); err != nil {
			t.Fatal(err)
		}
		if task.Due.Timezone != nil || task.Due.IsRecurring != nil {
			t.Fatalf("invented optional fact: %+v", task.Due)
		}
	}
	for _, input := range []string{`{"priority":"high"}`, `{"due":{"date":42}}`, `{"due":false}`} {
		var task Task
		if err := json.Unmarshal([]byte(input), &task); err == nil {
			t.Fatalf("accepted invalid existing field: %s", input)
		}
	}
	var task Task
	if err := json.Unmarshal([]byte(`{"due":{"is_recurring":false}}`), &task); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(task)
	if task.Due.IsRecurring == nil || *task.Due.IsRecurring || !strings.Contains(string(encoded), `"due":{}`) {
		t.Fatalf("explicit false lost: %s", encoded)
	}
}

func TestTaskDetailReturnedFieldsDoNotChangeSerialization(t *testing.T) {
	var task Task
	input := `{"description":"","section_id":null,"parent_id":"","checked":false,"completed_at":null,"note_count":0}`
	if err := json.Unmarshal([]byte(input), &task); err != nil {
		t.Fatal(err)
	}
	if task.Returned != (TaskReturnedFields{Description: true, SectionID: true, ParentID: true, Checked: true, CompletedAt: true, NoteCount: true}) {
		t.Fatalf("explicit empty/false/zero facts lost: %+v", task.Returned)
	}
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"","content":"","description":"","project_id":"","section_id":"","parent_id":"","labels":null,"priority":0,"checked":false,"due":null,"added_at":"","completed_at":"","updated_at":"","note_count":0}`
	if string(encoded) != want {
		t.Fatalf("machine serialization changed: %s", encoded)
	}
	for _, input := range []string{`{}`, `{"description":null,"checked":null,"note_count":null}`} {
		if err := json.Unmarshal([]byte(input), &task); err != nil {
			t.Fatal(err)
		}
		if task.Returned != (TaskReturnedFields{}) {
			t.Fatalf("unknown/stale facts for %s: %+v", input, task.Returned)
		}
	}
}
