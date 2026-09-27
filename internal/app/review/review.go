// Package review holds frozen task snapshots and review ordering rules.
package review

import (
	"bytes"
	"encoding/json"
	"sort"
	"time"
)

type Snapshot map[string]json.RawMessage

// SnapshotOf excludes server presentation fields unrelated to the reviewed task.
func SnapshotOf(task Snapshot) Snapshot {
	result := Snapshot{}
	for _, key := range []string{"id", "content", "description", "project_id", "section_id", "parent_id", "labels", "priority", "due", "duration", "deadline", "assignee_id", "responsible_uid", "checked", "is_completed", "updated_at"} {
		if value, ok := task[key]; ok {
			result[key] = value
		}
	}
	return result
}

func Text(task Snapshot, key string) string {
	var result string
	_ = json.Unmarshal(task[key], &result)
	return result
}

func Equal(a, b Snapshot) bool {
	// Decode before encoding to ignore insignificant whitespace and object-key order.
	var x, y any
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	_ = json.Unmarshal(aa, &x)
	_ = json.Unmarshal(bb, &y)
	aa, _ = json.Marshal(x)
	bb, _ = json.Marshal(y)
	return bytes.Equal(aa, bb)
}

func Sort(tasks []Snapshot) {
	sort.SliceStable(tasks, func(i, j int) bool {
		di, dj := due(tasks[i]), due(tasks[j])
		if di != dj {
			return di < dj
		}
		var pi, pj int
		_ = json.Unmarshal(tasks[i]["priority"], &pi)
		_ = json.Unmarshal(tasks[j]["priority"], &pj)
		if pi != pj {
			return pi > pj
		}
		return Text(tasks[i], "id") < Text(tasks[j], "id")
	})
}

func due(task Snapshot) int64 {
	var value map[string]string
	_ = json.Unmarshal(task["due"], &value)
	raw := value["datetime"]
	if raw == "" {
		raw = value["date"]
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return parsed.Unix()
		}
	}
	return 1<<63 - 1
}
