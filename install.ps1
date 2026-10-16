<#
.SYNOPSIS
Installs the AhaSend CLI on Windows.

.DESCRIPTION
Downloads the release binary for this machine, checks it against the
release's checksums.txt, and installs it in
%LOCALAPPDATA%\Programs\AhaSend\bin:

    irm https://ahasend.com/install.ps1 | iex

To pass parameters:

    & ([scriptblock]::Create((irm https://ahasend.com/install.ps1))) -Version v0.4.0

It never changes PATH, and never replaces a file that a package manager
installed. It runs on Windows PowerShell 5.1 and PowerShell 7.

All code is in functions and the last line runs them, so a download that
stops halfway runs nothing. It reports errors with throw and never calls
exit, which would close the PowerShell session that runs it.

.PARAMETER Version
The release to install, such as v0.4.0. The default is the latest release,
or AHASEND_VERSION when it is set.

.PARAMETER InstallDir
The directory to install into. The default is
%LOCALAPPDATA%\Programs\AhaSend\bin, or AHASEND_INSTALL_DIR when it is set.

.PARAMETER Quiet
Print only errors.

.NOTES
AHASEND_RELEASES_URL downloads from a copy of the releases page instead of
https://github.com/AhaSend/ahasend-cli/releases.
#>
param(
    [string]$Version = $env:AHASEND_VERSION,
    [string]$InstallDir = $env:AHASEND_INSTALL_DIR,
    [switch]$Quiet
)

# Get-AhaSendArchitecture returns the name the release assets use for this
# machine's processor. It asks the operating system, so an x64 PowerShell
# that runs under emulation on arm64 still gets arm64.
function Get-AhaSendArchitecture {
    try {
        $architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    } catch {
        # Older .NET Framework versions have no RuntimeInformation
        $architecture = $env:PROCESSOR_ARCHITEW6432
        if (-not $architecture) {
            $architecture = $env:PROCESSOR_ARCHITECTURE
        }
    }
    switch ($architecture) {
        { $_ -in 'X64', 'AMD64' } { return 'amd64' }
        'Arm64' { return 'arm64' }
    }
    throw "This script supports x64 and arm64 Windows, not $architecture."
}

# Get-LatestTag returns the tag of the newest stable release, from the URL that
# <releases>/latest redirects to. It does not use the GitHub API, which has
# rate limits.
function Get-LatestTag {
    param([string]$ReleasesUrl, [string]$StableTag)
    $url = "$ReleasesUrl/latest"
    try {
        $request = [System.Net.WebRequest]::Create($url)
        $request.Method = 'HEAD'
        $request.AllowAutoRedirect = $false
        $request.UserAgent = 'ahasend-install'
        $response = $request.GetResponse()
        $location = $response.Headers['Location']
        $response.Close()
    } catch {
        throw "Could not find the latest release at ${url}: $($_.Exception.GetBaseException().Message)"
    }
    if (-not $location) {
        throw "$url does not redirect to a release."
    }
    $tag = ($location -split '/tag/')[-1]
    if ($tag -cnotmatch $StableTag) {
        throw "The latest release, $tag, is not a stable version. Choose a release with -Version, from $ReleasesUrl"
    }
    return $tag
}

# Get-NpmPrefix returns the folder where npm installs global commands:
# NPM_CONFIG_PREFIX, else what npm says, else npm's default on Windows, or
# $null when there is none
function Get-NpmPrefix {
    if ($env:NPM_CONFIG_PREFIX) {
        return $env:NPM_CONFIG_PREFIX
    }
    # The npm program (npm.cmd on Windows), not npm.ps1, which the default
    # execution policy does not run
    $npm = @(Get-Command npm -CommandType Application -ErrorAction SilentlyContinue)[0]
    if (-not $npm) {
        if ($env:APPDATA) {
            return Join-Path $env:APPDATA 'npm'
        }
        return $null
    }
    # Windows PowerShell 5.1 turns a line npm writes to stderr into an error,
    # which would stop the script under Stop
    $ErrorActionPreference = 'Continue'
    try {
        $prefix = @(& $npm.Source prefix -g 2> $null)[0]
        if ($LASTEXITCODE -eq 0 -and $prefix) {
            return $prefix.Trim()
        }
    } catch {
        Write-Verbose "npm prefix -g failed: $_"
    }
    # npm's default global folder on Windows
    if ($env:APPDATA) {
        return Join-Path $env:APPDATA 'npm'
    }
    return $null
}

# Test-Under returns whether Path is inside Folder
function Test-Under {
    param([string]$Path, [string]$Folder)
    if (-not $Folder) {
        return $false
    }
    $folderPrefix = $Folder.Replace('\', '/').TrimEnd('/') + '/'
    return $Path.Replace('\', '/').StartsWith($folderPrefix, [System.StringComparison]::OrdinalIgnoreCase)
}

# Get-PackageManager returns the name of the package manager that owns Path,
# with the command that upgrades it, or $null when none does. A package
# manager owns every file in the folders it puts commands in.
function Get-PackageManager {
    param([string]$Path, [string]$NpmPrefix)
    $normalPath = $Path.Replace('\', '/')
    $scoopShims = foreach ($root in $env:SCOOP, $env:SCOOP_GLOBAL) {
        if ($root) {
            Join-Path $root 'shims'
        }
    }
    if ($normalPath -match '/scoop/shims/' -or @($scoopShims | Where-Object { Test-Under $Path $_ }).Count -gt 0) {
        return @{ Name = 'Scoop'; Upgrade = 'scoop update ahasend' }
    }
    if ($normalPath -match '/WinGet/Links/') {
        return @{ Name = 'WinGet'; Upgrade = 'winget upgrade AhaSend.CLI' }
    }
    $chocolateyBin = $null
    if ($env:ChocolateyInstall) {
        $chocolateyBin = Join-Path $env:ChocolateyInstall 'bin'
    }
    if ($normalPath -match '/chocolatey/bin/' -or (Test-Under $Path $chocolateyBin)) {
        return @{ Name = 'Chocolatey'; Upgrade = 'choco upgrade ahasend' }
    }
    if (Test-Under $Path $NpmPrefix) {
        return @{ Name = 'npm'; Upgrade = 'npm install -g @ahasend/cli' }
    }
    return $null
}

# Get-FullPath returns Path as a full path, relative to the current location
function Get-FullPath {
    param([string]$Path)
    $full = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
    return $full.TrimEnd('\', '/')
}

# Find-OtherCopy returns the first ahasend on PATH that comes before
# InstallDir, or $null, and whether InstallDir is on PATH
function Find-OtherCopy {
    param([string]$InstallDir)
    $other = $null
    foreach ($dir in ($env:PATH -split [System.IO.Path]::PathSeparator)) {
        $dir = $dir.Trim().Trim('"')
        if (-not $dir) {
            continue
        }
        try {
            $full = Get-FullPath $dir
        } catch {
            continue
        }
        if ($full -eq $InstallDir) {
            return @{ Other = $other; OnPath = $true }
        }
        if (-not $other) {
            foreach ($name in 'ahasend.exe', 'ahasend.cmd', 'ahasend.ps1') {
                $candidate = Join-Path $full $name
                if (Test-Path -LiteralPath $candidate -PathType Leaf) {
                    $other = $candidate
                    break
                }
            }
        }
    }
    return @{ Other = $other; OnPath = $false }
}

# Get-Download saves Url in File
function Get-Download {
    param([string]$Url, [string]$File)
    try {
        Invoke-WebRequest -Uri $Url -OutFile $File -UseBasicParsing
    } catch {
        throw "Could not download ${Url}: $($_.Exception.GetBaseException().Message)"
    }
}

function Install-AhaSend {
    param([string]$Version, [string]$InstallDir, [bool]$Quiet, [string[]]$Given)
    Set-StrictMode -Version 2.0
    $ErrorActionPreference = 'Stop'
    # An empty value, such as from an unset variable, would otherwise mean
    # the latest release or the default directory
    foreach ($name in 'Version', 'InstallDir') {
        if ($Given -contains $name -and -not (Get-Variable -Name $name -ValueOnly)) {
            throw "-$name needs a value."
        }
    }
    # The progress bar makes downloads slow in Windows PowerShell
    $ProgressPreference = 'SilentlyContinue'
    # Older Windows PowerShell setups do not offer TLS 1.2, which GitHub needs
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor
        [System.Net.SecurityProtocolType]::Tls12

    $stableTag = '^v[0-9]+\.[0-9]+\.[0-9]+\z'
    $anyTag = '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?\z'
    $releasesUrl = $env:AHASEND_RELEASES_URL
    if (-not $releasesUrl) {
        $releasesUrl = 'https://github.com/AhaSend/ahasend-cli/releases'
    }
    $releasesUrl = $releasesUrl.TrimEnd('/')

    $architecture = Get-AhaSendArchitecture
    if ($Version) {
        if ($Version -cnotmatch $anyTag) {
            throw "$Version is not a release tag; tags look like v0.4.0."
        }
        $tag = $Version
    } else {
        $tag = Get-LatestTag -ReleasesUrl $releasesUrl -StableTag $stableTag
    }

    if (-not $InstallDir) {
        if (-not $env:LOCALAPPDATA) {
            throw 'LOCALAPPDATA is not set; choose a directory with -InstallDir.'
        }
        $InstallDir = Join-Path (Join-Path (Join-Path $env:LOCALAPPDATA 'Programs') 'AhaSend') 'bin'
    }
    $InstallDir = Get-FullPath $InstallDir
    $target = Join-Path $InstallDir 'ahasend.exe'
    $npmPrefix = Get-NpmPrefix
    $manager = Get-PackageManager -Path $target -NpmPrefix $npmPrefix
    if ($manager) {
        throw "$($manager.Name) owns $target, and this script does not replace it. Upgrade it with: $($manager.Upgrade)"
    }
    if (Test-Path -LiteralPath $target -PathType Container) {
        throw "$target is a directory."
    }
    try {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    } catch {
        throw "Could not create ${InstallDir}: $($_.Exception.Message) Choose another directory with -InstallDir."
    }

    $asset = "ahasend-windows-$architecture.exe"
    $base = "$releasesUrl/download/$tag"
    if (-not $Quiet) {
        Write-Host "Installing ahasend $tag (windows/$architecture) into $InstallDir"
    }
    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ('ahasend-' + [System.IO.Path]::GetRandomFileName())
    $tempExe = Join-Path $InstallDir ('.ahasend-' + [System.IO.Path]::GetRandomFileName() + '.exe')
    try {
        # Made first, to check that the directory can be written to
        try {
            [System.IO.File]::Create($tempExe).Dispose()
        } catch {
            $tempExe = $null
            throw "Cannot write to $InstallDir. Choose a directory you can write to with -InstallDir."
        }

        New-Item -ItemType Directory -Path $tempDir | Out-Null
        $checksums = Join-Path $tempDir 'checksums.txt'
        Get-Download -Url "$base/checksums.txt" -File $checksums
        $expected = $null
        foreach ($line in Get-Content -LiteralPath $checksums) {
            if ($line -match '^([0-9a-fA-F]{64})\s+\*?(.+)$' -and $Matches[2] -eq $asset) {
                $expected = $Matches[1]
                break
            }
        }
        if (-not $expected) {
            throw "$base/checksums.txt has no line for $asset."
        }
        $download = Join-Path $tempDir $asset
        Get-Download -Url "$base/$asset" -File $download
        $actual = (Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash
        if ($actual -ne $expected) {
            throw "The SHA-256 of $asset does not match checksums.txt, so the download is damaged. Expected $expected, got $actual."
        }

        Copy-Item -LiteralPath $download -Destination $tempExe -Force
        try {
            $versionOutput = & $tempExe --version
        } catch {
            throw "The downloaded binary does not run: $($_.Exception.GetBaseException().Message)"
        }
        if ($LASTEXITCODE -ne 0) {
            throw "The downloaded binary does not run: it exited with $LASTEXITCODE."
        }
        if (Test-Path -LiteralPath $target) {
            # [NullString] passes null, where $null would pass an empty string
            [System.IO.File]::Replace($tempExe, $target, [NullString]::Value)
        } else {
            [System.IO.File]::Move($tempExe, $target)
        }
        $tempExe = $null
    } finally {
        if ($tempExe -and (Test-Path -LiteralPath $tempExe)) {
            Remove-Item -LiteralPath $tempExe -Force
        }
        if (Test-Path -LiteralPath $tempDir) {
            Remove-Item -LiteralPath $tempDir -Recurse -Force
        }
    }
    if ($Quiet) {
        return
    }
    Write-Host "Installed $(@($versionOutput)[0]) at $target" -ForegroundColor Green

    $found = Find-OtherCopy -InstallDir $InstallDir
    if ($found.Other) {
        $otherManager = Get-PackageManager -Path $found.Other -NpmPrefix $npmPrefix
        $message = "$($found.Other) comes first on PATH, so the ahasend command runs it, not $target."
        if ($otherManager) {
            $message += " $($otherManager.Name) installed it; upgrade it with: $($otherManager.Upgrade)"
        } else {
            $message += " Remove it, or put $InstallDir before $(Split-Path -Parent $found.Other) on PATH."
        }
        Write-Warning $message
    }
    if (-not $found.OnPath) {
        # The command reads and writes the user Path in the registry without
        # expanding names such as %USERPROFILE% in it, and keeps it an
        # expandable value; [Environment]::GetEnvironmentVariable and
        # SetEnvironmentVariable would expand them and store a plain string.
        # Clearing an unused variable then tells Windows that the
        # environment changed, so new terminals see the new Path.
        $quotedDir = $InstallDir.Replace("'", "''")
        Write-Host ''
        Write-Host "$InstallDir is not on PATH. To add it for your user, run:"
        Write-Host ("  `$key = 'HKCU:\Environment'; `$path = (Get-Item `$key).GetValue('Path', '', 'DoNotExpandEnvironmentNames'); " +
            "Set-ItemProperty `$key Path ((@(`$path, '$quotedDir') -ne '') -join ';') -Type ExpandString; " +
            "[Environment]::SetEnvironmentVariable('AHASEND_PATH_CHANGED', `$null, 'User')")
        Write-Host 'Then open a new terminal.'
    }
}

# In braces, so a download cut off anywhere in this line is a parse error and
# runs nothing; an if block keeps the script's own $PSBoundParameters
if ($true) { Install-AhaSend -Version $Version -InstallDir $InstallDir -Quiet $Quiet.IsPresent -Given @($PSBoundParameters.Keys) }
