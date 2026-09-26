package cli

const powerShellCompletionMarker = "# todoist completion (powershell)"

const powerShellCompletion = powerShellCompletionMarker + `
$todoistCommands = @{
    '' = @('today', 'completed', 'upcoming', 'inbox', 'add', 'auth', 'task', 'filter', 'project', 'workspace', 'section', 'label', 'comment', 'reminder', 'notification', 'activity', 'stats', 'settings', 'view', 'agent', 'completion', 'doctor', 'schema', 'planner', 'help')
    'inbox' = @('add')
    'auth' = @('login', 'status', 'logout')
    'task' = @('list', 'ls', 'add', 'view', 'show', 'update', 'move', 'complete', 'reopen', 'delete', 'rm', 'del')
    'filter' = @('list', 'ls', 'show', 'add', 'update', 'delete', 'rm', 'del')
    'project' = @('list', 'ls', 'view', 'show', 'browse', 'collaborators', 'add', 'create', 'update', 'move', 'archive', 'unarchive', 'delete', 'rm', 'del')
    'workspace' = @('list', 'ls')
    'section' = @('list', 'ls', 'add', 'update', 'delete', 'rm', 'del')
    'label' = @('list', 'ls', 'add', 'update', 'delete', 'rm', 'del')
    'comment' = @('list', 'ls', 'add', 'update', 'delete', 'rm', 'del')
    'reminder' = @('list', 'ls', 'add', 'update', 'delete', 'rm', 'del')
    'notification' = @('list', 'ls', 'view', 'accept', 'reject', 'read', 'unread')
    'stats' = @('goals', 'vacation')
    'settings' = @('view', 'update', 'themes')
    'agent' = @('plan', 'apply', 'run', 'schedule', 'examples', 'planner', 'status')
    'agent schedule' = @('print')
    'completion' = @('bash', 'zsh', 'fish', 'powershell', 'pwsh', 'install', 'uninstall')
    'completion install' = @('bash', 'zsh', 'fish', 'powershell', 'pwsh')
    'completion uninstall' = @('bash', 'zsh', 'fish', 'powershell', 'pwsh')
    'help' = @('today', 'completed', 'upcoming', 'inbox', 'add', 'auth', 'task', 'filter', 'project', 'workspace', 'section', 'label', 'comment', 'reminder', 'notification', 'activity', 'stats', 'settings', 'view', 'agent', 'completion', 'doctor', 'schema', 'planner', 'help')
}

$todoistAliases = @{
    'task ls' = 'task list'
    'task show' = 'task view'
    'task rm' = 'task delete'
    'task del' = 'task delete'
    'filter ls' = 'filter list'
    'filter rm' = 'filter delete'
    'filter del' = 'filter delete'
    'project ls' = 'project list'
    'project show' = 'project view'
    'project create' = 'project add'
    'project rm' = 'project delete'
    'project del' = 'project delete'
    'workspace ls' = 'workspace list'
    'section ls' = 'section list'
    'section rm' = 'section delete'
    'section del' = 'section delete'
    'label ls' = 'label list'
    'label rm' = 'label delete'
    'label del' = 'label delete'
    'comment ls' = 'comment list'
    'comment rm' = 'comment delete'
    'comment del' = 'comment delete'
    'reminder ls' = 'reminder list'
    'reminder rm' = 'reminder delete'
    'reminder del' = 'reminder delete'
    'notification ls' = 'notification list'
}

$todoistGlobalFlags = @(
    '-h', '--help', '--version', '-q', '--quiet', '--quiet-json', '-v', '--verbose',
    '--accessible', '--json', '--plain', '--ndjson', '--ids-only', '--no-color',
    '--no-input', '--timeout', '--config', '--profile', '-n', '--dry-run', '-f',
    '--force', '--fuzzy', '--no-fuzzy', '--progress-jsonl', '--base-url'
)

$todoistFlags = @{
    'completed' = @('--completed-by', '--since', '--until', '--project', '--section', '--filter', '--cursor', '--limit', '--all', '--wide')
    'upcoming' = @('--days', '--project', '--label', '--wide', '--sort', '--truncate-width')
    'inbox add' = @('--content', '--description', '--section', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee')
    'add' = @('--content', '--project', '--section', '--label', '--priority', '--due', '--strict')
    'auth login' = @('--token-stdin', '--print-env', '--oauth', '--oauth-device', '--read-only', '--no-browser', '--client-id', '--oauth-authorize-url', '--oauth-token-url', '--oauth-device-url', '--oauth-listen', '--oauth-redirect-uri')
    'task list' = @('--filter', '--project', '--section', '--parent', '--label', '--id', '--cursor', '--limit', '--all', '--all-projects', '--completed', '--completed-by', '--since', '--until', '--wide', '--preset', '--sort', '--truncate-width')
    'task add' = @('--content', '--description', '--project', '--section', '--parent', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee', '--quick', '--natural')
    'task view' = @('--id', '--full')
    'task update' = @('--id', '--content', '--description', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee', '--project', '--natural')
    'task move' = @('--id', '--project', '--section', '--parent', '--filter', '--yes')
    'task complete' = @('--id', '--filter', '--yes')
    'task reopen' = @('--id')
    'task delete' = @('--id', '--yes')
    'filter add' = @('--name', '--query', '--color', '--favorite')
    'filter update' = @('--id', '--name', '--query', '--color', '--favorite', '--unfavorite')
    'filter delete' = @('--id', '--yes')
    'project list' = @('--archived', '--cursor', '--limit', '--all')
    'project view' = @('--id')
    'project browse' = @('--id')
    'project collaborators' = @('--id', '--cursor', '--limit', '--all')
    'project add' = @('--name', '--description', '--parent', '--color', '--favorite', '--view', '--workspace')
    'project update' = @('--id', '--name', '--description', '--color', '--favorite', '--view')
    'project move' = @('--id', '--to-workspace', '--to-personal', '--visibility', '--yes')
    'project archive' = @('--id')
    'project unarchive' = @('--id')
    'project delete' = @('--id')
    'section list' = @('--project', '--cursor', '--limit', '--all')
    'section add' = @('--name', '--project')
    'section update' = @('--id', '--name')
    'section delete' = @('--id')
    'label list' = @('--cursor', '--limit', '--all')
    'label add' = @('--name', '--color', '--order', '--favorite')
    'label update' = @('--id', '--name', '--color', '--order', '--favorite', '--unfavorite')
    'label delete' = @('--id')
    'comment list' = @('--task', '--project', '--cursor', '--limit', '--all')
    'comment add' = @('--content', '--task', '--project')
    'comment update' = @('--id', '--content')
    'comment delete' = @('--id')
    'reminder list' = @('--task')
    'reminder add' = @('--task', '--before', '--at')
    'reminder update' = @('--id', '--before', '--at')
    'reminder delete' = @('--id', '--yes')
    'notification list' = @('--type', '--unread', '--read', '--limit', '--offset')
    'notification view' = @('--id')
    'notification accept' = @('--id')
    'notification reject' = @('--id')
    'notification read' = @('--id', '--all', '--yes')
    'notification unread' = @('--id')
    'activity' = @('--since', '--until', '--type', '--event', '--project', '--by', '--limit', '--cursor', '--all')
    'stats goals' = @('--daily', '--weekly')
    'stats vacation' = @('--on', '--off')
    'settings update' = @('--timezone', '--time-format', '--date-format', '--start-day', '--theme', '--auto-reminder', '--next-week', '--start-page', '--reminder-push', '--reminder-desktop', '--reminder-email', '--completed-sound-desktop', '--completed-sound-mobile')
    'agent plan' = @('--out', '--planner', '--plan-version', '--context-project', '--context-label', '--context-completed')
    'agent apply' = @('--plan', '--confirm', '--planner', '--policy', '--on-error', '--plan-version', '--context-project', '--context-label', '--context-completed')
    'agent run' = @('--plan', '--confirm', '--planner', '--policy', '--instruction', '--out', '--on-error', '--plan-version', '--context-project', '--context-label', '--context-completed')
    'agent schedule print' = @('--weekly', '--policy', '--planner', '--instruction', '--plan', '--confirm', '--force', '--dry-run', '--on-error', '--plan-version', '--context-project', '--context-label', '--context-completed', '--cron', '--bin')
    'agent planner' = @('--set', '--cmd')
    'completion install' = @('--path')
    'completion uninstall' = @('--path')
    'doctor' = @('--strict')
    'schema' = @('--name')
    'planner' = @('--set', '--cmd')
}

$todoistValueFlags = @{
    '' = @('--timeout', '--config', '--profile', '--progress-jsonl', '--base-url')
    'completed' = @('--completed-by', '--since', '--until', '--project', '--section', '--filter', '--cursor', '--limit')
    'upcoming' = @('--days', '--project', '--label', '--sort', '--truncate-width')
    'inbox add' = @('--content', '--description', '--section', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee')
    'add' = @('--content', '--project', '--section', '--label', '--priority', '--due')
    'auth login' = @('--client-id', '--oauth-authorize-url', '--oauth-token-url', '--oauth-device-url', '--oauth-listen', '--oauth-redirect-uri')
    'task list' = @('--filter', '--project', '--section', '--parent', '--label', '--id', '--cursor', '--limit', '--completed-by', '--since', '--until', '--preset', '--sort', '--truncate-width')
    'task add' = @('--content', '--description', '--project', '--section', '--parent', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee')
    'task view' = @('--id')
    'task update' = @('--id', '--content', '--description', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee', '--project')
    'task move' = @('--id', '--project', '--section', '--parent', '--filter')
    'task complete' = @('--id', '--filter')
    'task reopen' = @('--id')
    'task delete' = @('--id')
    'filter add' = @('--name', '--query', '--color')
    'filter update' = @('--id', '--name', '--query', '--color')
    'filter delete' = @('--id')
    'project list' = @('--cursor', '--limit')
    'project view' = @('--id')
    'project browse' = @('--id')
    'project collaborators' = @('--id', '--cursor', '--limit')
    'project add' = @('--name', '--description', '--parent', '--color', '--view', '--workspace')
    'project update' = @('--id', '--name', '--description', '--color', '--view')
    'project move' = @('--id', '--to-workspace', '--visibility')
    'project archive' = @('--id')
    'project unarchive' = @('--id')
    'project delete' = @('--id')
    'section list' = @('--project', '--cursor', '--limit')
    'section add' = @('--name', '--project')
    'section update' = @('--id', '--name')
    'section delete' = @('--id')
    'label list' = @('--cursor', '--limit')
    'label add' = @('--name', '--color', '--order')
    'label update' = @('--id', '--name', '--color', '--order')
    'label delete' = @('--id')
    'comment list' = @('--task', '--project', '--cursor', '--limit')
    'comment add' = @('--content', '--task', '--project')
    'comment update' = @('--id', '--content')
    'comment delete' = @('--id')
    'reminder list' = @('--task')
    'reminder add' = @('--task', '--before', '--at')
    'reminder update' = @('--id', '--before', '--at')
    'reminder delete' = @('--id')
    'notification list' = @('--type', '--limit', '--offset')
    'notification view' = @('--id')
    'notification accept' = @('--id')
    'notification reject' = @('--id')
    'notification read' = @('--id')
    'notification unread' = @('--id')
    'activity' = @('--since', '--until', '--type', '--event', '--project', '--by', '--limit', '--cursor')
    'stats goals' = @('--daily', '--weekly')
    'settings update' = @('--timezone', '--time-format', '--date-format', '--start-day', '--theme', '--auto-reminder', '--next-week', '--start-page', '--reminder-push', '--reminder-desktop', '--reminder-email', '--completed-sound-desktop', '--completed-sound-mobile')
    'agent plan' = @('--out', '--planner', '--plan-version', '--context-project', '--context-label', '--context-completed')
    'agent apply' = @('--plan', '--confirm', '--planner', '--policy', '--on-error', '--plan-version', '--context-project', '--context-label', '--context-completed')
    'agent run' = @('--plan', '--confirm', '--planner', '--policy', '--instruction', '--out', '--on-error', '--plan-version', '--context-project', '--context-label', '--context-completed')
    'agent schedule print' = @('--weekly', '--policy', '--planner', '--instruction', '--plan', '--confirm', '--on-error', '--plan-version', '--context-project', '--context-label', '--context-completed', '--bin')
    'agent planner' = @('--cmd')
    'completion install' = @('--path')
    'completion uninstall' = @('--path')
    'schema' = @('--name')
    'planner' = @('--cmd')
}

$todoistValues = @{
    'completed|--completed-by' = @('completion', 'due')
    'upcoming|--sort' = @('due', 'priority')
    'add|--priority' = @('1', '2', '3', '4', 'p1', 'p2', 'p3', 'p4')
    'inbox add|--priority' = @('1', '2', '3', '4', 'p1', 'p2', 'p3', 'p4')
    'inbox add|--duration-unit' = @('minute', 'day')
    'task list|--completed-by' = @('completion', 'due')
    'task list|--preset' = @('today', 'overdue', 'next7')
    'task list|--sort' = @('due', 'priority')
    'task add|--priority' = @('1', '2', '3', '4', 'p1', 'p2', 'p3', 'p4')
    'task add|--duration-unit' = @('minute', 'day')
    'task update|--priority' = @('1', '2', '3', '4', 'p1', 'p2', 'p3', 'p4')
    'task update|--duration-unit' = @('minute', 'day')
    'project move|--visibility' = @('restricted', 'team', 'public')
    'activity|--type' = @('task', 'comment', 'project')
    'settings update|--time-format' = @('12', '12h', '24', '24h')
    'settings update|--date-format' = @('us', 'mm-dd-yyyy', 'mdy', 'intl', 'dd-mm-yyyy', 'dmy')
    'settings update|--start-day' = @('monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun')
    'settings update|--theme' = @('todoist', 'dark', 'moonstone', 'tangerine', 'kale', 'blueberry', 'lavender', 'raspberry')
	'settings update|--next-week' = @('monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun')
    'settings update|--reminder-push' = @('true', 'on', 'yes', '1', 'false', 'off', 'no', '0')
    'settings update|--reminder-desktop' = @('true', 'on', 'yes', '1', 'false', 'off', 'no', '0')
    'settings update|--reminder-email' = @('true', 'on', 'yes', '1', 'false', 'off', 'no', '0')
    'settings update|--completed-sound-desktop' = @('true', 'on', 'yes', '1', 'false', 'off', 'no', '0')
    'settings update|--completed-sound-mobile' = @('true', 'on', 'yes', '1', 'false', 'off', 'no', '0')
    'agent apply|--on-error' = @('fail', 'continue')
    'agent run|--on-error' = @('fail', 'continue')
    'agent schedule print|--on-error' = @('fail', 'continue')
    'schema|--name' = @('ids_only', 'task_list', 'task_item_ndjson', 'error', 'plan', 'plan_preview', 'planner_request')
}

$todoistCompleter = {
    param($wordToComplete, $commandAst, $cursorPosition)

    $tokens = @()
    $elements = @($commandAst.CommandElements)
    for ($index = 1; $index -lt $elements.Count; $index++) {
        $element = $elements[$index]
        if ($element.Extent.StartOffset -ge $cursorPosition) {
            continue
        }
        if ($element.Extent.EndOffset -ge $cursorPosition) {
            continue
        }
		if ($element -is [System.Management.Automation.Language.StringConstantExpressionAst]) {
			$tokens += [string]$element.Value
		} else {
			$tokens += [string]$element.Extent.Text
        }
    }

    $path = ''
    $skipValue = $false
    $afterDoubleDash = $false
    foreach ($token in $tokens) {
        if ($skipValue) {
            $skipValue = $false
            continue
        }
        if ($token -eq '--') {
            $afterDoubleDash = $true
            continue
        }
        if ($afterDoubleDash) {
            continue
        }
        if ($token.StartsWith('-')) {
            $flag = $token
            if ($token.Contains('=')) {
                $flag = $token.Substring(0, $token.IndexOf('='))
            } else {
                $valueFlags = @($todoistValueFlags['']) + @($todoistValueFlags[$path])
                if ($valueFlags -contains $flag) {
                    $skipValue = $true
                }
            }
            continue
        }

        $children = @($todoistCommands[$path])
        if ($children -contains $token) {
            $candidatePath = if ($path) { "$path $token" } else { $token }
            if ($todoistAliases.ContainsKey($candidatePath)) {
                $path = $todoistAliases[$candidatePath]
            } else {
                $path = $candidatePath
            }
        }
    }

    if ($afterDoubleDash) {
        return
    }

    $valueFlag = $null
    $valuePrefix = $wordToComplete
    $completionPrefix = ''
    if ($wordToComplete.Contains('=')) {
        $equals = $wordToComplete.IndexOf('=')
        $valueFlag = $wordToComplete.Substring(0, $equals)
        $valuePrefix = $wordToComplete.Substring($equals + 1)
        $completionPrefix = "$valueFlag="
    } elseif ($tokens.Count -gt 0) {
        $previous = $tokens[$tokens.Count - 1]
        $valueFlags = @($todoistValueFlags['']) + @($todoistValueFlags[$path])
        if ($valueFlags -contains $previous) {
            $valueFlag = $previous
        }
    }

    if ($null -ne $valueFlag) {
        $values = @($todoistValues["$path|$valueFlag"])
        foreach ($value in $values) {
            if ($value.StartsWith($valuePrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
                $completion = "$completionPrefix$value"
                New-Object System.Management.Automation.CompletionResult -ArgumentList @($completion, $completion, 'ParameterValue', $completion)
            }
        }
        return
    }

	$flags = @($todoistGlobalFlags) + @($todoistFlags[$path])
	if ($wordToComplete.StartsWith('-')) {
		$candidates = $flags
	} else {
		$candidates = @($todoistCommands[$path]) + $flags
	}

	foreach ($candidate in ($candidates | Sort-Object -Unique)) {
        if ($candidate.StartsWith($wordToComplete, [System.StringComparison]::OrdinalIgnoreCase)) {
            New-Object System.Management.Automation.CompletionResult -ArgumentList @($candidate, $candidate, 'ParameterValue', $candidate)
        }
    }
}.GetNewClosure()

Register-ArgumentCompleter -Native -CommandName todoist -ScriptBlock $todoistCompleter

Remove-Variable todoistCommands, todoistAliases, todoistGlobalFlags, todoistFlags, todoistValueFlags, todoistValues, todoistCompleter
`
