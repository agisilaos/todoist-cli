package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

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

func TestReplayStoreRecordAppliedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ctx := &Context{ConfigPath: filepath.Join(dir, "config.json")}
	store, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatalf("loadReplayStore: %v", err)
	}
	appliedAt := time.Date(2026, time.August, 25, 12, 34, 56, 789, time.FixedZone("test", 2*60*60))
	if err := store.RecordApplied("key-1", appliedAt); err != nil {
		t.Fatalf("RecordApplied: %v", err)
	}

	reloaded, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatalf("reload replay store: %v", err)
	}
	if !reloaded.Contains("key-1") {
		t.Fatal("expected reloaded store to contain recorded key")
	}
	if got := reloaded.journal.Applied["key-1"]; got != "2026-08-25T10:34:56Z" {
		t.Fatalf("unexpected persisted timestamp: %q", got)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(replayJournalPath(ctx)); err != nil {
			t.Fatalf("stat replay journal: %v", err)
		} else if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("expected replay journal mode 0600, got %04o", got)
		}
	}
}

func TestReplayStorePreservesExistingFormatAndEntries(t *testing.T) {
	dir := t.TempDir()
	ctx := &Context{ConfigPath: filepath.Join(dir, "config.json")}
	path := replayJournalPath(ctx)
	legacy := []byte(`{"applied":{"legacy-key":"2025-01-02T03:04:05Z"},"future_field":{"ignored":true}}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy journal: %v", err)
	}
	store, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatalf("load legacy journal: %v", err)
	}
	if !store.Contains("legacy-key") {
		t.Fatal("expected legacy replay key to load")
	}
	if err := store.RecordApplied("new-key", time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)); err != nil {
		t.Fatalf("record new key: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten journal: %v", err)
	}
	var got replayJournal
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse rewritten journal: %v", err)
	}
	want := map[string]string{
		"legacy-key": "2025-01-02T03:04:05Z",
		"new-key":    "2026-01-02T03:04:05Z",
	}
	if !maps.Equal(got.Applied, want) {
		t.Fatalf("unexpected applied entries: got %#v want %#v", got.Applied, want)
	}
}

func TestReplayStoreDuplicateRecordDoesNotWriteOrChangeTimestamp(t *testing.T) {
	writes := 0
	store := &fileReplayStore{
		path:    "unused",
		journal: replayJournal{Applied: map[string]string{"key-1": "2025-01-02T03:04:05Z"}},
		persist: func(string, replayJournal) error {
			writes++
			return nil
		},
	}
	if err := store.RecordApplied("key-1", time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)); err != nil {
		t.Fatalf("RecordApplied duplicate: %v", err)
	}
	if writes != 0 {
		t.Fatalf("expected no persistence for duplicate key, got %d writes", writes)
	}
	if got := store.journal.Applied["key-1"]; got != "2025-01-02T03:04:05Z" {
		t.Fatalf("duplicate changed original timestamp to %q", got)
	}
}

func TestReplayStoreWriteFailureDoesNotCommitMemoryOrDisk(t *testing.T) {
	dir := t.TempDir()
	ctx := &Context{ConfigPath: filepath.Join(dir, "config.json")}
	path := replayJournalPath(ctx)
	original := []byte(`{"applied":{"existing":"2025-01-02T03:04:05Z"}}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write original journal: %v", err)
	}
	store, err := loadReplayStore(ctx)
	if err != nil {
		t.Fatalf("loadReplayStore: %v", err)
	}
	wantErr := errors.New("injected write failure")
	store.persist = func(string, replayJournal) error { return wantErr }
	if err := store.RecordApplied("new-key", time.Now()); !errors.Is(err, wantErr) {
		t.Fatalf("RecordApplied error = %v, want %v", err, wantErr)
	}
	if store.Contains("new-key") {
		t.Fatal("failed record must not commit in-memory state")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original journal: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("failed record changed journal: got %q want %q", got, original)
	}
}

func TestReplayStoreRejectsCorruptedJournal(t *testing.T) {
	dir := t.TempDir()
	ctx := &Context{ConfigPath: filepath.Join(dir, "config.json")}
	if err := os.WriteFile(replayJournalPath(ctx), []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("write corrupted journal: %v", err)
	}
	if _, err := loadReplayStore(ctx); err == nil {
		t.Fatal("expected corrupted replay journal to fail loading")
	}
}

func TestReplayStoreRequiresDurablePath(t *testing.T) {
	if _, err := loadReplayStore(&Context{}); !errors.Is(err, errReplayPathUnavailable) {
		t.Fatalf("loadReplayStore error = %v, want %v", err, errReplayPathUnavailable)
	}
}

func BenchmarkReplayStoreRecordApplied(b *testing.B) {
	for _, entries := range []int{0, 1_000, 10_000} {
		b.Run(fmt.Sprintf("entries_%d", entries), func(b *testing.B) {
			baseline := emptyReplayJournal()
			for i := 0; i < entries; i++ {
				baseline.Applied[fmt.Sprintf("key-%d", i)] = "2026-01-02T03:04:05Z"
			}
			store := &fileReplayStore{
				path:    "benchmark",
				journal: baseline,
				persist: func(_ string, journal replayJournal) error {
					_, err := json.Marshal(journal)
					return err
				},
			}
			at := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				store.journal = baseline
				if err := store.RecordApplied("benchmark-new-key", at); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
