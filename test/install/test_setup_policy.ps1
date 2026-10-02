# Test when install.ps1 runs `bl setup`, without running downloads or installing the CLI.
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
        $node.Name -eq "Test-SetupEnabled"
}, $true)
if ($null -eq $policy) { throw "Setup policy function was not found" }
. ([scriptblock]::Create($policy.Extent.Text))

$ciMarkers = @("CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE")
$keys = $ciMarkers + @("BL_INSTALL_SETUP", "BL_INSTALL_SKILLS")
$saved = @{}
foreach ($key in $keys) {
    $saved[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
}

function Assert-Policy {
    param([bool]$Expected, [string]$Label, [switch]$SkipSetup)
    $actual = Test-SetupEnabled -SkipSetup:$SkipSetup
    if ($actual -ne $Expected) {
        throw "${Label}: expected $Expected, got $actual"
    }
    Write-Host "PASS $Label"
}

try {
    foreach ($key in $keys) { [Environment]::SetEnvironmentVariable($key, $null, "Process") }
    Assert-Policy -Expected $true -Label "enabled outside CI"
    Assert-Policy -Expected $false -Label "SkipSetup disables setup" -SkipSetup
    $env:BL_INSTALL_SETUP = " FALSE "
    Assert-Policy -Expected $false -Label "false disables setup, case-insensitive and trimmed"
    $env:BL_INSTALL_SETUP = "other"
    Assert-Policy -Expected $true -Label "unrecognized override follows default policy"
    [Environment]::SetEnvironmentVariable("BL_INSTALL_SETUP", $null, "Process")
    $env:BL_INSTALL_SKILLS = "false"
    Assert-Policy -Expected $true -Label "BL_INSTALL_SKILLS=false still runs setup (setup skips the skills)"
    [Environment]::SetEnvironmentVariable("BL_INSTALL_SKILLS", $null, "Process")

    foreach ($marker in $ciMarkers) {
        [Environment]::SetEnvironmentVariable($marker, "true", "Process")
        Assert-Policy -Expected $false -Label "$marker skips setup by default"
        $env:BL_INSTALL_SETUP = "true"
        Assert-Policy -Expected $true -Label "BL_INSTALL_SETUP=true forces setup in $marker"
        Assert-Policy -Expected $false -Label "SkipSetup overrides true in $marker" -SkipSetup
        [Environment]::SetEnvironmentVariable("BL_INSTALL_SETUP", $null, "Process")
        $env:BL_INSTALL_SKILLS = "true"
        Assert-Policy -Expected $true -Label "BL_INSTALL_SKILLS=true forces setup in $marker"
        $env:BL_INSTALL_SETUP = "false"
        Assert-Policy -Expected $false -Label "BL_INSTALL_SETUP=false wins over BL_INSTALL_SKILLS=true in $marker"
        [Environment]::SetEnvironmentVariable("BL_INSTALL_SETUP", $null, "Process")
        [Environment]::SetEnvironmentVariable("BL_INSTALL_SKILLS", $null, "Process")
        [Environment]::SetEnvironmentVariable($marker, $null, "Process")
    }
}
finally {
    foreach ($key in $keys) {
        [Environment]::SetEnvironmentVariable($key, $saved[$key], "Process")
    }
}
