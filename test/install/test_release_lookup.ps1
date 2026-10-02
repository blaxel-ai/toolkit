# Checks how install.ps1 picks the latest release, with GitHub stubbed out:
# the latest-release redirect, unless it is a preview, then the newest
# stable tag from the API, as install.sh does.
$ErrorActionPreference = "Stop"
$ast = [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot "../../install.ps1"), [ref]$null, [ref]$null)
$unstable = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.AssignmentStatementAst] -and $n.Left.Extent.Text -eq '$Unstable' }, $true)[0]
$latest = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq "Get-LatestVersion" }, $true)[0]
if (-not $unstable -or -not $latest) { throw "install.ps1 no longer defines `$Unstable and Get-LatestVersion" }
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
