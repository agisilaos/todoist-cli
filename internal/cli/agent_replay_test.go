package cli

import "testing"

func TestMakeReplayKeyDeterministic(t *testing.T) {
	action := Action{Type: "task_add", Content: "A"}
	k1 := makeReplayKey("abcd", 0, action)
	k2 := makeReplayKey("abcd", 0, action)
	if k1 == "" || k1 != k2 {
		t.Fatalf("expected deterministic non-empty replay key, got %q and %q", k1, k2)
	}
	k3 := makeReplayKey("abcd", 1, action)
	if k3 == k1 {
		t.Fatalf("expected different key for different index")
	}
}
