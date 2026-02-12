package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
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

	shell := strings.ToLower(args[0])
	script, err := completionScript(shell)
	if err != nil {
		return err
	}
	fmt.Fprint(ctx.Stdout, script)
	return nil
}

func completionInstall(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("completion install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var path string
	var help bool
	fs.StringVar(&path, "path", "", "Install path override")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
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
			"shell": shell,
			"path":  path,
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Installed %s completion to %s\n", shell, path)
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

const bashCompletion = `# todoist completion
_todoist() {
  local cur prev cmd
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  cmd="${COMP_WORDS[1]}"

  local global_flags="--help -h --version --quiet -q --quiet-json --verbose -v --json --plain --ndjson --no-color --no-input --timeout --config --profile --dry-run -n --force -f --base-url"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "today inbox add auth task project section label comment agent completion schema planner help ${global_flags}" -- "$cur") )
    return 0
  fi

  case "$cmd" in
    add)
      local task_flags="--content --description --project --section --parent --label --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --strict"
      COMPREPLY=( $(compgen -W "${task_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    inbox)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "add" -- "$cur") )
        return 0
      fi
      local inbox_flags="--content --description --section --label --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee"
      COMPREPLY=( $(compgen -W "${inbox_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    auth)
      local subs="login status logout"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      if [[ ${COMP_WORDS[2]} == "login" ]]; then
        COMPREPLY=( $(compgen -W "--token-stdin --print-env ${global_flags}" -- "$cur") )
        return 0
      fi
      ;;
    task)
      local subs="list ls add view show update move complete reopen delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local task_flags="--filter --project --section --parent --label --id --cursor --limit --all --all-projects --completed --completed-by --since --until --wide --content --description --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --quick --preset --sort --truncate-width --yes"
      COMPREPLY=( $(compgen -W "${task_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    project)
      local subs="list ls add update archive unarchive delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local project_flags="--archived --id --name --description --parent --color --favorite --view"
      COMPREPLY=( $(compgen -W "${project_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    section)
      local subs="list ls add update delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local section_flags="--project --name --id"
      COMPREPLY=( $(compgen -W "${section_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    label)
      local subs="list ls add update delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local label_flags="--id --name --color --favorite --unfavorite"
      COMPREPLY=( $(compgen -W "${label_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    comment)
      local subs="list ls add update delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local comment_flags="--task --project --content --id"
      COMPREPLY=( $(compgen -W "${comment_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    agent)
      local subs="plan apply run schedule examples planner status"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local agent_flags="--out --planner --plan --confirm --instruction --on-error --plan-version --context-project --context-label --context-completed"
      COMPREPLY=( $(compgen -W "${agent_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    schema)
      local schema_flags="--name"
      COMPREPLY=( $(compgen -W "${schema_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    planner)
      local planner_flags="--set --cmd"
      COMPREPLY=( $(compgen -W "${planner_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
  esac
}
complete -F _todoist todoist
`

const zshCompletion = `#compdef todoist
_arguments -C \
  '1:command:(today inbox add auth task project section label comment agent completion schema planner help)' \
  '*::subcmd:->subcmds'

case $words[1] in
  inbox)
    _arguments '2:subcommand:(add)' '*:flags:(--content --description --section --label --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee)'
    ;;
  today)
    _arguments
    ;;
  add)
    _arguments '*:flags:(--content --description --project --section --parent --label --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --strict)'
    ;;
  auth)
    _arguments '2:subcommand:(login status logout)' '*:flags:(--token-stdin --print-env)'
    ;;
  task)
    _arguments '2:subcommand:(list ls add view show update move complete reopen delete rm del)' '*:flags:(--filter --project --section --parent --label --id --cursor --limit --all --all-projects --completed --completed-by --since --until --wide --content --description --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --quick --full --yes -n --dry-run -f --force --json --plain --ndjson --no-color --no-input --quiet -q --quiet-json --verbose -v --timeout --config --profile --base-url)'
    ;;
  project)
    _arguments '2:subcommand:(list ls add update archive unarchive delete rm del)' '*:flags:(--archived --id --name --description --parent --color --favorite --view)'
    ;;
  section)
    _arguments '2:subcommand:(list ls add update delete rm del)' '*:flags:(--project --name --id)'
    ;;
  label)
    _arguments '2:subcommand:(list ls add update delete rm del)' '*:flags:(--id --name --color --favorite --unfavorite)'
    ;;
  comment)
    _arguments '2:subcommand:(list ls add update delete rm del)' '*:flags:(--task --project --content --id)'
    ;;
  agent)
    _arguments '2:subcommand:(plan apply run schedule examples planner status)' '*:flags:(--out --planner --plan --confirm --instruction --on-error --plan-version --context-project --context-label --context-completed)'
    ;;
  schema)
    _arguments '*:flags:(--name)'
    ;;
  planner)
    _arguments '*:flags:(--set --cmd)'
    ;;
  completion)
    _arguments '2:shell:(bash zsh fish)'
    ;;
  help)
    _arguments '2:command:(today inbox add auth task project section label comment agent completion schema planner help)'
    ;;
esac
`

const fishCompletion = `# todoist completion
complete -c todoist -f -n '__fish_use_subcommand' -a 'today inbox add auth task project section label comment agent completion schema planner help'

# Global flags
complete -c todoist -s h -l help -d "Show help"
complete -c todoist -l version -d "Show version"
complete -c todoist -s q -l quiet -d "Suppress non-essential output"
complete -c todoist -l quiet-json -d "Compact single-line JSON errors"
complete -c todoist -s v -l verbose -d "Enable verbose output"
complete -c todoist -l json -d "JSON output"
complete -c todoist -l plain -d "Plain output"
complete -c todoist -l ndjson -d "NDJSON output"
complete -c todoist -l no-color -d "Disable color"
complete -c todoist -l no-input -d "Disable prompts"
complete -c todoist -l timeout -d "Request timeout"
complete -c todoist -l config -d "Config file path"
complete -c todoist -l profile -d "Profile name"
complete -c todoist -s n -l dry-run -d "Preview changes without applying"
complete -c todoist -s f -l force -d "Skip confirmation prompts"
complete -c todoist -l base-url -d "Override API base URL"

# auth
complete -c todoist -n '__fish_seen_subcommand_from auth; and __fish_use_subcommand' -a 'login status logout'
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l token-stdin -d "Read token from stdin"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l print-env -d "Print TODOIST_TOKEN export"

# task
complete -c todoist -n '__fish_seen_subcommand_from task; and __fish_use_subcommand' -a 'list ls add view show update move complete reopen delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from task' -l filter -l project -l section -l parent -l label -l id -l cursor -l limit -l all -l all-projects -l completed -l completed-by -l since -l until -l wide -l content -l description -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee -l full -l yes

# project
complete -c todoist -n '__fish_seen_subcommand_from project; and __fish_use_subcommand' -a 'list ls add update archive unarchive delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from project' -l archived -l id -l name -l description -l parent -l color -l favorite -l view

# section
complete -c todoist -n '__fish_seen_subcommand_from section; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from section' -l project -l name -l id

# label
complete -c todoist -n '__fish_seen_subcommand_from label; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from label' -l id -l name -l color -l favorite -l unfavorite

# comment
complete -c todoist -n '__fish_seen_subcommand_from comment; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from comment' -l task -l project -l content -l id

# inbox
complete -c todoist -n '__fish_seen_subcommand_from inbox; and __fish_use_subcommand' -a 'add'
complete -c todoist -n '__fish_seen_subcommand_from inbox' -l content -l description -l section -l label -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee

# today
complete -c todoist -n '__fish_seen_subcommand_from today'

# add alias
complete -c todoist -n '__fish_seen_subcommand_from add' -l content -l description -l project -l section -l parent -l label -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee -l strict

# agent
complete -c todoist -n '__fish_seen_subcommand_from agent; and __fish_use_subcommand' -a 'plan apply run schedule examples planner status'
complete -c todoist -n '__fish_seen_subcommand_from agent' -l out -l planner -l plan -l confirm -l instruction -l on-error -l plan-version -l context-project -l context-label -l context-completed

# schema
complete -c todoist -n '__fish_seen_subcommand_from schema' -l name

# planner
complete -c todoist -n '__fish_seen_subcommand_from planner' -l set
complete -c todoist -n '__fish_seen_subcommand_from planner' -l cmd

# completion helper
complete -c todoist -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
