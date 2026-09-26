package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestCompletionPowerShellAndPwshSelectIdenticalScript(t *testing.T) {
	powerShell, err := completionScript("powershell")
	if err != nil {
		t.Fatalf("completionScript(powershell): %v", err)
	}
	pwsh, err := completionScript("pwsh")
	if err != nil {
		t.Fatalf("completionScript(pwsh): %v", err)
	}
	if powerShell != pwsh {
		t.Fatal("powershell and pwsh selected different scripts")
	}
	if !strings.HasPrefix(powerShell, powerShellCompletionMarker+"\n") {
		t.Fatalf("PowerShell completion missing ownership marker: %q", powerShell[:min(len(powerShell), 80)])
	}
	if !strings.Contains(powerShell, "Register-ArgumentCompleter -Native -CommandName todoist") {
		t.Fatal("PowerShell completion does not register todoist")
	}

	var canonicalOut bytes.Buffer
	canonicalCtx := &Context{Stdout: &canonicalOut, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	if err := completionCommand(canonicalCtx, []string{"powershell"}); err != nil {
		t.Fatalf("completion powershell: %v", err)
	}
	var aliasOut bytes.Buffer
	aliasCtx := &Context{Stdout: &aliasOut, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	if err := completionCommand(aliasCtx, []string{"pwsh"}); err != nil {
		t.Fatalf("completion pwsh: %v", err)
	}
	if canonicalOut.String() != aliasOut.String() {
		t.Fatal("powershell and pwsh commands produced different output")
	}
}

func TestDefaultPowerShellCompletionPath(t *testing.T) {
	t.Run("xdg", func(t *testing.T) {
		xdg := filepath.Join(t.TempDir(), "data")
		t.Setenv("XDG_DATA_HOME", xdg)
		want := filepath.Join(xdg, "todoist", "completions", "todoist.ps1")
		for _, shell := range []string{"powershell", "pwsh"} {
			if got := defaultCompletionPath(shell); got != want {
				t.Fatalf("defaultCompletionPath(%s) = %q, want %q", shell, got, want)
			}
		}
	})

	t.Run("home fallback", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", home)
		want := filepath.Join(home, ".local", "share", "todoist", "completions", "todoist.ps1")
		if got := defaultCompletionPath("powershell"); got != want {
			t.Fatalf("defaultCompletionPath(powershell) = %q, want %q", got, want)
		}
	})
}

func TestDetectShellRecognizesPowerShellWithoutShell(t *testing.T) {
	tests := []struct {
		name         string
		shell        string
		modulePath   string
		distribution string
	}{
		{name: "module path", modulePath: "/opt/microsoft/powershell/Modules"},
		{name: "distribution channel", distribution: "PSGitHub"},
		{name: "pwsh basename", shell: "/usr/local/bin/pwsh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shell)
			t.Setenv("PSModulePath", tt.modulePath)
			t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", tt.distribution)
			if got := detectShell(); got != "powershell" {
				t.Fatalf("detectShell() = %q, want powershell", got)
			}
		})
	}
}

func TestCompletionInstallPowerShellHumanOutputAndProfileSafety(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	profile := filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}
	const profileContents = "# existing profile content\n"
	if err := os.WriteFile(profile, []byte(profileContents), 0o644); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	var out bytes.Buffer
	ctx := &Context{Stdout: &out, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	if err := completionCommand(ctx, []string{"install", "pwsh"}); err != nil {
		t.Fatalf("completion install pwsh: %v", err)
	}
	installedPath := defaultCompletionPath("powershell")
	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("installed completion: %v", err)
	}
	profileAfter, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if string(profileAfter) != profileContents {
		t.Fatalf("PowerShell profile changed: %q", string(profileAfter))
	}
	if !strings.Contains(out.String(), "Installed powershell completion") {
		t.Fatalf("install output did not use canonical shell: %q", out.String())
	}
	if !strings.Contains(out.String(), "Activate now: . '") || !strings.Contains(out.String(), "$PROFILE") {
		t.Fatalf("install output missing activation instructions: %q", out.String())
	}
}

func TestCompletionInstallPowerShellQuotesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user's files", "todoist.ps1")
	var out bytes.Buffer
	ctx := &Context{Stdout: &out, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	if err := completionCommand(ctx, []string{"install", "--path", path, "powershell"}); err != nil {
		t.Fatalf("completion install: %v", err)
	}
	wantCommand := ". '" + strings.ReplaceAll(path, "'", "''") + "'"
	if !strings.Contains(out.String(), wantCommand) {
		t.Fatalf("activation output %q does not contain %q", out.String(), wantCommand)
	}
}

