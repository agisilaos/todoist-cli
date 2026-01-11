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
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  if [[ ${COMP_CWORD} == 1 ]]; then
    COMPREPLY=( $(compgen -W "auth task project section label comment agent completion help" -- "$cur") )
    return 0
  fi
}
complete -F _todoist todoist
`

const zshCompletion = `#compdef todoist
_arguments '1: :->cmds'
case $state in
  cmds)
    _values "commands" auth task project section label comment agent completion help
    ;;
esac
`

const fishCompletion = `complete -c todoist -f -n '__fish_use_subcommand' -a 'auth task project section label comment agent completion help'
`
