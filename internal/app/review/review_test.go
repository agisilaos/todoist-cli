package review

import (
	"encoding/json"
	"testing"
)

func TestOrderDueInstantsPriorityAndIDs(t *testing.T) {
	snapshot := func(id, date string, priority int) Snapshot {
		raw, _ := json.Marshal(map[string]any{"id": id, "priority": priority, "due": map[string]any{"date": date, "is_recurring": true}})
		var task Snapshot
		_ = json.Unmarshal(raw, &task)
		return task
	}
	tasks := []Snapshot{snapshot("undated", "", 4), snapshot("later", "2026-10-01T09:00:00Z", 4), snapshot("early-low", "2026-10-01T10:00:00+02:00", 1), snapshot("early-b", "2026-10-01T08:00:00Z", 4), snapshot("early-a", "2026-10-01T08:00:00Z", 4)}
	Sort(tasks)
	for i, want := range []string{"early-a", "early-b", "early-low", "later", "undated"} {
		if got := Text(tasks[i], "id"); got != want {
			t.Errorf("%d=%s want %s", i, got, want)
		}
	}
}
