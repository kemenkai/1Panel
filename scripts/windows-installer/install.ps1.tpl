param(
    [string]$InstallDir = "__INSTALL_DIR__",
    [string]$PanelPort = "9999",
    [string]$BindAddress = "0.0.0.0",
    [string]$PanelMode = "release",
    [string]$PanelVersion = "__APP_VERSION__",
    [string]$PanelUsername = "admin",
    [string]$PanelPassword = "admin123",
    [string]$PanelLanguage = "zh",
    [string]$PanelEdition = "cn",
    [string]$PanelEntrance = "",
    [string]$SSLMode = "disable",
    [string]$WinSWPath = "",
    [switch]$SkipServiceInstall
)

$ErrorActionPreference = "Stop"

function Write-Log {
    param([string]$Message)
    Write-Host "[1Panel WindowsInstaller] $Message"
}

function Ensure-Dir {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Convert-ToYamlSingleQuoted {
    param([AllowEmptyString()][string]$Value)
    return "'" + ($Value -replace "'", "''") + "'"
}

function Validate-PanelPort {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return "9999"
    }
    $port = 0
    if (-not [int]::TryParse($Value, [ref]$port)) {
        throw "invalid panel port: $Value"
    }
    if ($port -lt 1 -or $port -gt 65535) {
        throw "panel port out of range: $Value"
    }
    return [string]$port
}

function Validate-PanelUsername {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        throw "panel username cannot be empty"
    }
    if ($Value.Length -lt 3 -or $Value.Length -gt 32) {
        throw "panel username must be 3-32 characters"
    }
    if ($Value -notmatch '^[A-Za-z0-9._@-]+$') {
        throw "panel username contains unsupported characters"
    }
    return $Value
}

function Validate-PanelPassword {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        throw "panel password cannot be empty"
    }
    if ($Value.Length -lt 6 -or $Value.Length -gt 64) {
        throw "panel password must be 6-64 characters"
    }
    if ($Value -notmatch '^[A-Za-z0-9._!@#$%^&*()\-]+$') {
        throw "panel password contains unsupported characters"
    }
    return $Value
}

function Validate-EnvValue {
    param(
        [string]$Label,
        [string]$Value
    )

    if ($Value.Contains("`r") -or $Value.Contains("`n") -or $Value.Contains("=")) {
        throw "$Label contains unsupported characters"
    }
    return $Value
}

function Copy-FileSafe {
    param(
        [string]$SourcePath,
        [string]$DestinationPath
    )
    if (-not (Test-Path $SourcePath)) {
        return
    }
    $sourceFull = [System.IO.Path]::GetFullPath($SourcePath)
    $destFull = [System.IO.Path]::GetFullPath($DestinationPath)
    if ($sourceFull -ieq $destFull) {
        return
    }
    $parentDir = Split-Path -Parent $DestinationPath
    if (-not [string]::IsNullOrWhiteSpace($parentDir)) {
        Ensure-Dir $parentDir
    }
    Copy-Item -Path $SourcePath -Destination $DestinationPath -Force
}

function Copy-DirContentsSafe {
    param(
        [string]$SourceDir,
        [string]$DestinationDir
    )
    if (-not (Test-Path $SourceDir)) {
        return
    }
    $sourceFull = [System.IO.Path]::GetFullPath($SourceDir)
    $destFull = [System.IO.Path]::GetFullPath($DestinationDir)
    if ($sourceFull -ieq $destFull) {
        return
    }
    Ensure-Dir $DestinationDir
    Copy-Item -Path (Join-Path $SourceDir "*") -Destination $DestinationDir -Recurse -Force
}

function Expand-BundledJdkIfPresent {
    param(
        [string]$RuntimeDirPath,
        [string]$DestinationDir
    )
    if (-not (Test-Path $RuntimeDirPath)) {
        return
    }
    $jdkZip = Get-ChildItem -Path $RuntimeDirPath -Filter "*.zip" | Select-Object -First 1
    if ($null -eq $jdkZip) {
        return
    }
    $jdkDirName = [System.IO.Path]::GetFileNameWithoutExtension($jdkZip.Name)
    $jdkTargetDir = Join-Path $DestinationDir $jdkDirName
    $javaExe = Join-Path $jdkTargetDir "bin\\java.exe"
    if (Test-Path $javaExe) {
        return
    }
    if (Test-Path $jdkTargetDir) {
        Remove-Item -LiteralPath $jdkTargetDir -Recurse -Force
    }
    Expand-Archive -LiteralPath $jdkZip.FullName -DestinationPath $DestinationDir -Force
}

