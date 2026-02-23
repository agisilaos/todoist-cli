package cli

const bashCompletion = `# todoist completion
_todoist() {
  local cur prev cmd
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  cmd="${COMP_WORDS[1]}"

  local global_flags="--help -h --version --quiet -q --quiet-json --verbose -v --accessible --json --plain --ndjson --no-color --no-input --timeout --config --profile --dry-run -n --force -f --fuzzy --no-fuzzy --progress-jsonl --base-url"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "today completed upcoming inbox add auth task filter project workspace section label comment reminder notification activity stats settings view agent completion doctor schema planner help ${global_flags}" -- "$cur") )
    return 0
  fi

  case "$cmd" in
    upcoming)
      local upcoming_flags="--days --project --label --wide --sort --truncate-width"
      COMPREPLY=( $(compgen -W "${upcoming_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    completed)
      local completed_flags="--completed-by --since --until --project --section --filter --cursor --limit --all --wide"
      COMPREPLY=( $(compgen -W "${completed_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
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
        COMPREPLY=( $(compgen -W "--token-stdin --print-env --oauth --oauth-device --no-browser --client-id --oauth-authorize-url --oauth-token-url --oauth-device-url --oauth-listen --oauth-redirect-uri ${global_flags}" -- "$cur") )
        return 0
      fi
      ;;
    task)
      local subs="list ls add view show update move complete reopen delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local task_flags="--filter --project --section --parent --label --id --cursor --limit --all --all-projects --completed --completed-by --since --until --wide --content --description --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --quick --natural --preset --sort --truncate-width --yes"
      COMPREPLY=( $(compgen -W "${task_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    filter)
      local subs="list ls show add update delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local filter_flags="--id --name --query --color --favorite --unfavorite --yes"
      COMPREPLY=( $(compgen -W "${filter_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    project)
      local subs="list ls view show collaborators add update archive unarchive delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local project_flags="--archived --id --name --description --parent --color --favorite --view --cursor --limit --all"
      COMPREPLY=( $(compgen -W "${project_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    workspace)
      local subs="list ls"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "${global_flags}" -- "$cur") )
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
    reminder)
      local subs="list ls add update delete rm del"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local reminder_flags="--task --id --before --at --yes"
      COMPREPLY=( $(compgen -W "${reminder_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    notification)
      local subs="list view accept reject read unread"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local notification_flags="--type --unread --read --limit --offset --id --all --yes"
      COMPREPLY=( $(compgen -W "${notification_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    activity)
      local activity_flags="--since --until --type --event --project --by --limit --cursor --all"
      COMPREPLY=( $(compgen -W "${activity_flags} ${global_flags}" -- "$cur") )
      return 0
      ;;
    stats)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "goals vacation" -- "$cur") )
        return 0
      fi
      if [[ ${COMP_WORDS[2]} == "goals" ]]; then
        COMPREPLY=( $(compgen -W "--daily --weekly ${global_flags}" -- "$cur") )
        return 0
      fi
      if [[ ${COMP_WORDS[2]} == "vacation" ]]; then
        COMPREPLY=( $(compgen -W "--on --off ${global_flags}" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "${global_flags}" -- "$cur") )
      return 0
      ;;
    settings)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "view update themes" -- "$cur") )
        return 0
      fi
      if [[ ${COMP_WORDS[2]} == "update" ]]; then
        local settings_flags="--timezone --time-format --date-format --start-day --theme --auto-reminder --next-week --start-page --reminder-push --reminder-desktop --reminder-email --completed-sound-desktop --completed-sound-mobile"
        COMPREPLY=( $(compgen -W "${settings_flags} ${global_flags}" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "${global_flags}" -- "$cur") )
      return 0
      ;;
    view)
      COMPREPLY=( $(compgen -W "${global_flags}" -- "$cur") )
      return 0
      ;;
    agent)
      local subs="plan apply run schedule examples planner status"
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${subs}" -- "$cur") )
        return 0
      fi
      local agent_flags="--out --planner --policy --plan --confirm --instruction --on-error --plan-version --context-project --context-label --context-completed"
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
    doctor)
      COMPREPLY=( $(compgen -W "--strict ${global_flags}" -- "$cur") )
      return 0
      ;;
  esac
}
complete -F _todoist todoist
`

