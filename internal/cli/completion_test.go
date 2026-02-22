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

func TestCompletionScriptsIncludeOAuthAuthLoginFlags(t *testing.T) {
	expectedFlags := map[string][]string{
		"bash": {
			"--oauth",
			"--oauth-device",
			"--no-browser",
			"--client-id",
			"--oauth-authorize-url",
			"--oauth-token-url",
			"--oauth-device-url",
			"--oauth-listen",
			"--oauth-redirect-uri",
		},
		"zsh": {
			"--oauth",
			"--oauth-device",
			"--no-browser",
			"--client-id",
			"--oauth-authorize-url",
			"--oauth-token-url",
			"--oauth-device-url",
			"--oauth-listen",
			"--oauth-redirect-uri",
		},
		"fish": {
			"-l oauth",
			"-l oauth-device",
			"-l no-browser",
			"-l client-id",
			"-l oauth-authorize-url",
			"-l oauth-token-url",
			"-l oauth-device-url",
			"-l oauth-listen",
			"-l oauth-redirect-uri",
		},
	}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		for _, flag := range expectedFlags[shell] {
			if !strings.Contains(script, flag) {
				t.Fatalf("%s completion missing %s", shell, flag)
			}
		}
	}
}

func TestCompletionScriptsIncludeFuzzyGlobalFlags(t *testing.T) {
	expectedFlags := map[string][]string{
		"bash": {"--fuzzy", "--no-fuzzy", "--progress-jsonl", "--accessible"},
		"zsh":  {"--fuzzy", "--no-fuzzy", "--progress-jsonl", "--accessible"},
		"fish": {"-l fuzzy", "-l no-fuzzy", "-l progress-jsonl", "-l accessible"},
	}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		for _, flag := range expectedFlags[shell] {
			if !strings.Contains(script, flag) {
				t.Fatalf("%s completion missing %s", shell, flag)
			}
		}
	}
}

func TestCompletionScriptsIncludeAgentPolicyFlag(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		flag := "--policy"
		if shell == "fish" {
			flag = "-l policy"
		}
		if !strings.Contains(script, flag) {
			t.Fatalf("%s completion missing %s", shell, flag)
		}
	}
}

func TestCompletionScriptsIncludeFilterCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "filter") {
			t.Fatalf("%s completion missing filter command", shell)
		}
	}
}
