param(
    [string]$InstallDir = "C:\1Panel",
    [string]$ZipPath = "",
    [switch]$SkipBackup,
    [switch]$FullBackup,
    [switch]$Rollback,
    [switch]$RestoreData,
    [switch]$NoDbFix
)

$ErrorActionPreference = "Stop"

$CoreServiceName  = "1panel-core-service"
$AgentServiceName = "1panel-agent-service"

# Win7 SP1 ships PowerShell 2.0 on the .NET 2.0 CLR. Compute the script dir the
# PS2 way ($PSScriptRoot is empty in script scope before PS 3.0) and provide a
# blank test that avoids [string]::IsNullOrWhiteSpace (a .NET 4.0 method).
$script:ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

function Test-Blank {
    param([string]$Value)
    if ($null -eq $Value) { return $true }
    return ($Value.Trim().Length -eq 0)
}

function Expand-ZipCompat {
    param([string]$ZipFile, [string]$DestDir)
    Ensure-Dir $DestDir
    if ($null -ne (Get-Command Expand-Archive -ErrorAction SilentlyContinue)) {
        Expand-Archive -LiteralPath $ZipFile -DestinationPath $DestDir -Force
        return
    }
    # PowerShell 2.0 fallback: extract via the Shell COM object (CopyHere is
    # async, so wait until the item count settles).
    $shell = New-Object -ComObject Shell.Application
    $zipNs = $shell.NameSpace($ZipFile)
    $dstNs = $shell.NameSpace($DestDir)
    if ($null -eq $zipNs -or $null -eq $dstNs) { throw "cannot open zip via shell: $ZipFile" }
    $dstNs.CopyHere($zipNs.Items(), 0x14)
    for ($i = 0; $i -lt 120; $i++) {
        Start-Sleep -Milliseconds 500
        if (Test-Path (Join-Path $DestDir "1panel-core.exe")) { break }
    }
}

function Write-Log {
    param([string]$Message)
    Write-Host "[1Panel Upgrade] $Message"
}

function Ensure-Dir {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Assert-Administrator {
    $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object System.Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "please run as Administrator"
    }
}

function Get-ServiceDir {
    param([string]$Root)
    return (Join-Path $Root "service")
}

function Get-WrapperPath {
    param(
        [string]$Root,
        [string]$ServiceName
    )
    return (Join-Path (Get-ServiceDir $Root) "$ServiceName.exe")
}

function Stop-PanelService {
    param(
        [string]$Root,
        [string]$ServiceName
    )
    $stopped = $false
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -ne $svc) {
        try {
            Stop-Service -Name $ServiceName -Force -ErrorAction Stop
            $stopped = $true
        } catch {
            Write-Log "Stop-Service failed for $ServiceName ($($_.Exception.Message)); trying WinSW wrapper"
        }
    }
    if (-not $stopped) {
        $wrapper = Get-WrapperPath -Root $Root -ServiceName $ServiceName
        if (Test-Path $wrapper) {
            try {
                & $wrapper "stop" | Out-Null
            } catch {
                Write-Log "WinSW stop failed for $ServiceName ($($_.Exception.Message))"
            }
        } else {
            Write-Log "no service or wrapper found for $ServiceName; assuming already stopped"
        }
    }
    Write-Log "requested stop: $ServiceName"
}

function Start-PanelService {
    param(
        [string]$Root,
        [string]$ServiceName
    )
    $started = $false
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -ne $svc) {
        try {
            Start-Service -Name $ServiceName -ErrorAction Stop
            $started = $true
        } catch {
            Write-Log "Start-Service failed for $ServiceName ($($_.Exception.Message)); trying WinSW wrapper"
        }
    }
    if (-not $started) {
        $wrapper = Get-WrapperPath -Root $Root -ServiceName $ServiceName
        if (Test-Path $wrapper) {
            & $wrapper "start" | Out-Null
        } else {
            throw "cannot start ${ServiceName}: neither Windows service nor WinSW wrapper found"
        }
    }
    Write-Log "requested start: $ServiceName"
}

function Wait-ServiceState {
    param(
        [string]$ServiceName,
        [string]$DesiredState,
        [int]$TimeoutSeconds = 60
    )
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($null -ne $svc -and $svc.Status -eq $DesiredState) {
            Write-Log "service $ServiceName reached state: $DesiredState"
            return $true
        }
        Start-Sleep -Seconds 1
    }
    Write-Log "WARNING: service $ServiceName did not reach $DesiredState within ${TimeoutSeconds}s"
    return $false
}

