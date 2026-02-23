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
	if !strings.Contains(out.String(), "Activate now: source") {
		t.Fatalf("expected activation hint, got %q", out.String())
	}
}

func TestCompletionUninstallRemovesPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todoist.fish")
	if err := os.WriteFile(path, []byte("script"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &out,
		Mode:   output.ModeHuman,
	}
	if err := completionCommand(ctx, []string{"uninstall", "--path", path}); err != nil {
		t.Fatalf("completion uninstall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}
	if !strings.Contains(out.String(), "Removed completion script") {
		t.Fatalf("expected removal output, got %q", out.String())
	}
}

func TestCompletionUninstallNoopWhenNothingFound(t *testing.T) {
	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &out,
		Mode:   output.ModeHuman,
	}
	if err := completionCommand(ctx, []string{"uninstall", "--path", filepath.Join(t.TempDir(), "missing")}); err != nil {
		t.Fatalf("completion uninstall: %v", err)
	}
	if !strings.Contains(out.String(), "No completion scripts found to remove.") {
		t.Fatalf("unexpected output: %q", out.String())
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

func TestCompletionScriptsIncludeUpcomingCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "upcoming") {
			t.Fatalf("%s completion missing upcoming command", shell)
		}
	}
}

func TestCompletionScriptsIncludeCompletedCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "completed") {
			t.Fatalf("%s completion missing completed command", shell)
		}
	}
}

func TestCompletionScriptsIncludeReminderCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "reminder") {
			t.Fatalf("%s completion missing reminder command", shell)
		}
	}
}
