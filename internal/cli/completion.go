package cli

import (
	"errors"
	"fmt"
	"strings"
)

func completionCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printCompletionHelp(ctx.Stdout)
		if len(args) == 0 {
			return &CodeError{Code: exitUsage, Err: errors.New("shell is required")}
		}
		return nil
	}
	shell := strings.ToLower(args[0])
	switch shell {
	case "bash":
		fmt.Fprint(ctx.Stdout, bashCompletion)
	case "zsh":
		fmt.Fprint(ctx.Stdout, zshCompletion)
	case "fish":
		fmt.Fprint(ctx.Stdout, fishCompletion)
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported shell: %s", shell)}
	}
	return nil
}

const bashCompletion = `# todoist completion
_todoist() {
  local cur prev cmd
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  cmd="${COMP_WORDS[1]}"

  local global_flags="--help -h --version --quiet -q --verbose -v --json --plain --no-color --no-input --timeout --config --profile --dry-run -n --force -f --base-url"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "auth task project section label comment agent completion help ${global_flags}" -- "$cur") )
    return 0
  fi

  case "$cmd" in
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
      local subs="list add update move complete reopen delete"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local task_flags="--filter --project --section --parent --label --id --cursor --limit --all --all-projects --completed --completed-by --since --until --wide --content --description --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee"
      COMPREPLY=( $(compgen -W "${task_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    project)
      local subs="list add update archive unarchive delete"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local project_flags="--archived --id --name --description --parent --color --favorite --view"
      COMPREPLY=( $(compgen -W "${project_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    section)
      local subs="list add update delete"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local section_flags="--project --name --id"
      COMPREPLY=( $(compgen -W "${section_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    label)
      local subs="list add update delete"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local label_flags="--id --name --color --favorite --unfavorite"
      COMPREPLY=( $(compgen -W "${label_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    comment)
      local subs="list add update delete"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local comment_flags="--task --project --content --id"
      COMPREPLY=( $(compgen -W "${comment_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    agent)
      local subs="plan apply status"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local agent_flags="--out --planner --plan --confirm"
      COMPREPLY=( $(compgen -W "${agent_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
  esac
}
complete -F _todoist todoist
`

const zshCompletion = `#compdef todoist
_arguments -C \
  '1:command:(auth task project section label comment agent completion help)' \
  '*::subcmd:->subcmds'

case $words[1] in
  auth)
    _arguments '2:subcommand:(login status logout)' '*:flags:(--token-stdin --print-env)'
    ;;
  task)
    _arguments '2:subcommand:(list add update move complete reopen delete)' '*:flags:(--filter --project --section --parent --label --id --cursor --limit --all --all-projects --completed --completed-by --since --until --wide --content --description --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee -n --dry-run -f --force --json --plain --no-color --no-input --quiet -q --verbose -v --timeout --config --profile --base-url)'
    ;;
  project)
    _arguments '2:subcommand:(list add update archive unarchive delete)' '*:flags:(--archived --id --name --description --parent --color --favorite --view)'
    ;;
  section)
    _arguments '2:subcommand:(list add update delete)' '*:flags:(--project --name --id)'
    ;;
  label)
    _arguments '2:subcommand:(list add update delete)' '*:flags:(--id --name --color --favorite --unfavorite)'
    ;;
  comment)
    _arguments '2:subcommand:(list add update delete)' '*:flags:(--task --project --content --id)'
    ;;
  agent)
    _arguments '2:subcommand:(plan apply status)' '*:flags:(--out --planner --plan --confirm)'
    ;;
  completion)
    _arguments '2:shell:(bash zsh fish)'
    ;;
  help)
    _arguments '2:command:(auth task project section label comment agent completion help)'
    ;;
esac
`

const fishCompletion = `# todoist completion
complete -c todoist -f -n '__fish_use_subcommand' -a 'auth task project section label comment agent completion help'

# Global flags
complete -c todoist -s h -l help -d "Show help"
complete -c todoist -l version -d "Show version"
complete -c todoist -s q -l quiet -d "Suppress non-essential output"
complete -c todoist -s v -l verbose -d "Enable verbose output"
complete -c todoist -l json -d "JSON output"
complete -c todoist -l plain -d "Plain output"
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
complete -c todoist -n '__fish_seen_subcommand_from task; and __fish_use_subcommand' -a 'list add update move complete reopen delete'
complete -c todoist -n '__fish_seen_subcommand_from task' -l filter -l project -l section -l parent -l label -l id -l cursor -l limit -l all -l all-projects -l completed -l completed-by -l since -l until -l wide -l content -l description -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee

# project
complete -c todoist -n '__fish_seen_subcommand_from project; and __fish_use_subcommand' -a 'list add update archive unarchive delete'
complete -c todoist -n '__fish_seen_subcommand_from project' -l archived -l id -l name -l description -l parent -l color -l favorite -l view

# section
complete -c todoist -n '__fish_seen_subcommand_from section; and __fish_use_subcommand' -a 'list add update delete'
complete -c todoist -n '__fish_seen_subcommand_from section' -l project -l name -l id

# label
complete -c todoist -n '__fish_seen_subcommand_from label; and __fish_use_subcommand' -a 'list add update delete'
complete -c todoist -n '__fish_seen_subcommand_from label' -l id -l name -l color -l favorite -l unfavorite

# comment
complete -c todoist -n '__fish_seen_subcommand_from comment; and __fish_use_subcommand' -a 'list add update delete'
complete -c todoist -n '__fish_seen_subcommand_from comment' -l task -l project -l content -l id

# agent
complete -c todoist -n '__fish_seen_subcommand_from agent; and __fish_use_subcommand' -a 'plan apply status'
complete -c todoist -n '__fish_seen_subcommand_from agent' -l out -l planner -l plan -l confirm

# completion helper
complete -c todoist -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
