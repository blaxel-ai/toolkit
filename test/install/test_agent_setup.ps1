# Exercise the installer's actual hand-off with a recording CLI. Downloads,
# user PATH changes, browser login and real configuration are not invoked.
# Run: pwsh -NoProfile -File test/install/test_agent_setup.ps1
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "test_setup_policy.ps1")

$source = @()
foreach ($name in @("Forced", "AgentRun")) {
    $node = $ast.Find({
        param($node)
        $node -is [System.Management.Automation.Language.AssignmentStatementAst] -and
            $node.Left.Extent.Text -eq "`$$name"
    }, $true)
    if ($null -eq $node) { throw "Missing assignment: $name" }
    $source += $node.Extent.Text
}
$handoff = $ast.Find({
    param($node)
    $node -is [System.Management.Automation.Language.IfStatementAst] -and
        $node.Clauses[0].Item1.Extent.Text.StartsWith('$SetupAvailable -and (Test-SetupEnabled')
}, $true)
if ($null -eq $handoff) { throw "Missing setup hand-off" }
$runHandoff = [scriptblock]::Create(($source + $handoff.Extent.Text) -join "`n")

function Write-Step {
    param([string]$Kind, [string]$Label, [string]$Detail)
    Write-Host "$Kind $Label $Detail"
}

$keys = $ciMarkers + $agentMarkers + @(
    "BL_INSTALL_SETUP", "BL_INSTALL_SKILLS", "BL_INSTALL_LOGIN", "BL_INSTALL_MCP",
    "BL_INSTALL_TRACKING", "DO_NOT_TRACK", "BL_API_KEY", "BL_CLIENT_CREDENTIALS",
    "BL_INSTALLER", "BL_INSTALLER_SHELL"
)
$saved = @{}
foreach ($key in $keys) { $saved[$key] = [Environment]::GetEnvironmentVariable($key, "Process") }

function Assert-Handoff {
    param(
        [string]$Label, [hashtable]$Env = @{}, [string[]]$Expected = @(),
        [bool]$Interactive = $false, [bool]$SetupAvailable = $true,
        [switch]$SkipSetup, [switch]$SkipSkills, [switch]$HasToken,
        [int]$SetupExit = 0, [int]$LoginExit = 0, [switch]$ThrowSetup, [switch]$ThrowLogin,
        [string]$Retry = "", [string]$Tracking = ""
    )
    foreach ($key in $keys) { [Environment]::SetEnvironmentVariable($key, $null, "Process") }
    foreach ($key in $Env.Keys) { [Environment]::SetEnvironmentVariable($key, $Env[$key], "Process") }
    $calls = [System.Collections.Generic.List[string]]::new()
    $state = @{}
    $BlaxelExe = {
        $calls.Add($args -join " ")
        switch ($args[0]) {
            setup {
                $state.Tracking = $env:DO_NOT_TRACK
                if ($ThrowSetup) { throw "fixture setup failure" }
                $global:LASTEXITCODE = $SetupExit
            }
            token {
                Write-Output "fixture-token-never-display"
                $global:LASTEXITCODE = [int](-not $HasToken)
            }
            login {
                if ($ThrowLogin) { throw "fixture login failure" }
                Write-Output "https://example.invalid/device?code=fixture"
                $global:LASTEXITCODE = $LoginExit
            }
        }
    }
    $output = (& $runHandoff 6>&1 | Out-String)
    if (($calls -join ",") -ne ($Expected -join ",")) { throw "${Label}: unexpected calls: $calls" }
    if ($PSBoundParameters.ContainsKey("Tracking") -and ([string]$state.Tracking -ne $Tracking)) { throw "${Label}: unexpected tracking environment" }
    if ($Retry -and $output -notmatch "next bl $Retry to") { throw "${Label}: missing retry: $output" }
    if (-not $Retry -and $output -match "next bl") { throw "${Label}: unexpected retry: $output" }
    if ($calls.Contains("login") -and -not $ThrowLogin -and $output -notmatch "https://example.invalid/device") { throw "${Label}: login URL missing" }
    if ($output -match "fixture-token-never-display|Successfully logged in|Blaxel is ready") { throw "${Label}: misleading output: $output" }
    if ($global:LASTEXITCODE -ne 0 -and $calls.Count -gt 0) { throw "${Label}: CLI install should remain successful" }
    foreach ($key in @("BL_INSTALLER", "BL_INSTALLER_SHELL", "DO_NOT_TRACK")) {
        if ([Environment]::GetEnvironmentVariable($key, "Process") -ne $Env[$key]) { throw "${Label}: $key leaked" }
    }
    Write-Host "PASS $Label"
}

