# Tests the WinGet manifests that scripts/winget-manifest.sh writes into
# <manifests> for the fake release v99.0.0, with AHASEND_RELEASES_URL set to
# a fake release's URL, such as http://127.0.0.1:8484. It serves the staged
# assets of v99.0.0 in <release>\v99.0.0 at that URL with the fake-release
# server:
#
#   test/install/winget.ps1 -Manifests <manifests> -Release <release> -Server <fake-release.exe>
#
# It installs WinGet when the machine lacks it or has a version older than
# 1.12, which reads the manifests' schema; enables local manifests; runs
# winget validate; installs the manifests; and checks ahasend --version in a
# new shell. It runs as an administrator.
param(
    [Parameter(Mandatory)] [string] $Manifests,
    [Parameter(Mandatory)] [string] $Release,
    [Parameter(Mandatory)] [string] $Server
)
$ErrorActionPreference = 'Stop'
if (-not $env:AHASEND_RELEASES_URL) {
    throw 'set AHASEND_RELEASES_URL to the URL the manifests download from'
}

# wingetVersion returns the version of the winget on PATH, or $null
function wingetVersion {
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        return $null
    }
    $text = winget --version
    if ($text -match '^v?(\d+\.\d+)') {
        return [version]$Matches[1]
    }
    return $null
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
$serverArgs = @('-url', "$tmp\url", '-latest', "$tmp\latest", '-addr', ($env:AHASEND_RELEASES_URL -replace '^http://', ''), $Release)
$process = Start-Process -FilePath $Server -ArgumentList $serverArgs -RedirectStandardError "$tmp\server.log" -NoNewWindow -PassThru
try {
    $tries = 0
    while (-not (Test-Path "$tmp\url")) {
        $tries++
        if ($tries -gt 100) {
            Get-Content "$tmp\server.log"
            throw 'the fake release server did not start'
        }
        Start-Sleep -Milliseconds 100
    }

    $version = wingetVersion
    if (-not $version -or $version -lt [version]'1.12') {
        Write-Host "Installing WinGet (found $version)"
        Install-Module Microsoft.WinGet.Client -Force -Scope CurrentUser -Repository PSGallery
        Repair-WinGetPackageManager -AllUsers -Latest -Force
        $env:Path += ";$env:LOCALAPPDATA\Microsoft\WindowsApps"
    }
    winget --version

    winget settings --enable LocalManifestFiles
    if ($LASTEXITCODE) {
        throw "winget settings failed with exit code $LASTEXITCODE"
    }

    Write-Host 'Validating the manifests'
    winget validate --manifest $Manifests
    if ($LASTEXITCODE) {
        throw "winget validate failed with exit code $LASTEXITCODE"
    }

    Write-Host 'Installing the manifests'
    winget install --manifest $Manifests --accept-package-agreements --accept-source-agreements --disable-interactivity
    if ($LASTEXITCODE) {
        Get-Content "$tmp\server.log"
        throw "winget install failed with exit code $LASTEXITCODE"
    }

    # A new shell, with the PATH a new session gets, which holds WinGet's
    # Links folder, and a new home folder, so nothing reads the user's own
    # files. The PATH is read before LOCALAPPDATA changes, because it may
    # name that folder.
    Write-Host 'Checking ahasend --version in a new shell'
    New-Item -ItemType Directory -Path "$tmp\home" | Out-Null
    $saved = $env:Path, $env:HOME, $env:USERPROFILE, $env:LOCALAPPDATA
    $env:Path = [Environment]::GetEnvironmentVariable('Path', 'User') + ';' + [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $env:HOME = $env:USERPROFILE = $env:LOCALAPPDATA = "$tmp\home"
    try {
        $output = @(pwsh -NoProfile -Command 'ahasend --version')
        $status = $LASTEXITCODE
    }
    finally {
        $env:Path, $env:HOME, $env:USERPROFILE, $env:LOCALAPPDATA = $saved
    }
    $output
    if ($status -or $output[0] -ne 'AhaSend CLI v99.0.0') {
        throw 'ahasend --version does not print AhaSend CLI v99.0.0'
    }
    Write-Host 'The WinGet manifests work'
}
finally {
    Stop-Process -Id $process.Id -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
