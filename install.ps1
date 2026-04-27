<#
.SYNOPSIS
    Installs the Blaxel CLI (blaxel / bl) on Windows.

.DESCRIPTION
    Downloads the latest (or a specified) release of the Blaxel CLI from GitHub,
    extracts it to $env:LOCALAPPDATA\blaxel, creates a bl.exe alias, and adds
    the directory to the user's PATH.

.PARAMETER Version
    The release tag to install (e.g. "v0.1.21"). Defaults to the latest release.

.EXAMPLE
    # Install the latest version:
    powershell -Command "irm https://raw.githubusercontent.com/blaxel-ai/toolkit/main/install.ps1 | iex"

    # Install a specific version:
    .\install.ps1 -Version v0.1.21
#>

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

$Owner = "blaxel-ai"
$Repo  = "toolkit"

# ── Detect architecture ──────────────────────────────────────────────
function Get-BlaxelArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64"   { return "x86_64" }
        "x86"     { return "i386" }
        "ARM64"   { return "arm64" }
        default   {
            # Fallback: check via .NET
            $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
            switch ($arch) {
                "X64"   { return "x86_64" }
                "X86"   { return "i386" }
                "Arm64" { return "arm64" }
                default {
                    Write-Error "Unsupported architecture: $arch"
                    exit 1
                }
            }
        }
    }
}

# ── Resolve version ──────────────────────────────────────────────────
function Get-LatestVersion {
    $url = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
    try {
        $release = Invoke-RestMethod -Uri $url -UseBasicParsing
        return $release.tag_name
    }
    catch {
        Write-Error "Failed to fetch the latest release from GitHub: $_"
        exit 1
    }
}

# ── Main install logic ───────────────────────────────────────────────
$Arch = Get-BlaxelArch

if (-not $Version) {
    Write-Host "blaxel-ai/toolkit: checking GitHub for latest version"
    $Version = Get-LatestVersion
}

Write-Host "blaxel-ai/toolkit: installing version $Version for Windows $Arch"

$ZipName    = "blaxel_Windows_${Arch}.zip"
$DownloadUrl = "https://github.com/$Owner/$Repo/releases/download/$Version/$ZipName"

# Destination directory
$InstallDir = Join-Path $env:LOCALAPPDATA "blaxel"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Download to a temporary file
$TempZip = Join-Path $env:TEMP "blaxel_install_$([System.IO.Path]::GetRandomFileName()).zip"

Write-Host "blaxel-ai/toolkit: downloading from $DownloadUrl"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempZip -UseBasicParsing
}
catch {
    Write-Error "Failed to download $DownloadUrl : $_"
    exit 1
}

# Extract
Write-Host "blaxel-ai/toolkit: extracting to $InstallDir"
try {
    Expand-Archive -Path $TempZip -DestinationPath $InstallDir -Force
}
catch {
    Write-Error "Failed to extract archive: $_"
    exit 1
}
finally {
    Remove-Item -Path $TempZip -Force -ErrorAction SilentlyContinue
}

# Ensure blaxel.exe exists
$BlaxelExe = Join-Path $InstallDir "blaxel.exe"
if (-not (Test-Path $BlaxelExe)) {
    Write-Error "blaxel.exe not found in the extracted archive."
    exit 1
}

# Create bl.exe alias (copy)
$BlExe = Join-Path $InstallDir "bl.exe"
Copy-Item -Path $BlaxelExe -Destination $BlExe -Force

# ── Add to PATH ──────────────────────────────────────────────────────
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -split ";" | Where-Object { $_ -eq $InstallDir }) {
    Write-Host "blaxel-ai/toolkit: $InstallDir is already in your PATH"
}
else {
    [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$UserPath", "User")
    Write-Host "blaxel-ai/toolkit: added $InstallDir to your user PATH"
}

# Also update the current session so the binary is usable immediately
if ($env:Path -split ";" | Where-Object { $_ -eq $InstallDir }) {
    # Already in session PATH
}
else {
    $env:Path = "$InstallDir;$env:Path"
}

# Broadcast WM_SETTINGCHANGE so newly spawned processes (e.g. from Explorer)
# pick up the updated user PATH without requiring a logout.
try {
    if (-not ('NativeMethods.Win32EnvBroadcast' -as [type])) {
        Add-Type -Namespace NativeMethods -Name Win32EnvBroadcast -MemberDefinition @'
[System.Runtime.InteropServices.DllImport("user32.dll", SetLastError = true, CharSet = System.Runtime.InteropServices.CharSet.Auto)]
public static extern System.IntPtr SendMessageTimeout(
    System.IntPtr hWnd, uint Msg, System.UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out System.UIntPtr lpdwResult);
'@ -ErrorAction Stop
    }
    $HWND_BROADCAST = [IntPtr]0xffff
    $WM_SETTINGCHANGE = 0x1A
    [System.UIntPtr]$result = [System.UIntPtr]::Zero
    [void][NativeMethods.Win32EnvBroadcast]::SendMessageTimeout(
        $HWND_BROADCAST, $WM_SETTINGCHANGE, [System.UIntPtr]::Zero, "Environment",
        2, 5000, [ref]$result)
}
catch {
    # Best-effort; PATH is still persisted via SetEnvironmentVariable above.
}

# ── Check optional dependencies ──────────────────────────────────────
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Host ""
    Write-Host "Note: Git was not found on your PATH." -ForegroundColor Yellow
    Write-Host "Some 'bl' commands (like 'bl new') require Git to be installed." -ForegroundColor Yellow
    Write-Host "Install it from: https://git-scm.com/download/win" -ForegroundColor Yellow
}

# ── Done ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "blaxel and bl were installed successfully to $InstallDir"
Write-Host "Installed version: $Version"
Write-Host ""
Write-Host "Open a NEW terminal window for 'bl' to be available on your PATH." -ForegroundColor Cyan
Write-Host "(Already-open terminals will not pick up the updated PATH.)"
Write-Host ""
Write-Host "Then run: bl --help"
