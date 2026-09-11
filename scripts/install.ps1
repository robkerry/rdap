# Install the latest rdap release for Windows.
# Usage:
#   irm https://raw.githubusercontent.com/robkerry/rdap/main/scripts/install.ps1 | iex
#   $env:RDAP_INSTALL_DIR = "$env:USERPROFILE\bin"; .\scripts\install.ps1

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = 'robkerry/rdap'
$BaseUrl = "https://github.com/$Repo/releases/latest/download"

$arch = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) {
    $arch = $env:PROCESSOR_ARCHITEW6432
}

switch -Regex ($arch) {
    'ARM64' { $goarch = 'arm64' }
    'AMD64' { $goarch = 'amd64' }
    default {
        throw "Unsupported architecture '$arch'. Download a zip from https://github.com/$Repo/releases/latest"
    }
}

$installDir = $env:RDAP_INSTALL_DIR
if (-not $installDir) {
    $installDir = Join-Path $env:LOCALAPPDATA 'rdap'
}

$tmpdir = Join-Path ([System.IO.Path]::GetTempPath()) ("rdap-install-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmpdir | Out-Null

try {
    $sumsPath = Join-Path $tmpdir 'SHA256SUMS'
    Write-Host 'Downloading checksums...'
    Invoke-WebRequest -Uri "$BaseUrl/SHA256SUMS" -OutFile $sumsPath -UseBasicParsing

    $asset = $null
    $expected = $null
    Get-Content $sumsPath | ForEach-Object {
        if ($_ -match '^\s*([A-Fa-f0-9]{64})\s+(\S+)$') {
            $name = $Matches[2]
            if ($name -match "^rdap_.+_windows_$goarch\.zip$") {
                $expected = $Matches[1].ToLowerInvariant()
                $asset = $name
            }
        }
    }

    if (-not $asset -or -not $expected) {
        throw "No release archive found for windows/$goarch."
    }

    $zipPath = Join-Path $tmpdir $asset
    Write-Host "Downloading $asset..."
    Invoke-WebRequest -Uri "$BaseUrl/$asset" -OutFile $zipPath -UseBasicParsing

    $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "Checksum mismatch for $asset.`n  expected: $expected`n  actual:   $actual"
    }

    Expand-Archive -Path $zipPath -DestinationPath $tmpdir -Force
    $binary = Join-Path $tmpdir 'rdap.exe'
    if (-not (Test-Path $binary)) {
        throw 'Archive did not contain rdap.exe.'
    }

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item -Path $binary -Destination (Join-Path $installDir 'rdap.exe') -Force

    $installed = Join-Path $installDir 'rdap.exe'
    Write-Host "Installed $installed"
    & $installed --version

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) {
        $userPath = ''
    }
    $parts = $userPath -split ';' | Where-Object { $_ -ne '' }
    if ($parts -notcontains $installDir) {
        $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        $env:Path = "$env:Path;$installDir"
        Write-Host
        Write-Host "Added $installDir to your user PATH. Open a new terminal if rdap is not found."
    }
}
finally {
    Remove-Item -Recurse -Force $tmpdir -ErrorAction SilentlyContinue
}
