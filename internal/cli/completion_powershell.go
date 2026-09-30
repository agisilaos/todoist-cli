package cli

const powerShellCompletionMarker = "# todoist completion (powershell)"

var powerShellCompletion = renderPowerShellInventory(powerShellCompletionTemplate)

const powerShellCompletionTemplate = powerShellCompletionMarker + `
{{powershell-commands}}
$todoistGlobalFlags = @(
    '-h', '--help', '--version', '-q', '--quiet', '--quiet-json', '-v', '--verbose',
    '--accessible', '--json', '--plain', '--ndjson', '--ids-only', '--task-output-version', '--no-color',
    '--no-input', '--timeout', '--config', '--profile', '-n', '--dry-run', '-f',
    '--force', '--fuzzy', '--no-fuzzy', '--progress-jsonl', '--base-url'
)

# Command switches and value-taking flags are disjoint; suggestions combine both.
$todoistSwitchFlags = @{
    'completed' = @('--all', '--wide')
    'upcoming' = @('--wide')
    'add' = @('--strict')
    'auth login' = @('--token-stdin', '--print-env', '--oauth', '--oauth-device', '--read-only', '--no-browser')
    'task list' = @('--all', '--all-projects', '--completed', '--wide')
    'task add' = @('--quick', '--natural')
    'task view' = @('--full')
    'task update' = @('--natural')
    'task move' = @('--yes')
    'task complete' = @('--yes')
    'task delete' = @('--yes')
    'filter add' = @('--favorite')
    'filter update' = @('--favorite', '--unfavorite')
    'filter delete' = @('--yes')
    'project list' = @('--archived', '--all')
    'project collaborators' = @('--all')
    'project add' = @('--favorite')
    'project update' = @('--favorite')
    'project move' = @('--to-personal', '--yes')
    'section list' = @('--all')
    'label list' = @('--all')
    'label add' = @('--favorite')
    'label update' = @('--favorite', '--unfavorite')
    'comment list' = @('--all')
    'reminder delete' = @('--yes')
    'notification list' = @('--unread', '--read')
    'notification read' = @('--all', '--yes')
    'activity' = @('--all')
    'stats vacation' = @('--on', '--off')
    'agent schedule print' = @('--force', '--dry-run', '--cron')
    'agent planner' = @('--set')
    'doctor' = @('--strict')
    'planner' = @('--set')
}

$todoistValueFlags = @{
    '' = @('--timeout', '--config', '--profile', '--progress-jsonl', '--base-url', '--task-output-version')
    'review' = @('--filter', '--out')
    'completed' = @('--completed-by', '--since', '--until', '--project', '--section', '--filter', '--cursor', '--limit')
    'upcoming' = @('--days', '--project', '--label', '--sort', '--truncate-width')
    'inbox add' = @('--content', '--description', '--section', '--label', '--priority', '--due', '--due-date', '--due-datetime', '--due-lang', '--duration', '--duration-unit', '--deadline', '--assignee')
    'add' = @('--content', '--project', '--section', '--label', '--priority', '--due')
    'auth migrate' = @('--credential-store')
    'auth login' = @('--credential-store', '--client-id', '--oauth-authorize-url', '--oauth-token-url', '--oauth-device-url', '--oauth-listen', '--oauth-redirect-uri')
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
    '|--task-output-version' = @('1', '2')
    'schema|--name' = @('review_report', 'ids_only', 'task_item', 'task_list', 'task_item_ndjson', 'task_item_v2', 'task_list_v2', 'error', 'plan', 'plan_preview', 'planner_request')
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
        if ($todoistValues.ContainsKey("$path|$valueFlag")) {
            $values = @($todoistValues["$path|$valueFlag"])
        } elseif ($todoistValues.ContainsKey("|$valueFlag")) {
            $values = @($todoistValues["|$valueFlag"])
        } else {
            return
        }
        foreach ($value in $values) {
            if ($value.StartsWith($valuePrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
                $completion = "$completionPrefix$value"
                New-Object System.Management.Automation.CompletionResult -ArgumentList @($completion, $completion, 'ParameterValue', $completion)
            }
        }
        return
    }

	$flags = @($todoistGlobalFlags) + @($todoistSwitchFlags[$path]) + @($todoistValueFlags[$path])
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

Remove-Variable todoistCommands, todoistAliases, todoistGlobalFlags, todoistSwitchFlags, todoistValueFlags, todoistValues, todoistCompleter
`
