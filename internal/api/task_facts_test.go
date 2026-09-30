package api

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func responseFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/tasks/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTaskFaithfulResponseFixtures(t *testing.T) {
	for _, name := range []string{"populated", "absent", "null", "false-zero-empty"} {
		t.Run(name, func(t *testing.T) {
			data := responseFixture(t, name)
			var task Task
			if err := json.Unmarshal(data, &task); err != nil {
				t.Fatal(err)
			}
			var expected map[string]any
			decoder := json.NewDecoder(strings.NewReader(string(data)))
			decoder.UseNumber()
			if err := decoder.Decode(&expected); err != nil {
				t.Fatal(err)
			}
			delete(expected, "future_field")
			if due, ok := expected["due"].(map[string]any); ok {
				delete(due, "future_due_field")
			}
			if content, ok := expected["content"].(string); ok {
				expected["reference_item"] = map[string]any{"is_reference": strings.HasPrefix(content, "* "), "source": "content_prefix"}
			}
			if got := task.FaithfulResource(); !reflect.DeepEqual(got, expected) {
				t.Fatalf("returned values changed:\ngot %#v\nwant %#v", got, expected)
			}
			if len(task.ResponseIssues()) != 0 {
				t.Fatalf("well-typed response diagnosed: %+v", task.ResponseIssues())
			}
		})
	}
}

func TestTaskResponsePresenceAndSnapshotOwnership(t *testing.T) {
	var task Task
	if err := json.Unmarshal(responseFixture(t, "populated"), &task); err != nil {
		t.Fatal(err)
	}
	if value, ok := task.ResponseFact("child_order").Int(); !ok || value != 0 {
		t.Fatal("zero order lost")
	}
	if value, ok := task.ResponseFact("is_uncompletable").Bool(); !ok || value {
		t.Fatal("false flag lost or replaced by title inference")
	}
	if value, ok := task.Due.ResponseFact("lang").Text(); !ok || value != "en" || task.Due.Lang == nil || *task.Due.Lang != "en" {
		t.Fatal("due language lost")
	}
	if task.Due.ResponseFact("datetime").State != ResponseNull {
		t.Fatal("nested null lost")
	}
	copyOfTask := task
	resource := task.FaithfulResource()
	resource["due"].(map[string]any)["lang"] = "changed"
	resource["id"] = "changed"
	raw := task.ResponseFact("content").JSON()
	raw[0] = 'x'
	task.Content = "changed typed value"
	if value, _ := copyOfTask.ResponseFact("content").Text(); value != "* Prepare launch checklist" {
		t.Fatal("caller changed immutable fact")
	}
	if got := task.FaithfulResource()["id"]; got != "task-fidelity-fixture" {
		t.Fatal("caller changed snapshot")
	}
	if err := json.Unmarshal(responseFixture(t, "null"), &task); err != nil {
		t.Fatal(err)
	}
	if task.ResponseFact("child_order").State != ResponseNull || task.ResponseFact("due.lang").State != ResponseAbsent {
		t.Fatal("stale nested facts or null lost")
	}
	if err := json.Unmarshal(responseFixture(t, "absent"), &task); err != nil {
		t.Fatal(err)
	}
	if task.ResponseFact("child_order").State != ResponseAbsent || task.ResponseFact("due").State != ResponseAbsent {
		t.Fatal("reused target retained old facts")
	}
	if got := (Task{Content: "* invented", Checked: false}).FaithfulResource(); len(got) != 0 {
		t.Fatalf("Go defaults established response facts: %#v", got)
	}
}

func TestTaskMalformedOptionalFactsRetainValidSiblings(t *testing.T) {
	var task Task
	if err := json.Unmarshal(responseFixture(t, "malformed"), &task); err != nil {
		t.Fatalf("advisory facts turned response into failure: %v", err)
	}
	for _, path := range []string{"due.lang", "due.timezone", "due.is_recurring", "deadline.lang", "duration.amount", "responsible_uid", "child_order", "is_uncompletable"} {
		if task.ResponseFact(path).State != ResponseInvalid {
			t.Errorf("malformed %s established a value", path)
		}
	}
	resource := task.FaithfulResource()
	if resource["deadline"].(map[string]any)["date"] != "2026-10-10" || resource["duration"].(map[string]any)["unit"] != "minute" {
		t.Fatal("malformed member discarded valid siblings")
	}
	if _, present := resource["responsible_uid"]; present || len(task.ResponseIssues()) != 8 {
		t.Fatalf("invalid facts not omitted/diagnosed: %#v %+v", resource, task.ResponseIssues())
	}
	issues := task.ResponseIssues()
	issues[0].Path = "mutated"
	if task.ResponseIssues()[0].Path == "mutated" {
		t.Fatal("caller changed diagnostics")
	}
	for _, data := range []string{`{"priority":"high"}`, `{"due":{"date":42}}`, `{"due":false}`} {
		if err := json.Unmarshal([]byte(data), &task); err == nil {
			t.Fatalf("existing strict decoder weakened: %s", data)
		}
	}
}
