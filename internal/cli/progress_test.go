package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgressSinkStdErrMode(t *testing.T) {
	var stderr bytes.Buffer
	sink, err := newProgressSink("-", &stderr)
	if err != nil {
		t.Fatalf("newProgressSink: %v", err)
	}
	ctx := &Context{Progress: sink}
	emitProgress(ctx, "test_event", map[string]any{"k": "v"})
	got := stderr.String()
	if !strings.Contains(got, `"type":"test_event"`) || !strings.Contains(got, `"k":"v"`) {
		t.Fatalf("unexpected progress output: %q", got)
	}
}

func TestProgressSinkFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.jsonl")
	sink, err := newProgressSink(path, os.Stderr)
	if err != nil {
		t.Fatalf("newProgressSink: %v", err)
	}
	ctx := &Context{Progress: sink}
	emitProgress(ctx, "test_event_file", map[string]any{"ok": true})
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read progress file: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"type":"test_event_file"`) {
		t.Fatalf("unexpected progress file output: %q", got)
	}
}
