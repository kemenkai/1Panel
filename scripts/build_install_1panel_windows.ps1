param(
    [string]$InstallDir = "C:\1Panel",
    [string]$PanelPort = "9999",
    [string]$AgentPort = "9998",
    [string]$BindAddress = "0.0.0.0",
    [string]$PanelMode = "dev",
    [string]$PanelVersion = "",
    [string]$PanelUsername = "admin",
    [string]$PanelPassword = "admin123",
    [string]$PanelLanguage = "zh",
    [string]$PanelEdition = "cn",
    [string]$PanelEntrance = "",
    [string]$SSLMode = "disable",
    [string]$WinSWPath = "",
    [switch]$SkipFrontend,
    [switch]$SkipNpmInstall,
    [switch]$SkipBuild,
    [switch]$SkipServiceInstall
)

$ErrorActionPreference = "Stop"

function Write-Log {
    param([string]$Message)
    Write-Host "[1Panel WindowsBuild] $Message"
}

function Resolve-CommandPath {
    param([string]$Name)

    $candidates = @()
    if (-not [System.IO.Path]::HasExtension($Name)) {
        $candidates += @("$Name.cmd", "$Name.exe", "$Name.bat")
    }
    $candidates += $Name

    foreach ($candidate in $candidates) {
        $command = Get-Command $candidate -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($command -and $command.Source) {
            return $command.Source
        }
    }

    throw "missing command: $Name"
}

function Assert-Command {
    param([string]$Name)
    [void](Resolve-CommandPath -Name $Name)
}

function Resolve-PanelVersion {
    param([string]$RequestedVersion)

    if (-not [string]::IsNullOrWhiteSpace($RequestedVersion)) {
        return $RequestedVersion
    }

    if (-not [string]::IsNullOrWhiteSpace($env:PANEL_VERSION)) {
        return $env:PANEL_VERSION
    }

    $gitCmd = Get-Command git.exe -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($gitCmd -and $gitCmd.Source) {
        try {
            $versionText = & $gitCmd.Source -C $RepoDir describe --tags --abbrev=0 2>$null
            if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($versionText)) {
                return $versionText.Trim()
            }
        }
        catch {}
    }

    $installedEnv = "C:\1Panel\1panel\conf\1pctl.env"
    if (Test-Path $installedEnv) {
        $versionLine = Get-Content -Path $installedEnv | Where-Object { $_ -like "ORIGINAL_VERSION=*" } | Select-Object -First 1
        if ($versionLine) {
            return ($versionLine -replace "^ORIGINAL_VERSION=", "").Trim()
        }
    }

    return "v0.0.0-dev"
}

function Invoke-External {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory = ""
    )

    Write-Log "$FilePath $($Arguments -join ' ')"
    $originalLocation = Get-Location
    try {
        if (-not [string]::IsNullOrWhiteSpace($WorkingDirectory)) {
            Set-Location $WorkingDirectory
        }
        & $FilePath @Arguments
        $exitCode = $LASTEXITCODE
        if ($null -eq $exitCode) {
            $exitCode = 0
        }
        if ($exitCode -ne 0) {
            throw "command failed: $FilePath $($Arguments -join ' ')"
        }
    }
    finally {
        if (-not [string]::IsNullOrWhiteSpace($WorkingDirectory)) {
            Set-Location $originalLocation
        }
    }
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
  level: $(Convert-ToYamlSingleQuoted 'debug')
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

function Install-WinSWService {
    param(
        [string]$ServiceName,
        [string]$SourceWinSW,
        [string]$ServiceDirPath
    )

    $wrapperExe = Join-Path $ServiceDirPath "$ServiceName.exe"
    Copy-Item -Path $SourceWinSW -Destination $wrapperExe -Force
    Invoke-External -FilePath $wrapperExe -Arguments @("install")
    Invoke-External -FilePath $wrapperExe -Arguments @("start")
}

