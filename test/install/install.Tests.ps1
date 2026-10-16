# Tests install.ps1 against a fake release: the staged assets of the test
# snapshots v99.0.0 and v98.0.0-rc.1 in <Release>/<tag>/, served by the
# fake-release server (test/install/fake-release). Run it with Pester 5:
#
#   $data = @{ Release = '<release>'; Server = '<fake-release>' }
#   Invoke-Pester -Container (New-PesterContainer -Path test/install/install.Tests.ps1 -Data $data)
#
# Every install runs with a new temporary LOCALAPPDATA, USERPROFILE, HOME and
# TEMP, and with a PATH that holds no ahasend. The cases that run the
# installed binary run only on Windows; the others run on any PowerShell 7
# too. It needs no network.
param(
    [Parameter(Mandatory = $true)][string]$Release,
    [Parameter(Mandatory = $true)][string]$Server
)

BeforeDiscovery {
    $onWindows = $PSVersionTable.PSEdition -eq 'Desktop' -or $IsWindows
    # Root can write to any directory, so it cannot test an unwritable one
    $isRoot = (-not $onWindows) -and ((& id -u) -eq '0')
}

BeforeAll {
    $script:OnWindows = $PSVersionTable.PSEdition -eq 'Desktop' -or $IsWindows
    $script:Installer = (Resolve-Path (Join-Path $PSScriptRoot '../../install.ps1')).Path
    $script:Release = (Resolve-Path $Release).Path
    switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
        'X64' { $script:Asset = 'ahasend-windows-amd64.exe' }
        'Arm64' { $script:Asset = 'ahasend-windows-arm64.exe' }
    }

    $script:SavedEnvironment = @{}
    foreach ($name in 'LOCALAPPDATA', 'USERPROFILE', 'HOME', 'TEMP', 'TMP', 'TMPDIR', 'PATH', 'AHASEND_VERSION',
        'AHASEND_INSTALL_DIR', 'AHASEND_RELEASES_URL', 'NPM_CONFIG_PREFIX', 'SCOOP', 'SCOOP_GLOBAL', 'ChocolateyInstall') {
        $script:SavedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name)
    }

    # Tags the overlay folder adds to the release, which the server serves first
    $overlay = Join-Path $TestDrive 'overlay'
    $v99 = Join-Path $script:Release 'v99.0.0'
    # v97.0.0: the binary does not match checksums.txt
    $null = New-Item -ItemType Directory -Path (Join-Path $overlay 'v97.0.0')
    Copy-Item (Join-Path $v99 'checksums.txt'), (Join-Path $v99 $script:Asset) (Join-Path $overlay 'v97.0.0')
    Add-Content -LiteralPath (Join-Path $overlay "v97.0.0/$script:Asset") -Value 'x'
    # v96.0.0: checksums.txt lists the binary, which is missing
    $null = New-Item -ItemType Directory -Path (Join-Path $overlay 'v96.0.0')
    Copy-Item (Join-Path $v99 'checksums.txt') (Join-Path $overlay 'v96.0.0')
    # v94.0.0: checksums.txt has no line for the binary
    $null = New-Item -ItemType Directory -Path (Join-Path $overlay 'v94.0.0')
    Get-Content (Join-Path $v99 'checksums.txt') | Where-Object { $_ -notmatch " $([regex]::Escape($script:Asset))$" } |
        Set-Content (Join-Path $overlay 'v94.0.0/checksums.txt')
    Copy-Item (Join-Path $v99 $script:Asset) (Join-Path $overlay 'v94.0.0')
    # v93.0.0: the binary matches checksums.txt but does not run
    $null = New-Item -ItemType Directory -Path (Join-Path $overlay 'v93.0.0')
    $broken = Join-Path $overlay "v93.0.0/$script:Asset"
    Set-Content -LiteralPath $broken -Value 'not a program'
    $hash = (Get-FileHash -LiteralPath $broken -Algorithm SHA256).Hash.ToLower()
    Set-Content -LiteralPath (Join-Path $overlay 'v93.0.0/checksums.txt') -Value "$hash  $script:Asset"
    # The script under test, as the latest release serves it
    $null = New-Item -ItemType Directory -Path (Join-Path $overlay 'v99.0.0')
    Copy-Item $script:Installer (Join-Path $overlay 'v99.0.0')

    $script:LatestFile = Join-Path $TestDrive 'latest'
    Set-Content -LiteralPath $script:LatestFile -Value 'v99.0.0'
    $urlFile = Join-Path $TestDrive 'url'
    $script:ServerProcess = Start-Process -FilePath $Server -PassThru -NoNewWindow `
        -RedirectStandardError (Join-Path $TestDrive 'server.log') `
        -ArgumentList '-url', $urlFile, '-latest', $script:LatestFile, $overlay, $script:Release
    $tries = 0
    while (-not (Test-Path -LiteralPath $urlFile)) {
        $tries++
        if ($tries -gt 100) {
            throw 'The fake release server did not start.'
        }
        Start-Sleep -Milliseconds 100
    }
    $script:Url = (Get-Content -LiteralPath $urlFile).Trim()
    $env:AHASEND_RELEASES_URL = $script:Url

    $script:BasePath = Join-Path $TestDrive 'bin'
    $null = New-Item -ItemType Directory -Path $script:BasePath
    $script:Homes = 0

    # New-TestHome makes a new empty home folder, with an empty TEMP in it,
    # for the next install, and sets the environment back to the test's
    function New-TestHome {
        $script:Homes++
        $dir = Join-Path $TestDrive "home-$script:Homes"
        $temp = Join-Path $dir 'temp'
        $null = New-Item -ItemType Directory -Path $temp
        $env:LOCALAPPDATA = Join-Path $dir 'AppData/Local'
        $env:USERPROFILE = $dir
        $env:HOME = $dir
        $env:TEMP = $temp
        $env:TMP = $temp
        $env:TMPDIR = $temp
        $env:PATH = $script:BasePath
        foreach ($name in 'AHASEND_VERSION', 'AHASEND_INSTALL_DIR', 'NPM_CONFIG_PREFIX', 'SCOOP', 'SCOOP_GLOBAL',
            'ChocolateyInstall') {
            [Environment]::SetEnvironmentVariable($name, $null)
        }
        Set-Content -LiteralPath $script:LatestFile -Value 'v99.0.0'
        return [pscustomobject]@{
            Dir = $dir
            Temp = $temp
            DefaultInstallDir = Join-Path $dir 'AppData/Local/Programs/AhaSend/bin'
        }
    }

    # Invoke-Installer runs install.ps1 with the given parameters, and returns
    # its output and the message of the error it threw, if any
    function Invoke-Installer {
        param([hashtable]$Parameters = @{})
        $output = New-Object System.Collections.Generic.List[string]
        $failure = $null
        try {
            & $script:Installer @Parameters *>&1 | ForEach-Object { $output.Add([string]$_) }
        } catch {
            $failure = $_.Exception.Message
        }
        return [pscustomobject]@{ Output = $output -join "`n"; Error = $failure }
    }

    # Get-InstalledVersion returns the first line that Dir/ahasend.exe prints
    # for --version
    function Get-InstalledVersion {
        param([string]$Dir)
        return @(& (Join-Path $Dir 'ahasend.exe') --version)[0]
    }

    # New-OldBinary puts a file that stands in for an installed ahasend in Dir
    function New-OldBinary {
        param([string]$Dir)
        $null = New-Item -ItemType Directory -Force -Path $Dir
        Set-Content -LiteralPath (Join-Path $Dir 'ahasend.exe') -Value 'old binary'
    }

    # Assert-OldBinary checks that the file New-OldBinary made is unchanged
    function Assert-OldBinary {
        param([string]$Dir)
        Get-Content -LiteralPath (Join-Path $Dir 'ahasend.exe') | Should -Be 'old binary'
    }

    # Assert-NoLeftovers checks that TEMP is empty and that Dir holds no
    # temporary copy of the binary
    function Assert-NoLeftovers {
        param($TestHome, [string]$Dir)
        @(Get-ChildItem -Force -LiteralPath $TestHome.Temp) | Should -HaveCount 0
        if ($Dir -and (Test-Path -LiteralPath $Dir)) {
            @(Get-ChildItem -Force -LiteralPath $Dir | Where-Object { $_.Name -like '.ahasend-*' }) |
                Should -HaveCount 0
        }
    }

    # Invoke-Session runs the given lines in a new session of this PowerShell,
    # as if typed one after the other, and returns its output
    function Invoke-Session {
        param([string[]]$Lines)
        $exe = (Get-Process -Id $PID).Path
        $ErrorActionPreference = 'Continue'
        return ($Lines | & $exe -NoProfile -NonInteractive -Command - 2>&1 | ForEach-Object { [string]$_ }) -join "`n"
    }
}

