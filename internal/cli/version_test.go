package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	Version = "v1.2.3"
	Commit = "abc123"
	Date = "2024-01-02T03:04:05Z"

	var out bytes.Buffer
	code := Execute([]string{"--version"}, &out, io.Discard)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	got := strings.TrimSpace(out.String())
	want := "todoist v1.2.3 (abc123) 2024-01-02T03:04:05Z"
	if got != want {
		t.Fatalf("unexpected version output: %q (want %q)", got, want)
	}
}