const zshCompletion = `#compdef todoist
_arguments -C \
  '1:command:(today completed upcoming inbox add auth task filter project workspace section label comment reminder notification activity stats settings view agent completion doctor schema planner help)' \
  '*::subcmd:->subcmds'

case $words[1] in
  inbox)
    _arguments '2:subcommand:(add)' '*:flags:(--content --description --section --label --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee)'
    ;;
  today)
    _arguments
    ;;
  upcoming)
    _arguments '*:flags:(--days --project --label --wide --sort --truncate-width)'
    ;;
  completed)
    _arguments '*:flags:(--completed-by --since --until --project --section --filter --cursor --limit --all --wide)'
    ;;
  add)
    _arguments '*:flags:(--content --description --project --section --parent --label --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --strict)'
    ;;
  auth)
    _arguments '2:subcommand:(login status logout)' '*:flags:(--token-stdin --print-env --oauth --oauth-device --no-browser --client-id --oauth-authorize-url --oauth-token-url --oauth-device-url --oauth-listen --oauth-redirect-uri)'
    ;;
  task)
    _arguments '2:subcommand:(list ls add view show update move complete reopen delete rm del)' '*:flags:(--filter --project --section --parent --label --id --cursor --limit --all --all-projects --completed --completed-by --since --until --wide --content --description --priority --due --due-date --due-datetime --due-lang --duration --duration-unit --deadline --assignee --quick --natural --full --yes -n --dry-run -f --force --accessible --json --plain --ndjson --no-color --no-input --quiet -q --quiet-json --verbose -v --timeout --config --profile --fuzzy --no-fuzzy --progress-jsonl --base-url)'
    ;;
  filter)
    _arguments '2:subcommand:(list ls show add update delete rm del)' '*:flags:(--id --name --query --color --favorite --unfavorite --yes)'
    ;;
  project)
    _arguments '2:subcommand:(list ls view show collaborators add update archive unarchive delete rm del)' '*:flags:(--archived --id --name --description --parent --color --favorite --view --cursor --limit --all)'
    ;;
  workspace)
    _arguments '2:subcommand:(list ls)'
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
  reminder)
    _arguments '2:subcommand:(list ls add update delete rm del)' '*:flags:(--task --id --before --at --yes)'
    ;;
  notification)
    _arguments '2:subcommand:(list view accept reject read unread)' '*:flags:(--type --unread --read --limit --offset --id --all --yes)'
    ;;
  activity)
    _arguments '*:flags:(--since --until --type --event --project --by --limit --cursor --all)'
    ;;
  stats)
    _arguments '2:subcommand:(goals vacation)' '*:flags:(--daily --weekly --on --off)'
    ;;
  settings)
    _arguments '2:subcommand:(view update themes)' '*:flags:(--timezone --time-format --date-format --start-day --theme --auto-reminder --next-week --start-page --reminder-push --reminder-desktop --reminder-email --completed-sound-desktop --completed-sound-mobile)'
    ;;
  view)
    _arguments
    ;;
  agent)
    _arguments '2:subcommand:(plan apply run schedule examples planner status)' '*:flags:(--out --planner --policy --plan --confirm --instruction --on-error --plan-version --context-project --context-label --context-completed)'
    ;;
  schema)
    _arguments '*:flags:(--name)'
    ;;
  planner)
    _arguments '*:flags:(--set --cmd)'
    ;;
  doctor)
    _arguments '*:flags:(--strict)'
    ;;
  completion)
    _arguments '2:shell:(bash zsh fish)'
    ;;
  help)
    _arguments '2:command:(today completed upcoming inbox add auth task project section label comment reminder notification activity stats settings view agent completion doctor schema planner help)'
    ;;
esac
`

