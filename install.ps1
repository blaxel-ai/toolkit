<#
.SYNOPSIS
    Installs the Blaxel CLI (blaxel and bl) on Windows.

.DESCRIPTION
    Downloads the latest (or the given) release of the Blaxel CLI, verifies it
    against the release checksums, installs it to $env:LOCALAPPDATA\blaxel and
    adds that folder to your PATH. It then runs bl setup, which shows the coding
    agents it found and sets up Blaxel for them, then logs you in.

.PARAMETER Version
    The release tag to install (e.g. "v0.1.21"). Defaults to the latest release.

.PARAMETER InstallDir
    The folder to install bl.exe and blaxel.exe into. Defaults to
    $env:LOCALAPPDATA\blaxel; bl upgrade passes the folder it runs from.

.PARAMETER SkipSkills
    Leave your coding agents alone: bl setup is only suggested, unless
    BL_INSTALL_SETUP=true runs it without the skills. BL_INSTALL_SKILLS=false does the same.

.PARAMETER SkipSetup
    Do not run bl setup. BL_INSTALL_SETUP=false does the same; =true runs it
    with the defaults even without a terminal or in CI.

.EXAMPLE
    irm https://blaxel.ai/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -Version v0.1.21
#>

param(
    [string]$Version = "",
    [string]$InstallDir = "",
    [switch]$SkipSkills,
    [switch]$SkipSetup
)