AfterAll {
    if ($script:ServerProcess) {
        Stop-Process -Id $script:ServerProcess.Id -Force -ErrorAction SilentlyContinue
        $script:ServerProcess.WaitForExit()
    }
    foreach ($name in $script:SavedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $script:SavedEnvironment[$name])
    }
}

Describe 'Installs' -Skip:(-not $onWindows) {
    It 'installs into LOCALAPPDATA\Programs\AhaSend\bin by default' {
        $testHome = New-TestHome
        $result = Invoke-Installer
        $result.Error | Should -BeNullOrEmpty
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v99.0.0'
        $result.Output | Should -Match 'Installed AhaSend CLI v99\.0\.0'
        Assert-NoLeftovers $testHome $testHome.DefaultInstallDir
    }

    It 'installs into -InstallDir, creating it' {
        $testHome = New-TestHome
        $dir = Join-Path $testHome.Dir 'opt/bin'
        (Invoke-Installer @{ InstallDir = $dir }).Error | Should -BeNullOrEmpty
        Get-InstalledVersion $dir | Should -Be 'AhaSend CLI v99.0.0'
        Assert-NoLeftovers $testHome $dir
    }

    It 'installs into AHASEND_INSTALL_DIR' {
        $testHome = New-TestHome
        $dir = Join-Path $testHome.Dir 'env-bin'
        $env:AHASEND_INSTALL_DIR = $dir
        (Invoke-Installer).Error | Should -BeNullOrEmpty
        Get-InstalledVersion $dir | Should -Be 'AhaSend CLI v99.0.0'
    }

    It 'installs the -Version tag, a pre-release too' {
        $testHome = New-TestHome
        (Invoke-Installer @{ Version = 'v98.0.0-rc.1' }).Error | Should -BeNullOrEmpty
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v98.0.0-rc.1'
    }

    It 'installs the AHASEND_VERSION tag' {
        $testHome = New-TestHome
        $env:AHASEND_VERSION = 'v98.0.0-rc.1'
        (Invoke-Installer).Error | Should -BeNullOrEmpty
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v98.0.0-rc.1'
    }

    It 'upgrades an older binary' {
        $testHome = New-TestHome
        (Invoke-Installer @{ Version = 'v98.0.0-rc.1' }).Error | Should -BeNullOrEmpty
        (Invoke-Installer).Error | Should -BeNullOrEmpty
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v99.0.0'
        Assert-NoLeftovers $testHome $testHome.DefaultInstallDir
    }

    It 'prints nothing with -Quiet, not even the PATH hint' {
        $testHome = New-TestHome
        $result = Invoke-Installer @{ Quiet = $true }
        $result.Error | Should -BeNullOrEmpty
        $result.Output | Should -BeNullOrEmpty
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v99.0.0'
    }

    It 'prints the command that adds a directory not on PATH' {
        $testHome = New-TestHome
        $result = Invoke-Installer
        $result.Output | Should -BeLike "*$($testHome.DefaultInstallDir) is not on PATH*"
        $result.Output | Should -BeLike "*Set-ItemProperty `$key Path ((@(`$path, '$($testHome.DefaultInstallDir)') -ne '') -join ';') -Type ExpandString*"
    }

    It 'prints a PATH command that keeps the user Path expandable' {
        $testHome = New-TestHome
        $result = Invoke-Installer
        $command = @($result.Output -split "`n" | Where-Object { $_ -like '*HKCU:\Environment*' })[0]
        $command | Should -Not -BeNullOrEmpty
        $environment = 'HKCU:\Environment'
        $saved = (Get-Item $environment).GetValue('Path', $null, 'DoNotExpandEnvironmentNames')
        $savedKind = if ($null -ne $saved) { (Get-Item $environment).GetValueKind('Path') }
        try {
            Set-ItemProperty $environment Path '%SystemRoot%\ahasend-test' -Type ExpandString
            & ([scriptblock]::Create($command.Trim()))
            (Get-Item $environment).GetValueKind('Path') | Should -Be 'ExpandString'
            (Get-Item $environment).GetValue('Path', $null, 'DoNotExpandEnvironmentNames') |
                Should -Be "%SystemRoot%\ahasend-test;$($testHome.DefaultInstallDir)"
        } finally {
            if ($null -eq $saved) {
                Remove-ItemProperty $environment Path -ErrorAction SilentlyContinue
            } else {
                Set-ItemProperty $environment Path $saved -Type $savedKind
            }
        }
    }

    It 'prints no hint for a directory on PATH' {
        $testHome = New-TestHome
        $env:PATH = $script:BasePath + [System.IO.Path]::PathSeparator + $testHome.DefaultInstallDir
        $result = Invoke-Installer
        $result.Error | Should -BeNullOrEmpty
        $result.Output | Should -Not -BeLike '*not on PATH*'
        $result.Output | Should -Not -BeLike '*comes first on PATH*'
    }

    It 'installs and warns about another copy earlier on PATH' {
        $testHome = New-TestHome
        $other = Join-Path $testHome.Dir 'other'
        New-OldBinary $other
        $env:PATH = $other + [System.IO.Path]::PathSeparator + $testHome.DefaultInstallDir
        $result = Invoke-Installer
        $result.Error | Should -BeNullOrEmpty
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v99.0.0'
        $result.Output | Should -BeLike "*$(Join-Path $other 'ahasend.exe') comes first on PATH*"
        $result.Output | Should -BeLike "*Remove it, or put $($testHome.DefaultInstallDir) before $other on PATH*"
    }

    It 'installs and warns about a copy that WinGet installed earlier on PATH' {
        $testHome = New-TestHome
        $links = Join-Path $testHome.Dir 'AppData/Local/Microsoft/WinGet/Links'
        New-OldBinary $links
        $env:PATH = $links
        $result = Invoke-Installer
        $result.Error | Should -BeNullOrEmpty
        $result.Output | Should -BeLike '*WinGet installed it; upgrade it with: winget upgrade AhaSend.CLI*'
        $result.Output | Should -BeLike '*is not on PATH*'
    }

    It 'installs through irm | iex' {
        $testHome = New-TestHome
        $output = Invoke-Session "irm '$script:Url/latest/download/install.ps1' | iex"
        $output | Should -BeLike '*Installed AhaSend CLI v99.0.0*'
        Get-InstalledVersion $testHome.DefaultInstallDir | Should -Be 'AhaSend CLI v99.0.0'
    }

    It 'passes parameters through the [scriptblock]::Create form' {
        $testHome = New-TestHome
        $dir = Join-Path $testHome.Dir 'sb-bin'
        & ([scriptblock]::Create((Invoke-RestMethod "$script:Url/latest/download/install.ps1"))) `
            -Version v98.0.0-rc.1 -InstallDir $dir -Quiet
        Get-InstalledVersion $dir | Should -Be 'AhaSend CLI v98.0.0-rc.1'
    }
}

Describe 'Failures, each keeping the old binary' {
    BeforeEach {
        $testHome = New-TestHome
        $dir = $testHome.DefaultInstallDir
        New-OldBinary $dir
    }

    It 'fails on a checksum mismatch' {
        (Invoke-Installer @{ Version = 'v97.0.0' }).Error | Should -BeLike '*does not match checksums.txt*'
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'fails on a missing asset' {
        (Invoke-Installer @{ Version = 'v96.0.0' }).Error |
            Should -BeLike "*Could not download $script:Url/download/v96.0.0/$script:Asset*"
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'fails on a checksums.txt without the binary' {
        (Invoke-Installer @{ Version = 'v94.0.0' }).Error | Should -BeLike "*checksums.txt has no line for $script:Asset*"
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'fails on a binary that does not run' {
        (Invoke-Installer @{ Version = 'v93.0.0' }).Error | Should -BeLike '*The downloaded binary does not run*'
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'fails on a 404 on latest' {
        Remove-Item -LiteralPath $script:LatestFile
        (Invoke-Installer).Error | Should -BeLike "*Could not find the latest release at $script:Url/latest*"
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'refuses a pre-release returned as latest' {
        Set-Content -LiteralPath $script:LatestFile -Value 'v98.0.0-rc.1'
        (Invoke-Installer).Error | Should -BeLike '*v98.0.0-rc.1, is not a stable version*'
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'refuses an empty -Version or -InstallDir' {
        (Invoke-Installer @{ Version = '' }).Error | Should -BeLike '*-Version needs a value*'
        (Invoke-Installer @{ InstallDir = '' }).Error | Should -BeLike '*-InstallDir needs a value*'
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'refuses a -Version that is not a tag' {
        (Invoke-Installer @{ Version = '1.2.3' }).Error | Should -BeLike '*1.2.3 is not a release tag*'
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'prints only the error with -Quiet' {
        $result = Invoke-Installer @{ Version = 'v96.0.0'; Quiet = $true }
        $result.Error | Should -BeLike '*Could not download*'
        $result.Output | Should -BeNullOrEmpty
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }

    It 'fails on a directory it cannot write to' -Skip:$isRoot {
        if ($script:OnWindows) {
            $null = & "$env:SystemRoot\System32\icacls.exe" $dir /deny "$($env:USERNAME):(W)"
        } else {
            & /bin/chmod 555 $dir
        }
        try {
            (Invoke-Installer).Error | Should -BeLike "*Cannot write to $dir*"
        } finally {
            if ($script:OnWindows) {
                $null = & "$env:SystemRoot\System32\icacls.exe" $dir /remove:d $env:USERNAME
            } else {
                & /bin/chmod 755 $dir
            }
        }
        Assert-OldBinary $dir
        Assert-NoLeftovers $testHome $dir
    }
}

Describe 'Package-manager installs' {
    # Test-Refused checks that installing into Dir, with an ahasend.exe in
    # it, fails with Upgrade, keeps that file and leaves no temporary file
    BeforeAll {
        function Test-Refused {
            param($TestHome, [string]$Dir, [string]$Upgrade)
            New-OldBinary $Dir
            (Invoke-Installer @{ InstallDir = $Dir }).Error | Should -BeLike "*Upgrade it with: $Upgrade"
            Assert-OldBinary $Dir
            Assert-NoLeftovers $TestHome $Dir
        }
    }

    It 'refuses a Scoop shims folder with no ahasend.exe in it yet' {
        $testHome = New-TestHome
        $dir = Join-Path $testHome.Dir 'scoop/shims'
        $null = New-Item -ItemType Directory -Force -Path $dir
        (Invoke-Installer @{ InstallDir = $dir }).Error | Should -BeLike '*Upgrade it with: scoop update ahasend'
        Test-Path -LiteralPath (Join-Path $dir 'ahasend.exe') | Should -BeFalse
        Assert-NoLeftovers $testHome $dir
    }

    It 'refuses a Scoop shims folder' {
        $testHome = New-TestHome
        Test-Refused $testHome (Join-Path $testHome.Dir 'scoop/shims') 'scoop update ahasend'
    }

    It 'refuses the shims folder of SCOOP' {
        $testHome = New-TestHome
        $env:SCOOP = Join-Path $testHome.Dir 'apps'
        Test-Refused $testHome (Join-Path $env:SCOOP 'shims') 'scoop update ahasend'
    }

    It 'refuses a WinGet\Links folder' {
        $testHome = New-TestHome
        Test-Refused $testHome (Join-Path $testHome.Dir 'AppData/Local/Microsoft/WinGet/Links') 'winget upgrade AhaSend.CLI'
    }

    It 'refuses a chocolatey\bin folder' {
        $testHome = New-TestHome
        Test-Refused $testHome (Join-Path $testHome.Dir 'ProgramData/chocolatey/bin') 'choco upgrade ahasend'
    }

    It 'refuses the bin folder of ChocolateyInstall' {
        $testHome = New-TestHome
        $env:ChocolateyInstall = Join-Path $testHome.Dir 'choco-root'
        Test-Refused $testHome (Join-Path $env:ChocolateyInstall 'bin') 'choco upgrade ahasend'
    }

    It 'refuses the npm prefix' {
        $testHome = New-TestHome
        $env:NPM_CONFIG_PREFIX = Join-Path $testHome.Dir 'npm-global'
        Test-Refused $testHome $env:NPM_CONFIG_PREFIX 'npm install -g @ahasend/cli'
    }
}

Describe 'Running from the web' {
    It 'leaves the session running when it fails under iex' {
        $testHome = New-TestHome
        $env:AHASEND_VERSION = 'v96.0.0'
        $output = Invoke-Session "irm '$script:Url/latest/download/install.ps1' | iex", "'still running'"
        $output | Should -BeLike '*Could not download*'
        $output | Should -BeLike '*still running*'
        Assert-NoLeftovers $testHome $testHome.DefaultInstallDir
    }

    It 'passes -Version through the [scriptblock]::Create form' {
        $null = New-TestHome
        $script = Invoke-RestMethod "$script:Url/latest/download/install.ps1"
        { & ([scriptblock]::Create($script)) -Version v96.0.0 -Quiet } | Should -Throw '*download/v96.0.0/*'
    }

    It 'runs nothing when cut off at any line' {
        $testHome = New-TestHome
        $lines = @(Get-Content -LiteralPath $script:Installer)
        $lines[-1] | Should -Be 'if ($true) { Install-AhaSend -Version $Version -InstallDir $InstallDir -Quiet $Quiet.IsPresent -Given @($PSBoundParameters.Keys) }'
        for ($n = 1; $n -lt $lines.Count; $n++) {
            $text = $lines[0..($n - 1)] -join "`n"
            try {
                $null = & ([scriptblock]::Create($text)) *>&1
            } catch {
                # A parse error runs nothing, which is the point
                $null = $_
            }
        }
        Test-Path -LiteralPath (Join-Path $testHome.Dir 'AppData') | Should -BeFalse
        Assert-NoLeftovers $testHome
    }
}