function Wait-ProcessExit {
    param(
        [string]$BinDir,
        [int]$TimeoutSeconds = 60
    )
    $exeNames = @("1panel-core", "1panel-agent")
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        $running = @()
        foreach ($name in $exeNames) {
            $procs = Get-Process -Name $name -ErrorAction SilentlyContinue
            foreach ($p in $procs) {
                try {
                    $path = $p.Path
                } catch {
                    $path = $null
                }
                $binPrefix = $BinDir.TrimEnd('\') + '\'
                if ($null -ne $path -and $path.StartsWith($binPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
                    $running += $p
                }
            }
        }
        if ($running.Count -eq 0) {
            return $true
        }
        Start-Sleep -Seconds 1
    }
    Write-Log "WARNING: panel processes still running after ${TimeoutSeconds}s"
    return $false
}

function Test-FileUnlocked {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        return $true
    }
    try {
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
        $stream.Close()
        $stream.Dispose()
        return $true
    } catch {
        return $false
    }
}

function Get-PackageVersion {
    param(
        [string]$ExtractDir,
        [string]$ZipFileName
    )
    # The bare-binary path extracts nothing, so also look next to the script
    # (release packages ship VERSION alongside the binaries and this script).
    foreach ($dir in @($ExtractDir, $script:ScriptDir)) {
        if (Test-Blank $dir) { continue }
        $versionFile = Join-Path $dir "VERSION"
        if (Test-Path $versionFile) {
            $content = ([System.IO.File]::ReadAllText($versionFile)).Trim()
            if (-not (Test-Blank $content)) {
                return $content
            }
        }
    }
    if (-not (Test-Blank $ZipFileName)) {
        $m = [regex]::Match($ZipFileName, '^1panel-(.+)-windows-[^-]+\.zip$')
        if ($m.Success) {
            return $m.Groups[1].Value
        }
    }
    return "unknown"
}

function Invoke-DbFix {
    param(
        [string]$Root,
        [string]$DbPath
    )
    if ($NoDbFix) {
        Write-Log "skip db fix (-NoDbFix)"
        return
    }
    if (-not (Test-Path $DbPath)) {
        Write-Log "db not found, skip jerinte fix: $DbPath"
        return
    }
    $sqlite = $null
    $toolsSqlite = Join-Path $Root "tools\sqlite3.exe"
    if (Test-Path $toolsSqlite) {
        $sqlite = $toolsSqlite
    } else {
        $cmd = Get-Command "sqlite3.exe" -ErrorAction SilentlyContinue
        if ($null -ne $cmd) {
            $sqlite = $cmd.Source
        }
    }

    if ($null -ne $sqlite) {
        # Guard the whole fix: a schema mismatch or corrupt db must not abort the
        # upgrade (services are already stopped and binaries replaced by now).
        $hasTable = & $sqlite $DbPath "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='windows_services';" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Log "WARNING: sqlite3 could not read db ($hasTable); skip jerinte fix. Fix manually if needed."
            return
        }
        if ("$hasTable".Trim() -ne "1") {
            Write-Log "windows_services table not present, skip jerinte fix"
            return
        }
        $sql = "BEGIN; UPDATE windows_services SET service_type='package' WHERE service_type='jerinte'; SELECT changes(); COMMIT;"
        $result = & $sqlite $DbPath $sql 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Log "WARNING: jerinte fix failed ($result); run manually: sqlite3.exe `"$DbPath`" `"UPDATE windows_services SET service_type='package' WHERE service_type='jerinte';`""
            return
        }
        Write-Log "jerinte -> package rows updated: $result"
    } else {
        $bytes = [System.IO.File]::ReadAllBytes($DbPath)
        $text = [System.Text.Encoding]::ASCII.GetString($bytes)
        if ($text.Contains("jerinte")) {
            Write-Log "WARNING: sqlite3 not found but 'jerinte' bytes present in db."
            Write-Log "Run manually: sqlite3.exe `"$DbPath`" `"UPDATE windows_services SET service_type='package' WHERE service_type='jerinte';`""
        } else {
            Write-Log "sqlite3 not found; no 'jerinte' bytes detected, skip"
        }
    }
}

function Resolve-Sqlite {
    param([string]$Root)
    $toolsSqlite = Join-Path $Root "tools\sqlite3.exe"
    if (Test-Path $toolsSqlite) { return $toolsSqlite }
    $cmd = Get-Command "sqlite3.exe" -ErrorAction SilentlyContinue
    if ($null -ne $cmd) { return $cmd.Source }
    return $null
}