# Under `irm | iex` this runs in the user's own PowerShell session: everything
# happens in a child scope, so their preferences and variables stay as they
# were, and nothing calls exit, which would close their window. Run as a file,
# the exit code still reports a failure.
& {
    param([string]$Version, [string]$InstallDir, [switch]$SkipSkills, [switch]$SkipSetup)

    try {
        $ErrorActionPreference = "Stop"
        # Windows PowerShell draws download progress slowly; the steps below say enough.
        $ProgressPreference = "SilentlyContinue"
        # Windows PowerShell 5.1 can default to TLS versions GitHub no longer accepts.
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

        $Owner = "blaxel-ai"
        $Repo = "toolkit"
        $Releases = "https://github.com/$Owner/$Repo/releases"

        # ── Output: styled where the console takes escape codes, plain elsewhere ──
        $Styled = $Host.UI.SupportsVirtualTerminal -and -not [Console]::IsOutputRedirected -and -not $env:NO_COLOR
        $Esc = [char]27
        function Write-Step {
            param([string]$Kind, [string]$Label, [string]$Detail)
            $marker = @{ ok = "+"; fail = "x"; next = ">"; note = "!" }[$Kind]
            if ($Styled) {
                $glyph = @{ ok = [char]0x2713; fail = [char]0x2717; next = [char]0x203A; note = "!" }[$Kind]
                $color = @{ ok = "38;5;78"; fail = "38;5;203"; next = "38;5;208"; note = "38;5;214" }[$Kind]
                Write-Host ("  $Esc[${color}m$glyph$Esc[0m {0,-14} $Esc[38;5;245m{1}$Esc[0m" -f $Label, $Detail)
            }
            else {
                Write-Host ("  {0} {1,-14} {2}" -f $marker, $Label, $Detail)
            }
        }
        # Stop-Install reports a failure and stops the install (see the note above).
        function Stop-Install {
            param([string]$Label, [string]$Detail)
            Write-Step fail $Label $Detail
            throw [System.OperationCanceledException]::new("blaxel-install-stopped")
        }

        # bl setup runs by default outside CI. BL_INSTALL_SETUP=true (or the previous
        # BL_INSTALL_SKILLS=true) forces it, BL_INSTALL_SETUP=false or -SkipSetup disables it.
        function Test-SetupEnabled {
            param([switch]$SkipSetup, [switch]$SkipSkills)

            if ($SkipSetup) { return $false }

            $override = ([string]$env:BL_INSTALL_SETUP).Trim()
            if ($override -eq "false") { return $false }
            if ($override -eq "true") { return $true }
            # Skipping the skills leaves the agents alone, so setup is only suggested.
            if ($SkipSkills -or (([string]$env:BL_INSTALL_SKILLS).Trim() -eq "false")) { return $false }
            if (([string]$env:BL_INSTALL_SKILLS).Trim() -eq "true") { return $true }

            foreach ($name in @("CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE")) {
                if (-not [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($name, "Process"))) {
                    return $false
                }
            }
            return $true
        }

        function Get-BlaxelArch {
            switch ($env:PROCESSOR_ARCHITECTURE) {
                "AMD64" { return "x86_64" }
                "x86" { return "i386" }
                "ARM64" { return "arm64" }
            }
            switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
                "X64" { return "x86_64" }
                "X86" { return "i386" }
                "Arm64" { return "arm64" }
            }
            Stop-Install "Platform" "$env:PROCESSOR_ARCHITECTURE is not supported (x86_64, arm64 and i386 are)"
        }

        # The newest stable release tag, as install.sh picks it: the redirect
        # GitHub serves for the latest release, or, when that is a preview or
        # the redirect fails, the newest stable tag from the GitHub API.
        $Unstable = "preview|alpha|beta|rc|dev|pre|snapshot|nightly|canary|experimental|unstable"
        function Get-RedirectTag {
            $location = $null
            try {
                $request = [System.Net.WebRequest]::Create("$Releases/latest")
                $request.Method = "HEAD"
                $request.AllowAutoRedirect = $false
                $response = $request.GetResponse()
                try { $location = $response.Headers["Location"] } finally { $response.Close() }
            }
            catch [System.Net.WebException] {
                # Some runtimes raise on the 302 when redirects are off; it still has the header.
                $response = $_.Exception.Response
                if ($response) {
                    try { $location = $response.Headers["Location"] } finally { $response.Close() }
                }
            }
            catch { }
            if ($location -match "/tag/([^/]+)$") { return $Matches[1] }
            return $null
        }
        # Release tags, newest first. The API is rate limited, so it is only the fallback.
        function Get-ApiTags {
            try { return @(Invoke-RestMethod -Uri "https://api.github.com/repos/$Owner/$Repo/releases" -UseBasicParsing | ForEach-Object { $_.tag_name }) }
            catch { return @() }
        }
        function Get-LatestVersion {
            $tag = Get-RedirectTag
            if ($tag -and $tag -notmatch $Unstable) { return $tag }
            $tag = @(Get-ApiTags | Where-Object { $_ -and $_ -notmatch $Unstable })[0]
            if ($tag) { return $tag }
            Stop-Install "Blaxel CLI" "could not find the latest release; pass -Version (see $Releases)"
        }

        # ── Download, verify and install ─────────────────────────────────────
        $Arch = Get-BlaxelArch
        if (-not $Version -or $Version -eq "latest") { $Version = Get-LatestVersion }
        if (-not $Version.StartsWith("v")) { $Version = "v$Version" }

        $ZipName = "blaxel_Windows_${Arch}.zip"
        $Temp = Join-Path ([System.IO.Path]::GetTempPath()) "blaxel-install-$([System.IO.Path]::GetRandomFileName())"
        New-Item -ItemType Directory -Path $Temp -Force | Out-Null
        if (-not $InstallDir) { $InstallDir = Join-Path $env:LOCALAPPDATA "blaxel" }
        try {
            $Zip = Join-Path $Temp $ZipName
            try {
                Invoke-WebRequest -Uri "$Releases/download/$Version/$ZipName" -OutFile $Zip -UseBasicParsing
            }
            catch {
                Stop-Install "Blaxel CLI" "could not download $Version for Windows $Arch"
            }

            $Checksums = Join-Path $Temp "checksums.txt"
            try {
                Invoke-WebRequest -Uri "$Releases/download/$Version/blaxel_$($Version.TrimStart('v'))_checksums.txt" -OutFile $Checksums -UseBasicParsing
            }
            catch {
                Stop-Install "Checksum" "could not download the checksums of $Version"
            }
            $line = Get-Content $Checksums | Where-Object { $_ -match "\s$([regex]::Escape($ZipName))$" } | Select-Object -First 1
            if (-not $line) { Stop-Install "Checksum" "$ZipName is not listed in the release checksums" }
            # .NET rather than Get-FileHash and Expand-Archive: those come from
            # script modules, which PowerShell 7 can't load under a Restricted
            # execution policy and Windows PowerShell can miss when started from
            # PowerShell 7.
            $Sha256 = [System.Security.Cryptography.SHA256]::Create()
            $Stream = [System.IO.File]::OpenRead($Zip)
            try { $Hash = [BitConverter]::ToString($Sha256.ComputeHash($Stream)) -replace "-", "" }
            finally { $Stream.Dispose(); $Sha256.Dispose() }
            # Hashes compare without regard to case.
            if ($Hash -ne ($line -split "\s+")[0]) {
                Stop-Install "Checksum" "the download does not match the release checksums; try again"
            }

            Add-Type -AssemblyName System.IO.Compression.FileSystem
            [System.IO.Compression.ZipFile]::ExtractToDirectory($Zip, (Join-Path $Temp "release"))
            $Extracted = Join-Path $Temp "release\blaxel.exe"
            if (-not (Test-Path $Extracted)) { Stop-Install "Blaxel CLI" "blaxel.exe is missing from the release archive" }
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
            $BlaxelExe = Join-Path $InstallDir "blaxel.exe"
            # Windows can't overwrite a running program (bl upgrade itself, or bl
            # in another terminal), but it can rename one: move it aside and delete
            # it now, or on the next install once it has exited.
            Get-ChildItem -LiteralPath $InstallDir -File | Where-Object { $_.Name -like "bl.exe.*.old" -or $_.Name -like "blaxel.exe.*.old" } |
                Remove-Item -Force -ErrorAction SilentlyContinue
            foreach ($Target in $BlaxelExe, (Join-Path $InstallDir "bl.exe")) {
                $Aside = $null
                if (Test-Path -LiteralPath $Target) {
                    $Aside = "$Target.$([System.IO.Path]::GetRandomFileName()).old"
                    Move-Item -LiteralPath $Target -Destination $Aside -Force
                }
                try {
                    Copy-Item -LiteralPath $Extracted -Destination $Target -Force
                }
                catch {
                    if ($Aside) { Move-Item -LiteralPath $Aside -Destination $Target -Force -ErrorAction SilentlyContinue }
                    throw
                }
                if ($Aside) { Remove-Item -LiteralPath $Aside -Force -ErrorAction SilentlyContinue }
            }
        }
        finally {
            Remove-Item -Path $Temp -Recurse -Force -ErrorAction SilentlyContinue
        }
        # The separator as a code: Windows PowerShell reads a file without a BOM as ANSI.
        $dot = [char]0x00B7
        Write-Step ok "Blaxel CLI" "$Version $dot $InstallDir\bl.exe $dot verified"

        # ── Add to PATH ──────────────────────────────────────────────────────
        $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if (-not ($UserPath -split ";" | Where-Object { $_ -eq $InstallDir })) {
            [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$UserPath", "User")
        }
        # This session can use bl right away.
        if (-not ($env:Path -split ";" | Where-Object { $_ -eq $InstallDir })) {
            $env:Path = "$InstallDir;$env:Path"
        }
        # Tell running programs (such as Explorer) that PATH changed, so new terminals see it.
        try {
            if (-not ('NativeMethods.Win32EnvBroadcast' -as [type])) {
                Add-Type -Namespace NativeMethods -Name Win32EnvBroadcast -MemberDefinition @'
[System.Runtime.InteropServices.DllImport("user32.dll", SetLastError = true, CharSet = System.Runtime.InteropServices.CharSet.Auto)]
public static extern System.IntPtr SendMessageTimeout(
    System.IntPtr hWnd, uint Msg, System.UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out System.UIntPtr lpdwResult);
'@ -ErrorAction Stop
            }
            [System.UIntPtr]$result = [System.UIntPtr]::Zero
            [void][NativeMethods.Win32EnvBroadcast]::SendMessageTimeout(
                [IntPtr]0xffff, 0x1A, [System.UIntPtr]::Zero, "Environment", 2, 5000, [ref]$result)
        }
        catch {
            # Best-effort; PATH is still saved for new sessions.
        }

        # Some bl commands (like bl new) clone templates with Git.
        if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
            Write-Step note "Git" "Git was not found; bl new needs it: https://git-scm.com/download/win"
        }

        # ── Hand-off to bl setup ─────────────────────────────────────────────
        $SetupAvailable = $false
        try {
            & $BlaxelExe setup --help *> $null
            $SetupAvailable = ($LASTEXITCODE -eq 0)
        }
        catch { }

        $Interactive = [Environment]::UserInteractive -and -not [Console]::IsInputRedirected -and -not [Console]::IsOutputRedirected
        $Forced = (([string]$env:BL_INSTALL_SETUP).Trim() -eq "true") -or (([string]$env:BL_INSTALL_SKILLS).Trim() -eq "true")
        if ($SetupAvailable -and (Test-SetupEnabled -SkipSetup:$SkipSetup -SkipSkills:$SkipSkills) -and ($Interactive -or $Forced)) {
            $SetupArgs = @("setup")
            if ($SkipSkills) { $SetupArgs += "--skip-skills" }
            if (-not $Interactive) { $SetupArgs += "--yes" }
            # bl setup shows the shell and what to run next itself, problems included.
            $env:BL_INSTALLER = "1"
            $env:BL_INSTALLER_SHELL = "bl on PATH"
            try {
                & $BlaxelExe @SetupArgs
            }
            catch {
                Write-Step next "bl setup" "to finish setting up"
            }
            finally {
                Remove-Item Env:BL_INSTALLER, Env:BL_INSTALLER_SHELL -ErrorAction SilentlyContinue
            }
            # bl is installed: setup has shown its own problems, and leaves the
            # install a success, as install.sh does.
            $global:LASTEXITCODE = 0
            return
        }

        Write-Step ok "Shell" "bl on PATH (open a new terminal elsewhere)"
        Write-Host ""
        # Skipping bl setup on purpose needs no reminder.
        if ($SkipSetup -or ([string]$env:BL_INSTALL_SETUP).Trim() -eq "false") {
        }
        elseif ($SetupAvailable) {
            Write-Step next "bl setup" "set up your coding agents and log in"
        }
        else {
            Write-Step next "bl login" "log in to Blaxel"
        }
        $global:LASTEXITCODE = 0
    }
    catch {
        if ($_.Exception -isnot [System.OperationCanceledException]) {
            Write-Host "  Blaxel install failed: $($_.Exception.Message)"
        }
        $global:LASTEXITCODE = 1
    }
} -Version $Version -InstallDir $InstallDir -SkipSkills:$SkipSkills -SkipSetup:$SkipSetup

if ($MyInvocation.MyCommand.CommandType -eq "ExternalScript") { exit $global:LASTEXITCODE }