func TestCompletionInstallPowerShellJSONIsCleanAndCanonical(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todoist.ps1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx := &Context{Stdout: &stdout, Stderr: &stderr, Mode: output.ModeJSON}
	if err := completionCommand(ctx, []string{"install", "--path", path, "pwsh"}); err != nil {
		t.Fatalf("completion install: %v", err)
	}
	var result struct {
		Shell      string `json:"shell"`
		Path       string `json:"path"`
		Activation string `json:"activation"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode install JSON %q: %v", stdout.String(), err)
	}
	if result.Shell != "powershell" || result.Path != path {
		t.Fatalf("unexpected install JSON: %+v", result)
	}
	if result.Activation != completionActivationHint("powershell", path) {
		t.Fatalf("activation = %q, want %q", result.Activation, completionActivationHint("powershell", path))
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestCompletionInstallPowerShellIsIdempotentAndUpdatesOwnedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todoist.ps1")
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	if err := completionCommand(ctx, []string{"install", "--path", path, "powershell"}); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	oldTime := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("set completion timestamp: %v", err)
	}
	if err := completionCommand(ctx, []string{"install", "--path", path, "pwsh"}); err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat completion: %v", err)
	}
	if !info.ModTime().Equal(oldTime) {
		t.Fatalf("identical reinstall rewrote file: modtime = %v, want %v", info.ModTime(), oldTime)
	}

	if err := os.WriteFile(path, []byte(powerShellCompletionMarker+"\n# stale\n"), 0o644); err != nil {
		t.Fatalf("seed stale completion: %v", err)
	}
	if err := completionCommand(ctx, []string{"install", "--path", path, "powershell"}); err != nil {
		t.Fatalf("upgrade owned completion: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read upgraded completion: %v", err)
	}
	if string(data) != powerShellCompletion {
		t.Fatal("owned completion was not upgraded")
	}
}

func TestCompletionInstallPowerShellRefusesUnrecognizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todoist.ps1")
	const contents = "# unrelated script\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	err := completionCommand(ctx, []string{"install", "--path", path, "powershell"})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("install error = %v, want overwrite refusal", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved file: %v", readErr)
	}
	if string(data) != contents {
		t.Fatalf("unrecognized file changed: %q", string(data))
	}
}

func TestCompletionUninstallPowerShellAliasesAndOwnership(t *testing.T) {
	for _, shell := range []string{"powershell", "pwsh"} {
		t.Run(shell, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todoist.ps1")
			if err := os.WriteFile(path, []byte(powerShellCompletion), 0o644); err != nil {
				t.Fatalf("seed completion: %v", err)
			}
			ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
			if err := completionCommand(ctx, []string{"uninstall", "--path", path, shell}); err != nil {
				t.Fatalf("completion uninstall %s: %v", shell, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("owned completion still exists: %v", err)
			}
		})
	}

	path := filepath.Join(t.TempDir(), "todoist.ps1")
	const contents = "# unrelated script\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("seed unrelated file: %v", err)
	}
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	err := completionCommand(ctx, []string{"uninstall", "--path", path, "powershell"})
	if err == nil || !strings.Contains(err.Error(), "refusing to remove") {
		t.Fatalf("uninstall error = %v, want ownership refusal", err)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != contents {
		t.Fatalf("unrelated file was not preserved: data=%q err=%v", string(data), readErr)
	}
}

func TestCompletionUninstallPowerShellJSONNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.ps1")
	var stdout bytes.Buffer
	ctx := &Context{Stdout: &stdout, Stderr: &bytes.Buffer{}, Mode: output.ModeJSON}
	if err := completionCommand(ctx, []string{"uninstall", "--path", path, "pwsh"}); err != nil {
		t.Fatalf("completion uninstall: %v", err)
	}
	var result struct {
		Removed []string `json:"removed"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode uninstall JSON %q: %v", stdout.String(), err)
	}
	if result.Removed == nil || len(result.Removed) != 0 {
		t.Fatalf("removed = %#v, want empty array", result.Removed)
	}
}

func TestCompletionUninstallTargetsIncludePowerShell(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	targets, err := completionUninstallTargets("", "")
	if err != nil {
		t.Fatalf("completionUninstallTargets: %v", err)
	}
	wantPath := filepath.Join(xdg, "todoist", "completions", "todoist.ps1")
	found := false
	for _, target := range targets {
		if target.shell == "powershell" && target.path == wantPath {
			found = true
		}
	}
	if !found {
		t.Fatalf("PowerShell target %q missing from %#v", wantPath, targets)
	}
}

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

func TestZshIDsOnlySupportedContexts(t *testing.T) {
	script, err := completionScript("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"inbox", "today", "upcoming", "completed", "task", "project", "filter", "workspace", "section", "label", "comment", "reminder", "notification", "activity"} {
		_, tail, found := strings.Cut(script, "\n  "+command+")\n")
		block, _, _ := strings.Cut(tail, ";;")
		if !found || !strings.Contains(block, "--ids-only") {
			t.Errorf("zsh %s context missing --ids-only", command)
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

func TestCompletionScriptsIncludeNotificationCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "notification") {
			t.Fatalf("%s completion missing notification command", shell)
		}
	}
}

func TestCompletionScriptsIncludeActivityCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "activity") {
			t.Fatalf("%s completion missing activity command", shell)
		}
	}
}

func TestCompletionScriptsIncludeStatsCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "stats") {
			t.Fatalf("%s completion missing stats command", shell)
		}
	}
}

func TestCompletionScriptsIncludeSettingsCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "settings") {
			t.Fatalf("%s completion missing settings command", shell)
		}
	}
}

func TestCompletionScriptsIncludeViewCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "view") {
			t.Fatalf("%s completion missing view command", shell)
		}
	}
}

func TestCompletionScriptsIncludeProjectCreateSubcommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%s): %v", shell, err)
		}
		if !strings.Contains(script, "create") {
			t.Fatalf("%s completion missing create subcommand", shell)
		}
	}
}