function Build-Frontend {
    if ($SkipFrontend) {
        Write-Log "skip frontend build"
        return
    }
    $npmCmd = Resolve-CommandPath -Name "npm"
    $frontendDir = Join-Path $RepoDir "frontend"
    if (-not $SkipNpmInstall) {
        Invoke-External -FilePath $npmCmd -Arguments @("install") -WorkingDirectory $frontendDir
    }
    Invoke-External -FilePath $npmCmd -Arguments @("run", "build:pro") -WorkingDirectory $frontendDir
}

function Build-Binaries {
    if ($SkipBuild) {
        Write-Log "skip go build"
        return
    }

    $goCmd = Resolve-CommandPath -Name "go"
    $buildDir = Join-Path $RepoDir "build\windows"
    Ensure-Dir $buildDir

    $coreArgs = @("build", "-trimpath", "-ldflags=-s -w", "-o", (Join-Path $buildDir "1panel-core.exe"), "./cmd/server/main.go")
    $agentArgs = @("build", "-trimpath", "-ldflags=-s -w", "-o", (Join-Path $buildDir "1panel-agent.exe"), "./cmd/server/main.go")

    $originalGoos = $env:GOOS
    $originalCgoEnabled = $env:CGO_ENABLED
    try {
        $env:GOOS = "windows"
        $env:CGO_ENABLED = "0"
        Invoke-External -FilePath $goCmd -Arguments $coreArgs -WorkingDirectory (Join-Path $RepoDir "core")
        Invoke-External -FilePath $goCmd -Arguments $agentArgs -WorkingDirectory (Join-Path $RepoDir "agent")
    }
    finally {
        $env:GOOS = $originalGoos
        $env:CGO_ENABLED = $originalCgoEnabled
    }
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoDir = Split-Path -Parent $ScriptDir
$PanelVersion = Resolve-PanelVersion -RequestedVersion $PanelVersion

$PanelPort = Validate-PanelPort -Value $PanelPort
$PanelUsername = Validate-PanelUsername -Value $PanelUsername
$PanelPassword = Validate-PanelPassword -Value $PanelPassword
$InstallDir = Validate-EnvValue -Label "install dir" -Value $InstallDir
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
$BuildDir = Join-Path $RepoDir "build\windows"

Ensure-Dir $InstallDir
Ensure-Dir $DataDir
Ensure-Dir $ConfigDir
Ensure-Dir $BinDir
Ensure-Dir $ServiceDir
Ensure-Dir $LogDir
Ensure-Dir (Join-Path $DataDir "db")
Ensure-Dir (Join-Path $DataDir "tmp")

Build-Frontend
Build-Binaries

Copy-Item -Path (Join-Path $BuildDir "1panel-core.exe") -Destination (Join-Path $BinDir "1panel-core.exe") -Force
Copy-Item -Path (Join-Path $BuildDir "1panel-agent.exe") -Destination (Join-Path $BinDir "1panel-agent.exe") -Force

Write-AppYaml -ConfigDirPath $ConfigDir
Write-ParamFile -ConfigDirPath $ConfigDir
Write-WinSWXml -ServiceName "1panel-core-service" -ExecutablePath (Join-Path $BinDir "1panel-core.exe") -ServiceDirPath $ServiceDir -LogDirPath $LogDir
Write-WinSWXml -ServiceName "1panel-agent-service" -ExecutablePath (Join-Path $BinDir "1panel-agent.exe") -ServiceDirPath $ServiceDir -LogDirPath $LogDir

if (-not [string]::IsNullOrWhiteSpace($WinSWPath) -and -not $SkipServiceInstall) {
    if (-not (Test-Path $WinSWPath)) {
        throw "WinSW.exe not found: $WinSWPath"
    }
    Install-WinSWService -ServiceName "1panel-core-service" -SourceWinSW $WinSWPath -ServiceDirPath $ServiceDir
    Install-WinSWService -ServiceName "1panel-agent-service" -SourceWinSW $WinSWPath -ServiceDirPath $ServiceDir
} else {
    Write-Log "WinSW path not provided or service install skipped; generated service XML only."
}

Write-Log "done"
Write-Log "install dir: $InstallDir"
Write-Log "config: $(Join-Path $ConfigDir 'app.yaml')"
Write-Log "param file: $(Join-Path $ConfigDir '1pctl.env')"
Write-Log "service xml dir: $ServiceDir"