function Write-AppYaml {
    param([string]$ConfigDirPath)

    $yaml = @"
base:
  install_dir: $(Convert-ToYamlSingleQuoted $InstallDir)
  mode: $(Convert-ToYamlSingleQuoted $PanelMode)
  is_demo: false
  is_offline: false
  is_fxplay: false
  username: $(Convert-ToYamlSingleQuoted $PanelUsername)
  password: $(Convert-ToYamlSingleQuoted $PanelPassword)
  language: $(Convert-ToYamlSingleQuoted $PanelLanguage)
  edition: $(Convert-ToYamlSingleQuoted $PanelEdition)
  version: $(Convert-ToYamlSingleQuoted $PanelVersion)

conn:
  port: $PanelPort
  bindAddress: $(Convert-ToYamlSingleQuoted $BindAddress)
  ipv6: $(Convert-ToYamlSingleQuoted 'Disable')
  ssl: $(Convert-ToYamlSingleQuoted $SSLMode)
  entrance: $(Convert-ToYamlSingleQuoted $PanelEntrance)

log:
  level: $(Convert-ToYamlSingleQuoted 'info')
  timeZone: $(Convert-ToYamlSingleQuoted 'Asia/Shanghai')
  log_name: $(Convert-ToYamlSingleQuoted '1Panel')
  log_suffix: $(Convert-ToYamlSingleQuoted '.log')
  max_backup: 10
"@
    Set-Content -Path (Join-Path $ConfigDirPath "app.yaml") -Value $yaml -Encoding UTF8
}

function Write-ParamFile {
    param([string]$ConfigDirPath)

    $envContent = @"
BASE_DIR=$InstallDir
ORIGINAL_PORT=$PanelPort
ORIGINAL_VERSION=$PanelVersion
ORIGINAL_USERNAME=$PanelUsername
ORIGINAL_PASSWORD=$PanelPassword
ORIGINAL_ENTRANCE=$PanelEntrance
LANGUAGE=$PanelLanguage
PANEL_EDITION=$PanelEdition
CHANGE_USER_INFO=
"@
    Set-Content -Path (Join-Path $ConfigDirPath "1pctl.env") -Value $envContent -Encoding ASCII
}

function Write-WinSWXml {
    param(
        [string]$ServiceName,
        [string]$ExecutablePath,
        [string]$ServiceDirPath,
        [string]$LogDirPath
    )

    $xmlPath = Join-Path $ServiceDirPath "$ServiceName.xml"
    $xml = @"
<service>
  <id>$ServiceName</id>
  <name>$ServiceName</name>
  <description>$ServiceName</description>
  <executable>$ExecutablePath</executable>
  <workingdirectory>$InstallDir</workingdirectory>
  <logpath>$LogDirPath</logpath>
  <log mode="roll" />
  <stoptimeout>15sec</stoptimeout>
  <onfailure action="restart" delay="5 sec" />
</service>
"@
    Set-Content -Path $xmlPath -Value $xml -Encoding UTF8
}

function Invoke-WinSW {
    param(
        [string]$WrapperPath,
        [string]$Operation
    )
    if (-not (Test-Path $WrapperPath)) {
        return
    }
    & $WrapperPath $Operation | Out-Null
    $exitCode = $LASTEXITCODE
    if ($null -eq $exitCode) {
        $exitCode = 0
    }
    if ($exitCode -ne 0 -and $Operation -eq "install") {
        throw "WinSW operation failed: $Operation"
    }
}

function Reinstall-WinSWService {
    param(
        [string]$ServiceName,
        [string]$SourceWinSW,
        [string]$ServiceDirPath
    )

    $wrapperExe = Join-Path $ServiceDirPath "$ServiceName.exe"
    if (Test-Path $wrapperExe) {
        try { Invoke-WinSW -WrapperPath $wrapperExe -Operation "stop" } catch {}
        try { Invoke-WinSW -WrapperPath $wrapperExe -Operation "uninstall" } catch {}
    }
    Copy-Item -Path $SourceWinSW -Destination $wrapperExe -Force
    Invoke-WinSW -WrapperPath $wrapperExe -Operation "install"
    Invoke-WinSW -WrapperPath $wrapperExe -Operation "start"
}

