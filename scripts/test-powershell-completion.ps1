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
        foreach ($alias in @('ls', 'show', 'rm', 'del')) {
            Assert-ContainsCompletion 'todoist task ' $alias
        }
        Assert-ContainsCompletion 'todoist task ' '--json'
        Assert-ContainsCompletion 'todoist task list --j' '--json'
        Assert-ContainsCompletion 'todoist task add --pr' '--priority'
        Assert-OmitsCompletion 'todoist filter list --n' '--name'
        Assert-ContainsCompletion 'todoist task add --priority p' 'p1'
        Assert-ContainsCompletion 'todoist task add --priority=p' '--priority=p1'
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
