package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/output"
)

const supportedCompletionShells = "bash, zsh, fish, powershell (alias: pwsh)"

func completionCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printCompletionHelp(ctx.Stdout)
		if len(args) == 0 {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("shell is required (supported: %s)", supportedCompletionShells)}
		}
		return nil
	}

	if args[0] == "install" {
		return completionInstall(ctx, args[1:])
	}
	if args[0] == "uninstall" {
		return completionUninstall(ctx, args[1:])
	}

	shell := canonicalCompletionShell(args[0])
	script, err := completionScript(shell)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: &unknownCommandError{
			parent: "completion", input: args[0], message: err.Error(),
		}}
	}
	fmt.Fprint(ctx.Stdout, script)
	return nil
}

func completionInstall(ctx *Context, args []string) error {
	fs := newFlagSet("completion install")
	var path string
	var help bool
	fs.StringVar(&path, "path", "", "Install path override")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCompletionHelp(ctx.Stdout)
		return nil
	}
	shell := ""
	if fs.NArg() > 0 {
		shell = canonicalCompletionShell(fs.Arg(0))
	}
	if shell == "" {
		shell = detectShell(ctx)
	}
	if shell == "" {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("shell is required (supported: %s)", supportedCompletionShells)}
	}
	script, err := completionScript(shell)
	if err != nil {
		return err
	}
	if path == "" {
		path = defaultCompletionPath(ctx, shell)
		if path == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s", shell)}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create completion dir: %w", err)
	}
	if err := writeCompletionFile(shell, path, script); err != nil {
		return err
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, map[string]any{
			"shell":      shell,
			"path":       path,
			"activation": completionActivationHint(shell, path),
		})
	}
	fmt.Fprintf(ctx.Stdout, "Installed %s completion to %s\n", shell, path)
	fmt.Fprintln(ctx.Stdout, completionActivationHint(shell, path))
	if shell == "powershell" {
		fmt.Fprintf(ctx.Stdout, "Enable for future shells: add %s to $PROFILE\n", powerShellSourceCommand(path))
	}
	return nil
}

func completionUninstall(ctx *Context, args []string) error {
	fs := newFlagSet("completion uninstall")
	var path string
	var help bool
	fs.StringVar(&path, "path", "", "Uninstall path override")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printCompletionHelp(ctx.Stdout)
		return nil
	}
	shell := ""
	if fs.NArg() > 0 {
		shell = canonicalCompletionShell(fs.Arg(0))
	}

	targets, err := completionUninstallTargets(ctx, shell, path)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
			return writeStructuredValue(ctx, map[string]any{
				"removed": []string{},
			})
		}
		fmt.Fprintln(ctx.Stdout, "No completion scripts found to remove.")
		return nil
	}

	removed := make([]string, 0, len(targets))
	for _, target := range targets {
		if _, statErr := os.Stat(target.path); statErr != nil {
			if !os.IsNotExist(statErr) {
				return fmt.Errorf("inspect completion %s: %w", target.path, statErr)
			}
			continue
		}
		if target.shell == "powershell" {
			owned, err := isOwnedPowerShellCompletion(target.path)
			if err != nil {
				return err
			}
			if !owned {
				return fmt.Errorf("refusing to remove unrecognized PowerShell completion file: %s", target.path)
			}
		}
		if err := os.Remove(target.path); err != nil {
			return fmt.Errorf("remove completion %s: %w", target.path, err)
		}
		removed = append(removed, target.path)
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, map[string]any{
			"removed": removed,
		})
	}
	if len(removed) == 0 {
		fmt.Fprintln(ctx.Stdout, "No completion scripts found to remove.")
		return nil
	}
	for _, p := range removed {
		fmt.Fprintf(ctx.Stdout, "Removed completion script: %s\n", p)
	}
	return nil
}

func completionScript(shell string) (string, error) {
	switch canonicalCompletionShell(shell) {
	case "bash":
		return bashCompletion, nil
	case "zsh":
		return zshCompletion, nil
	case "fish":
		return fishCompletion, nil
	case "powershell":
		return powerShellCompletion, nil
	default:
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s (supported: %s)", shell, supportedCompletionShells)}
	}
}

