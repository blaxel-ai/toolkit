# Checks how install.ps1 picks the latest release, with GitHub stubbed out:
# the latest-release redirect, unless it is a preview, then the newest
# stable tag from the API, as install.sh does. -Live also reads the real
# redirect, which must work without the rate-limited API.
param([switch]$Live)
$ErrorActionPreference = "Stop"
$ast = [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot "../../install.ps1"), [ref]$null, [ref]$null)
$unstable = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.AssignmentStatementAst] -and $n.Left.Extent.Text -eq '$Unstable' }, $true)[0]
$latest = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq "Get-LatestVersion" }, $true)[0]
if (-not $unstable -or -not $latest) { throw "install.ps1 no longer defines `$Unstable and Get-LatestVersion" }
$redirectTag = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq "Get-RedirectTag" }, $true)[0]
Invoke-Expression $unstable.Extent.Text
Invoke-Expression $latest.Extent.Text
function Stop-Install { param($Label, $Detail) throw "stopped: $Detail" }

function Assert-Latest {
    param([string]$Label, $Redirect, [string[]]$Tags, [string]$Expected)
    $script:redirect, $script:tags = $Redirect, $Tags
    function Get-RedirectTag { $script:redirect }
    function Get-ApiTags { $script:tags }
    $actual = try { Get-LatestVersion } catch { "$_" }
    if ($actual -ne $Expected) { throw "${Label}: expected $Expected, got $actual" }
    Write-Host "PASS $Label"
}

Assert-Latest "a stable latest release" "v0.1.119" @() "v0.1.119"
Assert-Latest "a preview latest release falls back to the newest stable tag" "v0.2.0-preview" @("v0.2.0-preview", "v0.1.119-rc1", "v0.1.118") "v0.1.118"
Assert-Latest "no redirect uses the API" $null @("v0.1.118", "v0.1.117") "v0.1.118"
Assert-Latest "nothing found stops the install" $null @("v0.2.0-beta") "stopped: could not find the latest release; pass -Version (see )"

if ($Live) {
    Invoke-Expression $redirectTag.Extent.Text
    $Releases = "https://github.com/blaxel-ai/toolkit/releases"
    $tag = Get-RedirectTag
    if ($tag -notmatch '^v\d+\.\d+\.\d+') { throw "the latest-release redirect gave '$tag' in PowerShell $($PSVersionTable.PSVersion)" }
    Write-Host "PASS the real redirect names $tag in PowerShell $($PSVersionTable.PSVersion)"
}
