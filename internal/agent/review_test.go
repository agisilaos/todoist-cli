package agent

import (
	"encoding/json"
	"testing"
)

func TestReviewRequiresCompleteActionAccounting(t *testing.T) {
	snapshot := map[string]json.RawMessage{"id": json.RawMessage(`"1"`), "content": json.RawMessage(`"task"`)}
	for _, test := range []struct {
		name, disposition, action string
		indices                   []int
		version                   int
	}{
		{"unaccounted", "change", "task_update", nil, 1},
		{"duplicates", "change", "task_update", []int{0, 0}, 1},
		{"wrong-kind", "change", "task_delete", []int{0}, 1},
		{"keep-mutates", "keep", "task_update", []int{0}, 1},
		{"complete-update", "complete", "task_update", []int{0}, 1},
		{"unsupported", "change", "task_update", []int{0}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := Plan{ConfirmToken: "x", Version: 1, Actions: []Action{{Type: test.action, TaskID: "1"}}, Review: &Review{Version: test.version, Tasks: []ReviewTask{{ID: "1", Snapshot: snapshot, Disposition: test.disposition, Actions: test.indices}}}}
			if err := ValidatePlan(plan, 1, false); err == nil {
				t.Fatal("accepted invalid review")
			}
		})
	}
}