func canonicalCompletionShell(shell string) string {
	shell = strings.ToLower(strings.TrimSpace(shell))
	if shell == "pwsh" {
		return "powershell"
	}
	return shell
}

func defaultCompletionPath(ctx *Context, shell string) string {
	xdg := ctx.getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			xdg = filepath.Join(home, ".local", "share")
		}
	}
	switch canonicalCompletionShell(shell) {
	case "bash":
		if xdg != "" {
			return filepath.Join(xdg, "bash-completion", "completions", "todoist")
		}
	case "zsh":
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, ".zfunc", "_todoist")
		}
	case "fish":
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, ".config", "fish", "completions", "todoist.fish")
		}
	case "powershell":
		if xdg != "" {
			return filepath.Join(xdg, "todoist", "completions", "todoist.ps1")
		}
	}
	return ""
}

func detectShell(ctx *Context) string {
	if ctx.getenv("POWERSHELL_DISTRIBUTION_CHANNEL") != "" || ctx.getenv("PSModulePath") != "" {
		return "powershell"
	}
	shell := ctx.getenv("SHELL")
	if shell == "" {
		return ""
	}
	return canonicalCompletionShell(filepath.Base(shell))
}

func completionActivationHint(shell, path string) string {
	switch canonicalCompletionShell(shell) {
	case "bash":
		return fmt.Sprintf("Activate now: source %s", shellEscape(path))
	case "zsh":
		// Register a function so custom filenames work and the script only runs
		// inside zsh's completion context. Resolve relative paths before the
		// user changes directory and invokes completion.
		if absolutePath, err := filepath.Abs(path); err == nil {
			path = absolutePath
		}
		return fmt.Sprintf("Activate now: autoload -Uz compinit && compinit && { _todoist() { source %s; }; compdef _todoist todoist; }", shellEscape(path))
	case "fish":
		// Fish interprets backslashes before quotes and backslashes even inside
		// single quotes, so use its quoting rules instead of POSIX shell syntax.
		quoted := strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(path)
		return fmt.Sprintf("Activate now: source '%s'", quoted)
	case "powershell":
		return "Activate now: " + powerShellSourceCommand(path)
	default:
		return "Restart your shell to enable completion."
	}
}

func powerShellSourceCommand(path string) string {
	quoted := strings.ReplaceAll(path, "'", "''")
	return fmt.Sprintf(". '%s'", quoted)
}

func writeCompletionFile(shell, path, script string) error {
	if canonicalCompletionShell(shell) == "powershell" {
		existing, err := os.ReadFile(path)
		switch {
		case err == nil && string(existing) == script:
			return nil
		case err == nil && !strings.HasPrefix(string(existing), powerShellCompletionMarker+"\n"):
			return fmt.Errorf("refusing to overwrite unrecognized PowerShell completion file: %s", path)
		case err != nil && !os.IsNotExist(err):
			return fmt.Errorf("inspect completion %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		return fmt.Errorf("write completion: %w", err)
	}
	return nil
}

func isOwnedPowerShellCompletion(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read completion %s: %w", path, err)
	}
	return strings.HasPrefix(string(data), powerShellCompletionMarker+"\n"), nil
}

type completionTarget struct {
	shell string
	path  string
}

func completionUninstallTargets(ctx *Context, shell, explicitPath string) ([]completionTarget, error) {
	shell = canonicalCompletionShell(shell)
	if explicitPath != "" {
		return []completionTarget{{shell: shell, path: explicitPath}}, nil
	}
	if shell != "" {
		path := defaultCompletionPath(ctx, shell)
		if path == "" {
			return nil, &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s (supported: %s)", shell, supportedCompletionShells)}
		}
		return []completionTarget{{shell: shell, path: path}}, nil
	}
	targets := make([]completionTarget, 0, 4)
	for _, candidate := range []string{"bash", "zsh", "fish", "powershell"} {
		path := defaultCompletionPath(ctx, candidate)
		if path != "" {
			targets = append(targets, completionTarget{shell: candidate, path: path})
		}
	}
	return targets, nil
}
