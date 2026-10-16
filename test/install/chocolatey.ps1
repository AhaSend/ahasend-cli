# Tests the Chocolatey package that scripts/choco-package.sh writes into
# <package> for the fake release v99.0.0, whose staged assets are in
# <release>\v99.0.0, with AHASEND_RELEASES_URL set to the fake release's URL:
#
#   test/install/chocolatey.ps1 -Package <package> -Release <release>
#
# It checks that VERIFICATION.txt gives the exe's release URL and its
# checksums.txt hash, packs the package, installs it from a local source,
# checks ahasend --version in a new shell and the licence files, and
# uninstalls it. It needs choco, which the Windows runners have, and runs as
# an administrator.
param(
    [Parameter(Mandatory)] [string] $Package,
    [Parameter(Mandatory)] [string] $Release
)
$ErrorActionPreference = 'Stop'
if (-not $env:AHASEND_RELEASES_URL) {
    throw 'set AHASEND_RELEASES_URL to the URL the package was filled in with'
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host 'Checking VERIFICATION.txt'
    $verification = Get-Content -Raw (Join-Path $Package 'tools\VERIFICATION.txt')
    $url = "$env:AHASEND_RELEASES_URL/download/v99.0.0/ahasend-windows-amd64.exe"
    $line = Get-Content (Join-Path $Release 'v99.0.0\checksums.txt') | Where-Object { $_ -match '  ahasend-windows-amd64\.exe$' }
    $sha256 = ($line -split '\s+')[0]
    if (-not $sha256) {
        throw 'checksums.txt has no line for ahasend-windows-amd64.exe'
    }
    if (-not $verification.Contains($url)) {
        throw "VERIFICATION.txt does not give the release URL $url"
    }
    if (-not $verification.Contains($sha256)) {
        throw "VERIFICATION.txt does not give the SHA-256 $sha256 from checksums.txt"
    }
    $exe = (Get-FileHash -Algorithm SHA256 (Join-Path $Package 'tools\ahasend.exe')).Hash.ToLower()
    if ($exe -ne $sha256) {
        throw "tools\ahasend.exe has the SHA-256 $exe, not $sha256"
    }

    Write-Host 'Packing the package'
    choco pack (Join-Path $Package 'ahasend.nuspec') --output-directory "$tmp\source"
    if ($LASTEXITCODE) {
        throw "choco pack failed with exit code $LASTEXITCODE"
    }

    Write-Host 'Installing the package from a local source'
    choco install ahasend --source "$tmp\source" --yes --no-progress
    if ($LASTEXITCODE) {
        throw "choco install failed with exit code $LASTEXITCODE"
    }

    # A new shell, with the PATH a new session gets, and a new home folder,
    # so nothing reads the user's own files
    Write-Host 'Checking ahasend --version in a new shell'
    New-Item -ItemType Directory -Path "$tmp\home" | Out-Null
    $saved = $env:Path, $env:HOME, $env:USERPROFILE, $env:LOCALAPPDATA
    $env:Path = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:HOME = $env:USERPROFILE = $env:LOCALAPPDATA = "$tmp\home"
    try {
        $version = @(pwsh -NoProfile -Command 'ahasend --version')
        $status = $LASTEXITCODE
    }
    finally {
        $env:Path, $env:HOME, $env:USERPROFILE, $env:LOCALAPPDATA = $saved
    }
    $version
    if ($status -or $version[0] -ne 'AhaSend CLI v99.0.0') {
        throw 'ahasend --version does not print AhaSend CLI v99.0.0'
    }

    Write-Host 'Checking the licence files'
    $tools = Join-Path $env:ChocolateyInstall 'lib\ahasend\tools'
    foreach ($file in 'LICENSE.txt', 'THIRD_PARTY_NOTICES.txt', 'VERIFICATION.txt') {
        if (-not (Test-Path (Join-Path $tools $file))) {
            throw "$file is missing from $tools"
        }
    }

    Write-Host 'Uninstalling the package'
    choco uninstall ahasend --yes
    if ($LASTEXITCODE) {
        throw "choco uninstall failed with exit code $LASTEXITCODE"
    }
    if (Test-Path (Join-Path $env:ChocolateyInstall 'bin\ahasend.exe')) {
        throw 'choco uninstall left the ahasend shim'
    }
    Write-Host 'The Chocolatey package works'
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
