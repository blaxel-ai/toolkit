# Test the installer policy without running downloads or installing the CLI.
$ErrorActionPreference = "Stop"
$installer = Join-Path $PSScriptRoot "../../install.ps1"
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $installer, [ref]$tokens, [ref]$parseErrors
)
if ($parseErrors.Count -gt 0) {
    throw "Installer parse errors: $($parseErrors -join '; ')"
}
$policy = $ast.Find({
    param($node)
    $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
        $node.Name -eq "Test-SkillsInstallationEnabled"
}, $true)
if ($null -eq $policy) { throw "Skills installation policy function was not found" }
. ([scriptblock]::Create($policy.Extent.Text))

$ciMarkers = @("CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE")
$keys = $ciMarkers + "BL_INSTALL_SKILLS"
$saved = @{}
foreach ($key in $keys) {
    $saved[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
}

function Assert-Policy {
    param([bool]$Expected, [string]$Label, [switch]$SkipSkills)
    $actual = Test-SkillsInstallationEnabled -SkipSkills:$SkipSkills
    if ($actual -ne $Expected) {
        throw "${Label}: expected $Expected, got $actual"
    }
    Write-Host "PASS $Label"
}

try {
    foreach ($key in $keys) { [Environment]::SetEnvironmentVariable($key, $null, "Process") }
    Assert-Policy -Expected $true -Label "enabled outside CI"
    Assert-Policy -Expected $false -Label "SkipSkills disables installation" -SkipSkills
    $env:BL_INSTALL_SKILLS = "false"
    Assert-Policy -Expected $false -Label "false disables outside CI"
    $env:BL_INSTALL_SKILLS = " FALSE "
    Assert-Policy -Expected $false -Label "false is case-insensitive and trimmed"
    $env:BL_INSTALL_SKILLS = "other"
    Assert-Policy -Expected $true -Label "unrecognized override follows default policy"
    $env:BL_INSTALL_SKILLS = " TRUE "
    Assert-Policy -Expected $true -Label "case and whitespace are normalized"
    Assert-Policy -Expected $false -Label "SkipSkills overrides true" -SkipSkills

    foreach ($marker in $ciMarkers) {
        [Environment]::SetEnvironmentVariable("BL_INSTALL_SKILLS", $null, "Process")
        [Environment]::SetEnvironmentVariable($marker, "true", "Process")
        Assert-Policy -Expected $false -Label "$marker skips installation by default"
        $env:BL_INSTALL_SKILLS = "true"
        Assert-Policy -Expected $true -Label "true forces installation in $marker"
        Assert-Policy -Expected $false -Label "SkipSkills overrides true in $marker" -SkipSkills
        $env:BL_INSTALL_SKILLS = "false"
        Assert-Policy -Expected $false -Label "false disables installation in $marker"
        [Environment]::SetEnvironmentVariable("BL_INSTALL_SKILLS", $null, "Process")
        [Environment]::SetEnvironmentVariable($marker, "false", "Process")
        Assert-Policy -Expected $false -Label "any nonempty $marker value counts as CI"
        [Environment]::SetEnvironmentVariable($marker, $null, "Process")
    }
}
finally {
    foreach ($key in $keys) {
        [Environment]::SetEnvironmentVariable($key, $saved[$key], "Process")
    }
}
