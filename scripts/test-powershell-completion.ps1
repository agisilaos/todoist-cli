$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("todoist-powershell-completion-" + [System.Guid]::NewGuid())
$binary = Join-Path $work 'todoist'

function Get-CompletionText {
    param([Parameter(Mandatory = $true)][string]$Line)

    return @((TabExpansion2 $Line $Line.Length).CompletionMatches | ForEach-Object CompletionText)
}

function Assert-ContainsCompletion {
    param(
        [Parameter(Mandatory = $true)][string]$Line,
        [Parameter(Mandatory = $true)][string]$Expected
    )

    $matches = Get-CompletionText -Line $Line
    if ($matches -notcontains $Expected) {
        throw "Expected '$Expected' for '$Line'; got: $($matches -join ', ')"
    }
}

function Assert-OmitsCompletion {
    param(
        [Parameter(Mandatory = $true)][string]$Line,
        [Parameter(Mandatory = $true)][string]$Unexpected
    )

    $matches = Get-CompletionText -Line $Line
    if ($matches -contains $Unexpected) {
        throw "Did not expect '$Unexpected' for '$Line'; got: $($matches -join ', ')"
    }
}

try {
    New-Item -ItemType Directory -Path $work | Out-Null
    Push-Location $root
    try {
        go build -o $binary ./cmd/todoist
        & $binary completion powershell | Out-String | Invoke-Expression

        Assert-ContainsCompletion 'todoist pro' 'project'
        Assert-ContainsCompletion 'todoist rev' 'review'
        Assert-ContainsCompletion 'todoist help rev' 'review'
        Assert-ContainsCompletion 'todoist notification l' 'ls'
        Assert-ContainsCompletion 'todoist notification ls --t' '--type'
        Assert-ContainsCompletion 'todoist completion p' 'pwsh'
        Assert-ContainsCompletion 'todoist agent schedule ' 'print'
        foreach ($alias in @('ls', 'show', 'rm', 'del')) {
            Assert-ContainsCompletion 'todoist task ' $alias
        }
        Assert-ContainsCompletion 'todoist task ' '--json'
        Assert-ContainsCompletion 'todoist task list --j' '--json'
        Assert-ContainsCompletion 'todoist task add --pr' '--priority'
        Assert-OmitsCompletion 'todoist filter list --n' '--name'
        Assert-ContainsCompletion 'todoist task add --priority p' 'p1'
        Assert-ContainsCompletion 'todoist task add --priority=p' '--priority=p1'
        # Both partitions contribute suggestions; only value-taking flags skip
        # the next token while resolving command paths.
        Assert-ContainsCompletion 'todoist task add --qu' '--quick'
        Assert-ContainsCompletion 'todoist task add --quick --pr' '--priority'
        Assert-ContainsCompletion 'todoist review --f' '--filter'
        Assert-ContainsCompletion 'todoist stats vacation --o' '--on'
        Assert-ContainsCompletion 'todoist --profile task task add --pr' '--priority'
        Assert-ContainsCompletion "todoist task add --content 'two words' --pr" '--priority'
        Assert-ContainsCompletion 'todoist task show --i' '--id'
        Assert-ContainsCompletion 'todoist task view --task-output-version ' '1'
        Assert-ContainsCompletion 'todoist task view --task-output-version ' '2'
        Assert-ContainsCompletion 'todoist task view --task-output-version=2' '--task-output-version=2'
        Assert-ContainsCompletion 'todoist --task-output-version 2 task list --s' '--sort'
        Assert-ContainsCompletion 'todoist schema --name task_item_v' 'task_item_v2'
        Assert-OmitsCompletion 'todoist task add -- --pr' '--priority'

        Assert-ContainsCompletion 'todoist task list --sort ' 'priority'
        Assert-ContainsCompletion 'todoist task list --preset ' 'next7'
        Assert-ContainsCompletion 'todoist completed --completed-by ' 'due'
        Assert-ContainsCompletion 'todoist project move --visibility ' 'restricted'
        Assert-ContainsCompletion 'todoist agent apply --on-error ' 'continue'
        Assert-ContainsCompletion 'todoist settings update --next-week ' 'monday'
        Assert-OmitsCompletion 'todoist settings update --auto-reminder ' 'true'
    } finally {
        Pop-Location
    }
} finally {
    if (Test-Path -LiteralPath $work) {
        Remove-Item -LiteralPath $work -Recurse -Force
    }
}
