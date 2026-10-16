# Tests the Scoop manifest that GoReleaser writes into
# <dist>\scoop\ahasend.json, generated with AHASEND_RELEASES_URL set to a
# fake release's URL, such as http://127.0.0.1:8484. It serves the staged
# assets of v99.0.0 in <release>\v99.0.0 at that URL with the fake-release
# server:
#
#   test/install/scoop.ps1 -Dist <dist> -Release <release> -Server <fake-release.exe>
#
# It needs Scoop, installed in the folder SCOOP names, so the test never
# touches the user's own Scoop. It installs the manifest, checks ahasend
# --version in a new shell and the licence files, and uninstalls it.
param(
    [Parameter(Mandatory)] [string] $Dist,
    [Parameter(Mandatory)] [string] $Release,
    [Parameter(Mandatory)] [string] $Server
)
$ErrorActionPreference = 'Stop'
if (-not $env:AHASEND_RELEASES_URL) {
    throw 'set AHASEND_RELEASES_URL to the URL the manifest downloads from'
}

if (-not $env:SCOOP) {
    throw 'set SCOOP to a Scoop installed for this test, not your own'
}
$scoopRoot = $env:SCOOP
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

    Write-Host 'Installing the manifest'
    scoop install (Join-Path $Dist 'scoop\ahasend.json')
    if ($LASTEXITCODE) {
        throw "scoop install failed with exit code $LASTEXITCODE"
    }

    # A new shell, with the PATH a new session of the user gets, and a new
    # home folder, so nothing reads the user's own files
    Write-Host 'Checking ahasend --version in a new shell'
    New-Item -ItemType Directory -Path "$tmp\home" | Out-Null
    $saved = $env:HOME, $env:USERPROFILE, $env:LOCALAPPDATA
    $env:HOME = $env:USERPROFILE = $env:LOCALAPPDATA = "$tmp\home"
    try {
        $version = @(pwsh -NoProfile -Command '$env:Path = [Environment]::GetEnvironmentVariable("Path", "User") + ";" + [Environment]::GetEnvironmentVariable("Path", "Machine"); ahasend --version')
        $status = $LASTEXITCODE
    }
    finally {
        $env:HOME, $env:USERPROFILE, $env:LOCALAPPDATA = $saved
    }
    $version
    if ($status -or $version[0] -ne 'AhaSend CLI v99.0.0') {
        throw 'ahasend --version does not print AhaSend CLI v99.0.0'
    }

    Write-Host 'Checking the licence files'
    $prefix = scoop prefix ahasend
    foreach ($file in 'LICENSE', 'THIRD_PARTY_NOTICES') {
        if (-not (Test-Path (Join-Path $prefix $file))) {
            throw "$file is missing from $prefix"
        }
    }

    Write-Host 'Uninstalling the manifest'
    scoop uninstall ahasend
    if ($LASTEXITCODE) {
        throw "scoop uninstall failed with exit code $LASTEXITCODE"
    }
    if (Test-Path (Join-Path $scoopRoot 'shims\ahasend.exe')) {
        throw 'scoop uninstall left the ahasend shim'
    }
    Write-Host 'The Scoop manifest works'
}
finally {
    Stop-Process -Id $process.Id -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
