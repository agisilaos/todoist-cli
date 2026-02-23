package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func completionCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printCompletionHelp(ctx.Stdout)
		if len(args) == 0 {
			return &CodeError{Code: exitUsage, Err: errors.New("shell is required")}
		}
		return nil
	}

	if args[0] == "install" {
		return completionInstall(ctx, args[1:])
	}
	if args[0] == "uninstall" {
		return completionUninstall(ctx, args[1:])
	}

	shell := strings.ToLower(args[0])
	script, err := completionScript(shell)
	if err != nil {
		return err
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
		shell = strings.ToLower(fs.Arg(0))
	}
	if shell == "" {
		shell = detectShell()
	}
	if shell == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("shell is required")}
	}
	script, err := completionScript(shell)
	if err != nil {
		return err
	}
	if path == "" {
		path = defaultCompletionPath(shell)
		if path == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s", shell)}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create completion dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		return fmt.Errorf("write completion: %w", err)
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"shell":      shell,
			"path":       path,
			"activation": completionActivationHint(shell, path),
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Installed %s completion to %s\n", shell, path)
	fmt.Fprintln(ctx.Stdout, completionActivationHint(shell, path))
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
		shell = strings.ToLower(fs.Arg(0))
	}

	paths, err := completionUninstallPaths(shell, path)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		if ctx.Mode == output.ModeJSON {
			return output.WriteJSON(ctx.Stdout, map[string]any{
				"removed": []string{},
			}, output.Meta{})
		}
		fmt.Fprintln(ctx.Stdout, "No completion scripts found to remove.")
		return nil
	}

	removed := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr != nil {
			continue
		}
		if err := os.Remove(p); err != nil {
			return fmt.Errorf("remove completion %s: %w", p, err)
		}
		removed = append(removed, p)
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"removed": removed,
		}, output.Meta{})
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
	switch shell {
	case "bash":
		return bashCompletion, nil
	case "zsh":
		return zshCompletion, nil
	case "fish":
		return fishCompletion, nil
	default:
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s", shell)}
	}
}

func defaultCompletionPath(shell string) string {
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			xdg = filepath.Join(home, ".local", "share")
		}
	}
	switch shell {
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
	}
	return ""
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	parts := strings.Split(shell, "/")
	return strings.ToLower(parts[len(parts)-1])
}

func completionActivationHint(shell, path string) string {
	switch shell {
	case "bash":
		return fmt.Sprintf("Activate now: source %s", path)
	case "zsh":
		return fmt.Sprintf("Activate now: source %s (ensure its directory is in $fpath, then run: autoload -U compinit && compinit)", path)
	case "fish":
		return fmt.Sprintf("Activate now: source %s", path)
	default:
		return "Restart your shell to enable completion."
	}
}

func completionUninstallPaths(shell, explicitPath string) ([]string, error) {
	if explicitPath != "" {
		return []string{explicitPath}, nil
	}
	if shell != "" {
		path := defaultCompletionPath(shell)
		if path == "" {
			return nil, &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s", shell)}
		}
		return []string{path}, nil
	}
	paths := make([]string, 0, 3)
	for _, candidate := range []string{"bash", "zsh", "fish"} {
		path := defaultCompletionPath(candidate)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
