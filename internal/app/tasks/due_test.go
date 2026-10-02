package tasks

import (
	"encoding/json"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func TestRescheduleTimeCharacterAndEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, due, date, instant, local, want string
		fail                                  bool
	}{
		{"recurring date", `{"date":"2026-10-01","timezone":null,"is_recurring":true,"string":"every day","lang":"en"}`, "2026-10-03", "", "", "2026-10-03", false},
		{"nonrecurring stale expression", `{"date":"2026-10-01","timezone":null,"is_recurring":false,"string":"stale"}`, "2026-10-03", "", "", "2026-10-03", false},
		{"floating retains clock", `{"date":"2026-10-01T09:15:30","timezone":null,"is_recurring":false}`, "2026-10-03", "", "", "2026-10-03T09:15:30", false},
		{"floating local target", `{"date":"2026-10-01T09:15:30","timezone":null,"is_recurring":false}`, "", "", "2026-10-03T10:00:00", "2026-10-03T10:00:00", false},
		{"fixed retains clock", `{"date":"2026-10-01T07:15:30Z","timezone":"Europe/Berlin","is_recurring":false}`, "2026-11-03", "", "", "2026-11-03T08:15:30Z", false},
		{"fixed fold explicit", `{"date":"2026-10-01T00:30:00Z","timezone":"Europe/Berlin","is_recurring":false}`, "", "2026-10-25T02:30:00+01:00", "", "2026-10-25T01:30:00Z", false},
		{"compatibility fixed", `{"date":"2026-10-01","datetime":"2026-10-01T07:15:30Z","timezone":"Europe/Berlin","is_recurring":false}`, "2026-11-03", "", "", "2026-11-03T08:15:30Z", false},
		{"unknown recurrence", `{"date":"2026-10-01","timezone":null}`, "2026-10-03", "", "", "", true},
		{"null recurrence", `{"date":"2026-10-01","timezone":null,"is_recurring":null}`, "2026-10-03", "", "", "", true},
		{"missing timezone", `{"date":"2026-10-01","is_recurring":false}`, "2026-10-03", "", "", "", true},
		{"fixed offset alone", `{"date":"2026-10-01T07:15:30Z","timezone":null,"is_recurring":false}`, "2026-10-03", "", "", "", true},
		{"blank expression", `{"date":"2026-10-01","timezone":null,"is_recurring":true,"string":" ","lang":"en"}`, "2026-10-03", "", "", "", true},
		{"missing language", `{"date":"2026-10-01","timezone":null,"is_recurring":true,"string":"daily"}`, "2026-10-03", "", "", "", true},
		{"contradictory dates", `{"date":"2026-10-01","datetime":"2026-10-02T12:00:00Z","timezone":"Europe/Berlin","is_recurring":false}`, "2026-10-03", "", "", "", true},
		{"date conversion", `{"date":"2026-10-01","timezone":null,"is_recurring":false}`, "", "2026-10-03T12:00:00Z", "", "", true},
		{"floating conversion", `{"date":"2026-10-01T12:00:00","timezone":null,"is_recurring":false}`, "", "2026-10-03T12:00:00Z", "", "", true},
		{"fixed conversion", `{"date":"2026-10-01T12:00:00Z","timezone":"Europe/Berlin","is_recurring":false}`, "", "", "2026-10-03T12:00:00", "", true},
		{"Berlin gap", `{"date":"2026-01-01T01:30:00Z","timezone":"Europe/Berlin","is_recurring":false}`, "2026-03-29", "", "", "", true},
		{"Berlin fold", `{"date":"2026-01-01T01:30:00Z","timezone":"Europe/Berlin","is_recurring":false}`, "2026-10-25", "", "", "", true},
		{"Lord Howe gap", `{"date":"2026-07-01T02:15:00+10:30","timezone":"Australia/Lord_Howe","is_recurring":false}`, "2026-10-04", "", "", "", true},
		{"Apia skipped date", `{"date":"2011-12-01T12:00:00-10:00","timezone":"Pacific/Apia","is_recurring":false}`, "2011-12-30", "", "", "", true},
		{"malformed offset", `{"date":"2026-10-01T12:00:00+00:60","timezone":"Europe/Berlin","is_recurring":false}`, "2026-10-03", "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var task api.Task
			if err := json.Unmarshal([]byte(`{"id":"t","due":`+tc.due+`}`), &task); err != nil {
				t.Fatal(err)
			}
			due, err := RescheduleDue(task, tc.date, tc.instant, tc.local)
			if (err != nil) != tc.fail {
				t.Fatal(err, due)
			}
			if tc.fail {
				return
			}
			if due["date"] != tc.want {
				t.Fatal(due)
			}
			recurring, _ := task.ResponseFact("due.is_recurring").Bool()
			if !recurring {
				if _, ok := due["string"]; ok {
					t.Fatal("stale recurrence retained", due)
				}
			}
		})
	}
}