const fishCompletion = `# todoist completion
complete -c todoist -f -n '__fish_use_subcommand' -a 'today completed upcoming inbox add auth task filter project workspace section label comment reminder notification activity stats settings view agent completion doctor schema planner help'

# Global flags
complete -c todoist -s h -l help -d "Show help"
complete -c todoist -l version -d "Show version"
complete -c todoist -s q -l quiet -d "Suppress non-essential output"
complete -c todoist -l quiet-json -d "Compact single-line JSON errors"
complete -c todoist -s v -l verbose -d "Enable verbose output"
complete -c todoist -l accessible -d "Add screen-reader-friendly labels in human output"
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
complete -c todoist -l fuzzy -d "Enable fuzzy name resolution"
complete -c todoist -l no-fuzzy -d "Disable fuzzy name resolution"
complete -c todoist -l progress-jsonl -d "Emit progress events as JSONL"
complete -c todoist -l base-url -d "Override API base URL"

# auth
complete -c todoist -n '__fish_seen_subcommand_from auth; and __fish_use_subcommand' -a 'login status logout'
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l token-stdin -d "Read token from stdin"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l print-env -d "Print TODOIST_TOKEN export"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth -d "Authenticate via OAuth PKCE flow"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth-device -d "Authenticate via OAuth device flow"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l no-browser -d "Do not auto-open browser for OAuth flow"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l client-id -d "OAuth client ID"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth-authorize-url -d "OAuth authorize URL"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth-token-url -d "OAuth token URL"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth-device-url -d "OAuth device code URL"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth-listen -d "OAuth callback listen address"
complete -c todoist -n '__fish_seen_subcommand_from auth; and contains login (commandline -opc)' -l oauth-redirect-uri -d "OAuth redirect URI"

# task
complete -c todoist -n '__fish_seen_subcommand_from task; and __fish_use_subcommand' -a 'list ls add view show update move complete reopen delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from task' -l filter -l project -l section -l parent -l label -l id -l cursor -l limit -l all -l all-projects -l completed -l completed-by -l since -l until -l wide -l content -l description -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee -l full -l yes

# project
complete -c todoist -n '__fish_seen_subcommand_from project; and __fish_use_subcommand' -a 'list ls view show collaborators add update archive unarchive delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from project' -l archived -l id -l name -l description -l parent -l color -l favorite -l view -l cursor -l limit -l all

# workspace
complete -c todoist -n '__fish_seen_subcommand_from workspace; and __fish_use_subcommand' -a 'list ls'

# filter
complete -c todoist -n '__fish_seen_subcommand_from filter; and __fish_use_subcommand' -a 'list ls show add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from filter' -l id -l name -l query -l color -l favorite -l unfavorite -l yes

# section
complete -c todoist -n '__fish_seen_subcommand_from section; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from section' -l project -l name -l id

# label
complete -c todoist -n '__fish_seen_subcommand_from label; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from label' -l id -l name -l color -l favorite -l unfavorite

# comment
complete -c todoist -n '__fish_seen_subcommand_from comment; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from comment' -l task -l project -l content -l id

# reminder
complete -c todoist -n '__fish_seen_subcommand_from reminder; and __fish_use_subcommand' -a 'list ls add update delete rm del'
complete -c todoist -n '__fish_seen_subcommand_from reminder' -l task -l id -l before -l at -l yes

# notification
complete -c todoist -n '__fish_seen_subcommand_from notification; and __fish_use_subcommand' -a 'list view accept reject read unread'
complete -c todoist -n '__fish_seen_subcommand_from notification' -l type -l unread -l read -l limit -l offset -l id -l all -l yes

# activity
complete -c todoist -n '__fish_seen_subcommand_from activity' -l since -l until -l type -l event -l project -l by -l limit -l cursor -l all

# stats
complete -c todoist -n '__fish_seen_subcommand_from stats'
complete -c todoist -n '__fish_seen_subcommand_from stats; and __fish_use_subcommand' -a 'goals vacation'
complete -c todoist -n '__fish_seen_subcommand_from stats; and contains goals (commandline -opc)' -l daily -l weekly
complete -c todoist -n '__fish_seen_subcommand_from stats; and contains vacation (commandline -opc)' -l on -l off

# settings
complete -c todoist -n '__fish_seen_subcommand_from settings; and __fish_use_subcommand' -a 'view update themes'
complete -c todoist -n '__fish_seen_subcommand_from settings; and contains update (commandline -opc)' -l timezone -l time-format -l date-format -l start-day -l theme -l auto-reminder -l next-week -l start-page -l reminder-push -l reminder-desktop -l reminder-email -l completed-sound-desktop -l completed-sound-mobile

# inbox
complete -c todoist -n '__fish_seen_subcommand_from inbox; and __fish_use_subcommand' -a 'add'
complete -c todoist -n '__fish_seen_subcommand_from inbox' -l content -l description -l section -l label -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee

# today
complete -c todoist -n '__fish_seen_subcommand_from today'

# completed
complete -c todoist -n '__fish_seen_subcommand_from completed' -l completed-by -l since -l until -l project -l section -l filter -l cursor -l limit -l all -l wide

# upcoming
complete -c todoist -n '__fish_seen_subcommand_from upcoming' -l days -l project -l label -l wide -l sort -l truncate-width

# add alias
complete -c todoist -n '__fish_seen_subcommand_from add' -l content -l description -l project -l section -l parent -l label -l priority -l due -l due-date -l due-datetime -l due-lang -l duration -l duration-unit -l deadline -l assignee -l strict

# agent
complete -c todoist -n '__fish_seen_subcommand_from agent; and __fish_use_subcommand' -a 'plan apply run schedule examples planner status'
complete -c todoist -n '__fish_seen_subcommand_from agent' -l out -l planner -l policy -l plan -l confirm -l instruction -l on-error -l plan-version -l context-project -l context-label -l context-completed

# doctor
complete -c todoist -n '__fish_seen_subcommand_from doctor' -l strict

# schema
complete -c todoist -n '__fish_seen_subcommand_from schema' -l name

# planner
complete -c todoist -n '__fish_seen_subcommand_from planner' -l set
complete -c todoist -n '__fish_seen_subcommand_from planner' -l cmd

# completion helper
complete -c todoist -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
