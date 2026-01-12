package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestCompletionInstallWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todoist.fish")

	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &out,
		Mode:   output.ModeHuman,
	}

	if err := completionCommand(ctx, []string{"install", "--path", path, "fish"}); err != nil {
		t.Fatalf("completion install: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed completion: %v", err)
	}
	if !bytes.Contains(data, []byte("todoist completion")) {
		t.Fatalf("completion script missing marker: %q", string(data))
	}
	if !strings.Contains(out.String(), "Installed fish completion") {
		t.Fatalf("expected install message, got %q", out.String())
	}
}
