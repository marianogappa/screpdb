# screpdb installer for Windows.
#
#   irm https://raw.githubusercontent.com/marianogappa/screpdb/main/install.ps1 | iex
#
# Downloads the screpdb dashboard, verifies it against the release's SHA256SUMS,
# installs it to %LOCALAPPDATA%\Programs\screpdb, adds a Start Menu shortcut and
# opens it. PowerShell downloads carry no Mark-of-the-Web, so SmartScreen doesn't
# block the unsigned binary. The folder is user-writable and not owned by a
# package manager, so the in-app Update button keeps it current.
#
# Environment overrides:
#   SCREPDB_VERSION      install a specific version (e.g. 1.29.1) instead of latest
#   SCREPDB_INSTALL_DIR  install directory (default: %LOCALAPPDATA%\Programs\screpdb)
#   SCREPDB_NO_LAUNCH    set to 1 to skip opening screpdb after installing

& {
    $ErrorActionPreference = 'Stop'
    # Windows PowerShell 5.1 can default to TLS 1.0, which GitHub rejects, and its
    # progress bar makes Invoke-WebRequest many times slower.
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'

    $repo = 'marianogappa/screpdb'
    $asset = 'screpdb-gui-windows-amd64.exe'

    if (-not [Environment]::Is64BitOperatingSystem) {
        throw 'screpdb needs 64-bit Windows.'
    }

    $scoopShim = Join-Path $env:USERPROFILE 'scoop\shims\screpdb-gui.exe'
    if (Test-Path $scoopShim) {
        Write-Host 'screpdb is already installed with Scoop. Upgrade it with: scoop update screpdb'
        return
    }

    $version = $env:SCREPDB_VERSION
    if ($version) {
        $base = "https://github.com/$repo/releases/download/v$($version.TrimStart('v'))"
    } else {
        $base = "https://github.com/$repo/releases/latest/download"
    }

    $dir = $env:SCREPDB_INSTALL_DIR
    if (-not $dir) {
        $dir = Join-Path $env:LOCALAPPDATA 'Programs\screpdb'
    }
    $exe = Join-Path $dir 'screpdb.exe'

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("screpdb-install-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading screpdb ($(if ($version) { $version } else { 'latest' }))..."
        $download = Join-Path $tmp $asset
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $download
        $sums = (Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS").Content
        if ($sums -is [byte[]]) {
            $sums = [Text.Encoding]::UTF8.GetString($sums)
        }

        $expected = $null
        foreach ($line in $sums -split "`n") {
            $fields = $line.Trim() -split '\s+'
            if ($fields.Count -eq 2 -and $fields[1].TrimStart('*') -eq $asset) {
                $expected = $fields[0]
                break
            }
        }
        if (-not $expected) {
            throw "No checksum for $asset in SHA256SUMS."
        }
        $actual = (Get-FileHash -Algorithm SHA256 -Path $download).Hash
        if ($actual -ne $expected) {
            throw "Checksum mismatch for $asset (expected $expected, got $actual)."
        }
        Write-Host 'Checksum OK.'

        Get-Process -Name screpdb -ErrorAction SilentlyContinue |
            Where-Object { $_.Path -eq $exe } |
            Stop-Process -Force
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        Move-Item -Force -Path $download -Destination $exe
    } finally {
        Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
    }

    $startMenu = [Environment]::GetFolderPath('Programs')
    $shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $startMenu 'screpdb.lnk'))
    $shortcut.TargetPath = $exe
    $shortcut.WorkingDirectory = $dir
    $shortcut.Description = 'screpdb dashboard'
    $shortcut.Save()

    Write-Host ''
    Write-Host "Installed screpdb to $exe"
    Write-Host 'Open it any time from the Start Menu (screpdb).'

    if ($env:SCREPDB_NO_LAUNCH -ne '1') {
        Start-Process -FilePath $exe -WorkingDirectory $dir
    }
}