try {
    Assert-Handoff "no agent, no terminal"
    Assert-Handoff "interactive agent uses setup screens" -Env @{ CLAUDECODE = "1" } -Interactive $true -Expected @("setup")
    Assert-Handoff "agent setup then login" -Env @{ CLAUDECODE = "1" } -Expected @("setup --yes", "token", "login") -Tracking "1"
    Assert-Handoff "CI wins over an agent" -Env @{ CLAUDECODE = "1"; CI = "true" }
    Assert-Handoff "forced setup in CI skips login" -Env @{ CLAUDECODE = "1"; CI = "true"; BL_INSTALL_SETUP = "true" } -Expected @("setup --yes")
    Assert-Handoff "forced setup without an agent skips login" -Env @{ BL_INSTALL_SETUP = "true" } -Expected @("setup --yes")
    Assert-Handoff "setup opt-out" -Env @{ CLAUDECODE = "1"; BL_INSTALL_SETUP = "false" }
    Assert-Handoff "SkipSetup" -Env @{ CLAUDECODE = "1" } -SkipSetup
    Assert-Handoff "skills opt-out" -Env @{ CLAUDECODE = "1"; BL_INSTALL_SKILLS = "false" }
    Assert-Handoff "SkipSkills" -Env @{ CLAUDECODE = "1" } -SkipSkills
    Assert-Handoff "forced setup honors SkipSkills" -Env @{ CLAUDECODE = "1"; BL_INSTALL_SETUP = "true" } -SkipSkills -Expected @("setup --skip-skills --yes", "token", "login")
    Assert-Handoff "login opt-out" -Env @{ CLAUDECODE = "1"; BL_INSTALL_LOGIN = " FALSE " } -Expected @("setup --yes")
    Assert-Handoff "API key skips browser login" -Env @{ CLAUDECODE = "1"; BL_API_KEY = "fixture" } -Expected @("setup --yes")
    Assert-Handoff "client credentials skip browser login" -Env @{ CLAUDECODE = "1"; BL_CLIENT_CREDENTIALS = "fixture" } -Expected @("setup --yes")
    Assert-Handoff "existing login skips browser login" -Env @{ CLAUDECODE = "1" } -HasToken -Expected @("setup --yes", "token")
    Assert-Handoff "explicit tracking" -Env @{ CLAUDECODE = "1"; BL_INSTALL_TRACKING = "true"; BL_INSTALL_LOGIN = "false" } -Expected @("setup --yes")
    Assert-Handoff "existing DO_NOT_TRACK" -Env @{ CLAUDECODE = "1"; DO_NOT_TRACK = "0"; BL_INSTALL_LOGIN = "false" } -Expected @("setup --yes") -Tracking "0"
    Assert-Handoff "setup exit failure" -Env @{ CLAUDECODE = "1" } -SetupExit 1 -Expected @("setup --yes", "token", "login") -Retry setup
    Assert-Handoff "login exit failure" -Env @{ CLAUDECODE = "1" } -LoginExit 1 -Expected @("setup --yes", "token", "login") -Retry login
    Assert-Handoff "setup exception" -Env @{ CLAUDECODE = "1" } -ThrowSetup -Expected @("setup --yes", "token", "login") -Retry setup
    Assert-Handoff "login exception" -Env @{ CLAUDECODE = "1" } -ThrowLogin -Expected @("setup --yes", "token", "login") -Retry login
    Assert-Handoff "release without setup" -Env @{ CLAUDECODE = "1" } -SetupAvailable $false
}
finally {
    foreach ($key in $keys) { [Environment]::SetEnvironmentVariable($key, $saved[$key], "Process") }
}