function Save-InstallLocation {
    param([string]$Path)

    $normalizedPath = [System.IO.Path]::GetFullPath($Path.Trim())
    foreach ($registryPath in @("HKLM:\Software\1Panel", "HKCU:\Software\1Panel")) {
        try {
            New-Item -Path $registryPath -Force | Out-Null
            New-ItemProperty -Path $registryPath -Name "InstallDir" -Value $normalizedPath -PropertyType String -Force | Out-Null
            return
        } catch {
            if ($registryPath -eq "HKCU:\Software\1Panel") {
                Write-Log "failed to persist install dir: $($_.Exception.Message)"
            }
        }
    }
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageBinDir = Join-Path $ScriptDir "bin"
$PackageToolsDir = Join-Path $ScriptDir "tools"
$PackageRuntimeDir = Join-Path $ScriptDir "runtime"

$PanelPort = Validate-PanelPort -Value $PanelPort
$PanelUsername = Validate-PanelUsername -Value $PanelUsername
$PanelPassword = Validate-PanelPassword -Value $PanelPassword
$InstallDir = Validate-EnvValue -Label "install dir" -Value $InstallDir
$InstallDir = [System.IO.Path]::GetFullPath($InstallDir.Trim())
$PanelVersion = Validate-EnvValue -Label "panel version" -Value $PanelVersion
$PanelUsername = Validate-EnvValue -Label "panel username" -Value $PanelUsername
$PanelPassword = Validate-EnvValue -Label "panel password" -Value $PanelPassword
$PanelEntrance = Validate-EnvValue -Label "panel entrance" -Value $PanelEntrance
$PanelLanguage = Validate-EnvValue -Label "panel language" -Value $PanelLanguage
$PanelEdition = Validate-EnvValue -Label "panel edition" -Value $PanelEdition

$ConfigDir = Join-Path $InstallDir "1panel\conf"
$DataDir = Join-Path $InstallDir "1panel"
$BinDir = Join-Path $InstallDir "bin"
$ServiceDir = Join-Path $InstallDir "service"
$LogDir = Join-Path $InstallDir "1panel\log"
$ToolsDir = Join-Path $InstallDir "tools"

if (-not (Test-Path (Join-Path $PackageBinDir "1panel-core.exe"))) {
    throw "missing packaged binary: bin\1panel-core.exe"
}
if (-not (Test-Path (Join-Path $PackageBinDir "1panel-agent.exe"))) {
    throw "missing packaged binary: bin\1panel-agent.exe"
}

Ensure-Dir $InstallDir
Ensure-Dir $DataDir
Ensure-Dir $ConfigDir
Ensure-Dir $BinDir
Ensure-Dir $ServiceDir
Ensure-Dir $LogDir
Ensure-Dir $ToolsDir
Ensure-Dir (Join-Path $DataDir "db")
Ensure-Dir (Join-Path $DataDir "tmp")
Save-InstallLocation -Path $InstallDir

Copy-DirContentsSafe -SourceDir $PackageBinDir -DestinationDir $BinDir
Copy-DirContentsSafe -SourceDir $PackageToolsDir -DestinationDir $ToolsDir
Expand-BundledJdkIfPresent -RuntimeDirPath $PackageRuntimeDir -DestinationDir $InstallDir
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "install.cmd") -DestinationPath (Join-Path $InstallDir "install.cmd")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "install-interactive.ps1") -DestinationPath (Join-Path $InstallDir "install-interactive.ps1")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "open-install-dir.cmd") -DestinationPath (Join-Path $InstallDir "open-install-dir.cmd")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "uninstall.cmd") -DestinationPath (Join-Path $InstallDir "uninstall.cmd")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "uninstall-interactive.ps1") -DestinationPath (Join-Path $InstallDir "uninstall-interactive.ps1")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "install.ps1") -DestinationPath (Join-Path $InstallDir "install.ps1")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "uninstall.ps1") -DestinationPath (Join-Path $InstallDir "uninstall.ps1")
Copy-FileSafe -SourcePath (Join-Path $ScriptDir "README.txt") -DestinationPath (Join-Path $InstallDir "README.txt")

if ([string]::IsNullOrWhiteSpace($WinSWPath)) {
    $bundledWinSW = Join-Path $ToolsDir "WinSW.exe"
    if (Test-Path $bundledWinSW) {
        $WinSWPath = $bundledWinSW
    }
}

Write-AppYaml -ConfigDirPath $ConfigDir
Write-ParamFile -ConfigDirPath $ConfigDir
Write-WinSWXml -ServiceName "1panel-core-service" -ExecutablePath (Join-Path $BinDir "1panel-core.exe") -ServiceDirPath $ServiceDir -LogDirPath $LogDir
Write-WinSWXml -ServiceName "1panel-agent-service" -ExecutablePath (Join-Path $BinDir "1panel-agent.exe") -ServiceDirPath $ServiceDir -LogDirPath $LogDir

if (-not [string]::IsNullOrWhiteSpace($WinSWPath) -and -not $SkipServiceInstall) {
    if (-not (Test-Path $WinSWPath)) {
        throw "WinSW.exe not found: $WinSWPath"
    }
    Reinstall-WinSWService -ServiceName "1panel-core-service" -SourceWinSW $WinSWPath -ServiceDirPath $ServiceDir
    Reinstall-WinSWService -ServiceName "1panel-agent-service" -SourceWinSW $WinSWPath -ServiceDirPath $ServiceDir
} else {
    Write-Log "WinSW path not provided or service install skipped; service XML generated only."
}

Write-Log "done"
Write-Log "install dir: $InstallDir"
Write-Log "config: $(Join-Path $ConfigDir 'app.yaml')"
Write-Log "param file: $(Join-Path $ConfigDir '1pctl.env')"
Write-Log "service xml dir: $ServiceDir"