# Set-PanelVersion makes the panel footer reflect the new version. The upgraded
# core binary self-heals SystemVersion in core.db on startup (compiled version
# injected via ldflags), so this is belt-and-suspenders: it updates
# ORIGINAL_VERSION in 1pctl.env (from which the agent re-derives its version)
# and, when sqlite3.exe is available, writes SystemVersion immediately.
function Set-PanelVersion {
    param(
        [string]$Root,
        [string]$ConfEnv,
        [string]$DataDir,
        [string]$Version
    )
    if ((Test-Blank $Version) -or ($Version -eq "unknown")) {
        Write-Log "version unknown, skip version sync"
        return
    }
    if (Test-Path $ConfEnv) {
        $lines = @(Get-Content -Path $ConfEnv)
        $found = $false
        for ($i = 0; $i -lt $lines.Count; $i++) {
            if ($lines[$i] -match '^ORIGINAL_VERSION=') {
                $lines[$i] = "ORIGINAL_VERSION=$Version"
                $found = $true
            }
        }
        if ($found) {
            Set-Content -Path $ConfEnv -Value $lines -Encoding ASCII
            Write-Log "updated ORIGINAL_VERSION in $ConfEnv : $Version"
        }
    }
    $sqlite = Resolve-Sqlite -Root $Root
    if ($null -ne $sqlite) {
        foreach ($db in @((Join-Path $DataDir "db\core.db"), (Join-Path $DataDir "db\agent.db"))) {
            if (-not (Test-Path $db)) { continue }
            & $sqlite $db "UPDATE settings SET value='$Version' WHERE key='SystemVersion';" 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) {
                Write-Log "WARNING: could not update SystemVersion in $db"
            }
        }
        Write-Log "SystemVersion set to $Version in core.db/agent.db"
    } else {
        Write-Log "sqlite3 not found; core binary will self-heal SystemVersion on next start"
    }
}

function Backup-Current {
    param(
        [string]$Root,
        [string]$BinDir,
        [string]$DataDir
    )
    if ($SkipBackup) {
        Write-Log "skip backup (-SkipBackup)"
        return ""
    }
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $backupDir = Join-Path (Join-Path $Root "upgrade-backup") $timestamp
    Ensure-Dir $backupDir
    $backupBin = Join-Path $backupDir "bin"
    Ensure-Dir $backupBin
    foreach ($exe in @("1panel-core.exe", "1panel-agent.exe")) {
        $src = Join-Path $BinDir $exe
        if (Test-Path $src) {
            Copy-Item -Path $src -Destination (Join-Path $backupBin $exe) -Force
        }
    }
    if ($FullBackup) {
        $backupData = Join-Path $backupDir "1panel"
        Ensure-Dir $backupData
        Copy-Item -Path (Join-Path $DataDir "*") -Destination $backupData -Recurse -Force
    } else {
        foreach ($sub in @("conf", "db")) {
            $srcDir = Join-Path $DataDir $sub
            if (Test-Path $srcDir) {
                $dstDir = Join-Path (Join-Path $backupDir "1panel") $sub
                Ensure-Dir $dstDir
                Copy-Item -Path (Join-Path $srcDir "*") -Destination $dstDir -Recurse -Force
            }
        }
    }
    Write-Log "backup created: $backupDir"
    return $backupDir
}

function Resolve-Package {
    param([string]$ExtractDir)
    # Returns a hashtable with Core, Agent, ZipName resolved.
    $scriptDir = $script:ScriptDir
    $result = @{ Core = ""; Agent = ""; ZipName = "" }

    $localCore  = Join-Path $scriptDir "1panel-core.exe"
    $localAgent = Join-Path $scriptDir "1panel-agent.exe"

    if (Test-Blank $ZipPath) {
        if ((Test-Path $localCore) -and (Test-Path $localAgent)) {
            $result.Core = $localCore
            $result.Agent = $localAgent
            Write-Log "using bare binaries next to script"
            return $result
        }
        $zip = Get-ChildItem -Path $scriptDir -Filter "1panel-*-windows-amd64.zip" -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending | Select-Object -First 1
        if ($null -eq $zip) {
            throw "no package found: pass -ZipPath or place binaries/zip next to this script"
        }
        $ZipPath = $zip.FullName
    }

    if (-not (Test-Path $ZipPath)) {
        throw "package not found: $ZipPath"
    }
    $result.ZipName = [System.IO.Path]::GetFileName($ZipPath)
    Ensure-Dir $ExtractDir
    Expand-ZipCompat -ZipFile $ZipPath -DestDir $ExtractDir
    $core = Get-ChildItem -Path $ExtractDir -Filter "1panel-core.exe" -Recurse | Select-Object -First 1
    $agent = Get-ChildItem -Path $ExtractDir -Filter "1panel-agent.exe" -Recurse | Select-Object -First 1
    if ($null -eq $core -or $null -eq $agent) {
        throw "extracted package missing binaries: $ZipPath"
    }
    $result.Core = $core.FullName
    $result.Agent = $agent.FullName
    Write-Log "extracted package: $ZipPath"
    return $result
}

function Replace-Binary {
    param(
        [string]$SourcePath,
        [string]$DestPath
    )
    $tempDest = "$DestPath.new"
    Copy-Item -Path $SourcePath -Destination $tempDest -Force
    Move-Item -Path $tempDest -Destination $DestPath -Force
}

function Invoke-Upgrade {
    param([string]$Root)

    $binDir = Join-Path $Root "bin"
    $dataDir = Join-Path $Root "1panel"
    $confEnv = Join-Path $dataDir "conf\1pctl.env"
    $dbPath = Join-Path $dataDir "db\agent.db"

    if (-not (Test-Path (Join-Path $binDir "1panel-core.exe"))) {
        throw "not a 1Panel install dir (missing bin\1panel-core.exe): $Root"
    }
    if (-not (Test-Path (Join-Path $binDir "1panel-agent.exe"))) {
        throw "not a 1Panel install dir (missing bin\1panel-agent.exe): $Root"
    }
    if (-not (Test-Path $confEnv)) {
        throw "not a 1Panel install dir (missing 1panel\conf\1pctl.env): $Root"
    }

    $extractDir = Join-Path $env:TEMP ("1panel-upgrade-" + [System.Guid]::NewGuid().ToString("N"))
    $newVersion = "unknown"
    try {
        $pkg = Resolve-Package -ExtractDir $extractDir
        $newVersion = Get-PackageVersion -ExtractDir $extractDir -ZipFileName $pkg.ZipName
        Write-Log "install dir: $Root"
        Write-Log "new version: $newVersion"

        Stop-PanelService -Root $Root -ServiceName $AgentServiceName
        Stop-PanelService -Root $Root -ServiceName $CoreServiceName
        Wait-ProcessExit -BinDir $binDir -TimeoutSeconds 60 | Out-Null

        foreach ($exe in @("1panel-core.exe", "1panel-agent.exe")) {
            $target = Join-Path $binDir $exe
            if (-not (Test-FileUnlocked -Path $target)) {
                throw "binary is locked, cannot replace: $target"
            }
        }

        $backupDir = Backup-Current -Root $Root -BinDir $binDir -DataDir $dataDir

        Replace-Binary -SourcePath $pkg.Core -DestPath (Join-Path $binDir "1panel-core.exe")
        Replace-Binary -SourcePath $pkg.Agent -DestPath (Join-Path $binDir "1panel-agent.exe")
        Write-Log "binaries replaced"

        Invoke-DbFix -Root $Root -DbPath $dbPath
        Set-PanelVersion -Root $Root -ConfEnv $confEnv -DataDir $dataDir -Version $newVersion

        Start-PanelService -Root $Root -ServiceName $CoreServiceName
        Start-PanelService -Root $Root -ServiceName $AgentServiceName
        Wait-ServiceState -ServiceName $CoreServiceName -DesiredState "Running" -TimeoutSeconds 60 | Out-Null
        Wait-ServiceState -ServiceName $AgentServiceName -DesiredState "Running" -TimeoutSeconds 60 | Out-Null

        Write-Log "===== upgrade summary ====="
        Write-Log "install dir : $Root"
        Write-Log "new version : $newVersion"
        if (Test-Blank $backupDir) {
            Write-Log "backup dir  : <skipped>"
        } else {
            Write-Log "backup dir  : $backupDir"
        }
        Write-Log "core service: $((Get-Service -Name $CoreServiceName -ErrorAction SilentlyContinue).Status)"
        Write-Log "agent service: $((Get-Service -Name $AgentServiceName -ErrorAction SilentlyContinue).Status)"
        Write-Log "done"
    } finally {
        if (Test-Path $extractDir) {
            Remove-Item -LiteralPath $extractDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-Rollback {
    param([string]$Root)

    $binDir = Join-Path $Root "bin"
    $dataDir = Join-Path $Root "1panel"
    $backupRoot = Join-Path $Root "upgrade-backup"
    if (-not (Test-Path $backupRoot)) {
        throw "no backup dir: $backupRoot"
    }
    $latest = Get-ChildItem -Path $backupRoot -ErrorAction SilentlyContinue |
        Where-Object { $_.PSIsContainer } | Sort-Object Name -Descending | Select-Object -First 1
    if ($null -eq $latest) {
        throw "no backup found under: $backupRoot"
    }
    Write-Log "rolling back from: $($latest.FullName)"

    Stop-PanelService -Root $Root -ServiceName $AgentServiceName
    Stop-PanelService -Root $Root -ServiceName $CoreServiceName
    Wait-ProcessExit -BinDir $binDir -TimeoutSeconds 60 | Out-Null

    foreach ($exe in @("1panel-core.exe", "1panel-agent.exe")) {
        $src = Join-Path (Join-Path $latest.FullName "bin") $exe
        if (Test-Path $src) {
            Replace-Binary -SourcePath $src -DestPath (Join-Path $binDir $exe)
        }
    }

    if ($RestoreData) {
        $answer = ""
        $answer = Read-Host "Restore conf + db from backup? This overwrites current data (type YES to confirm)"
        if ($answer -eq "YES") {
            foreach ($sub in @("conf", "db")) {
                $srcDir = Join-Path (Join-Path $latest.FullName "1panel") $sub
                if (Test-Path $srcDir) {
                    $dstDir = Join-Path $dataDir $sub
                    Ensure-Dir $dstDir
                    Copy-Item -Path (Join-Path $srcDir "*") -Destination $dstDir -Recurse -Force
                }
            }
            Write-Log "data restored from: $($latest.FullName)"
        } else {
            Write-Log "data restore cancelled; binaries only"
        }
    }

    Start-PanelService -Root $Root -ServiceName $CoreServiceName
    Start-PanelService -Root $Root -ServiceName $AgentServiceName
    Wait-ServiceState -ServiceName $CoreServiceName -DesiredState "Running" -TimeoutSeconds 60 | Out-Null
    Wait-ServiceState -ServiceName $AgentServiceName -DesiredState "Running" -TimeoutSeconds 60 | Out-Null

    Write-Log "===== rollback summary ====="
    Write-Log "install dir  : $Root"
    Write-Log "restored from: $($latest.FullName)"
    Write-Log "data restored: $RestoreData"
    Write-Log "done"
}

function Test-InstallDir {
    param([string]$Dir)
    return ($Dir -and (Test-Path (Join-Path $Dir "bin\1panel-core.exe")))
}

function Find-InstallDirFromServices {
    # Derive the install dir from the registered panel services: their binPath
    # points at the WinSW wrapper (<InstallDir>\service\<name>.exe) or the exe
    # itself, so walk up from there until we find bin\1panel-core.exe.
    foreach ($svc in @($CoreServiceName, $AgentServiceName)) {
        $wmi = Get-WmiObject -Class Win32_Service -Filter "Name='$svc'" -ErrorAction SilentlyContinue
        if ($null -eq $wmi -or [string]::IsNullOrEmpty($wmi.PathName)) { continue }
        $raw = $wmi.PathName.Trim()
        if ($raw.StartsWith('"')) {
            $end = $raw.IndexOf('"', 1)
            if ($end -gt 1) { $exe = $raw.Substring(1, $end - 1) } else { $exe = $raw }
        } else {
            $exe = ($raw -split '\s+')[0]
        }
        $dir = Split-Path $exe -Parent
        for ($i = 0; $i -lt 4 -and $dir; $i++) {
            if (Test-InstallDir $dir) { return $dir }
            $dir = Split-Path $dir -Parent
        }
    }
    return $null
}

Assert-Administrator

$explicitInstallDir = $PSBoundParameters.ContainsKey('InstallDir')
$InstallDir = $InstallDir.Trim()
if ($InstallDir) {
    $InstallDir = [System.IO.Path]::GetFullPath($InstallDir)
}

if (-not (Test-InstallDir $InstallDir) -and -not $explicitInstallDir) {
    # Default path had no install; try to locate it from the running services
    # (the panel is often installed on D:\ or a custom drive, not C:\1Panel).
    $detected = Find-InstallDirFromServices
    if ($detected) {
        Write-Log "auto-detected install dir from services: $detected"
        $InstallDir = $detected
    }
}

if (-not (Test-InstallDir $InstallDir)) {
    if ($explicitInstallDir) {
        throw "no 1Panel install found at -InstallDir '$InstallDir' (expected bin\1panel-core.exe there)"
    }
    throw "could not locate the 1Panel install directory (tried C:\1Panel and the registered services). Re-run with -InstallDir pointing at the folder that contains bin\1panel-core.exe, e.g.:  .\upgrade.ps1 -InstallDir D:\1Panel"
}

if ($Rollback) {
    Invoke-Rollback -Root $InstallDir
} else {
    Invoke-Upgrade -Root $InstallDir
}
