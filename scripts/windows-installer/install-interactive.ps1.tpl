param()

$ErrorActionPreference = "Stop"

function Read-InputOrDefault {
    param(
        [string]$Prompt,
        [string]$DefaultValue
    )

    $suffix = ""
    if (-not [string]::IsNullOrWhiteSpace($DefaultValue)) {
        $suffix = " [$DefaultValue]"
    }
    $value = Read-Host "$Prompt$suffix"
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $DefaultValue
    }
    return $value.Trim()
}

function Test-IsAdministrator {
    $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Restart-Elevated {
    $arguments = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", ('"{0}"' -f $PSCommandPath)
    )
    Start-Process -FilePath "powershell.exe" -ArgumentList $arguments -Verb RunAs | Out-Null
}

function Resolve-InstallDir {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        throw "install directory cannot be empty"
    }
    if (-not [System.IO.Path]::IsPathRooted($Value)) {
        throw "install directory must be an absolute path"
    }
    return [System.IO.Path]::GetFullPath($Value.Trim())
}

function Resolve-PanelPort {
    param([string]$Value)

    $port = 0
    if (-not [int]::TryParse($Value, [ref]$port)) {
        throw "panel port must be a number"
    }
    if ($port -lt 1 -or $port -gt 65535) {
        throw "panel port must be between 1 and 65535"
    }
    return [string]$port
}

function Resolve-PanelUsername {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        throw "panel username cannot be empty"
    }
    if ($Value.Length -lt 3 -or $Value.Length -gt 32) {
        throw "panel username must be 3-32 characters"
    }
    if ($Value -notmatch '^[A-Za-z0-9._@-]+$') {
        throw "panel username supports only letters, digits, dot, underscore, at and hyphen"
    }
    return $Value.Trim()
}

function Resolve-PanelPassword {
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
    return $Value.Trim()
}

function Read-ValidatedValue {
    param(
        [string]$Prompt,
        [string]$DefaultValue,
        [scriptblock]$Resolver
    )

    while ($true) {
        try {
            $value = Read-InputOrDefault -Prompt $Prompt -DefaultValue $DefaultValue
            return & $Resolver $value
        } catch {
            Write-Host $_.Exception.Message -ForegroundColor Yellow
        }
    }
}

function Read-YesNo {
    param(
        [string]$Prompt,
        [string]$DefaultValue = "Y"
    )

    while ($true) {
        $value = Read-InputOrDefault -Prompt $Prompt -DefaultValue $DefaultValue
        $normalized = $value.Trim().ToUpperInvariant()
        if ($normalized -in @("Y", "YES")) {
            return $true
        }
        if ($normalized -in @("N", "NO")) {
            return $false
        }
        Write-Host "Please enter Y or N." -ForegroundColor Yellow
    }
}

function Ensure-Dir {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Test-PortInUse {
    param([string]$Port)

    try {
        $listeners = [System.Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().GetActiveTcpListeners()
        return $listeners.Port -contains ([int]$Port)
    } catch {
        return $false
    }
}

function Get-CommonDesktopPath {
    return [Environment]::GetFolderPath("CommonDesktopDirectory")
}

function New-DesktopUrlShortcut {
    param(
        [string]$DesktopPath,
        [string]$FileName,
        [string]$TargetUrl
    )

    $shortcutPath = Join-Path $DesktopPath $FileName
    $content = @"
[InternetShortcut]
URL=$TargetUrl
"@
    Set-Content -Path $shortcutPath -Value $content -Encoding ASCII
}

function New-DesktopFileShortcut {
    param(
        [string]$DesktopPath,
        [string]$FileName,
        [string]$TargetPath,
        [string]$WorkingDirectory
    )

    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut((Join-Path $DesktopPath $FileName))
    $shortcut.TargetPath = $TargetPath
    $shortcut.WorkingDirectory = $WorkingDirectory
    $shortcut.Save()
}

function Get-InstallState {
    param([string]$InstallDir)

    $configPath = Join-Path $InstallDir "1panel\conf\1pctl.env"
    $corePath = Join-Path $InstallDir "bin\1panel-core.exe"
    $agentPath = Join-Path $InstallDir "bin\1panel-agent.exe"
    return [PSCustomObject]@{
        ConfigPath = $configPath
        HasExistingInstall = (Test-Path -LiteralPath $configPath) -or ((Test-Path -LiteralPath $corePath) -and (Test-Path -LiteralPath $agentPath))
    }
}

if (-not (Test-IsAdministrator)) {
    Write-Host ""
    Write-Host "Requesting administrator privileges..." -ForegroundColor Yellow
    Restart-Elevated
    exit 0
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$defaults = @{
    InstallDir = "__INSTALL_DIR__"
    PanelPort = "__PANEL_PORT__"
    PanelUsername = "__PANEL_USERNAME__"
    PanelPassword = "__PANEL_PASSWORD__"
}

Write-Host ""
Write-Host "1Panel Windows Installer" -ForegroundColor Cyan
Write-Host "Press Enter to accept the default value shown in brackets." -ForegroundColor DarkGray
Write-Host ""

$installDir = Read-ValidatedValue -Prompt "Install directory" -DefaultValue $defaults.InstallDir -Resolver { param($v) Resolve-InstallDir $v }
$installState = Get-InstallState -InstallDir $installDir
if ($installState.HasExistingInstall) {
    Write-Host ""
    Write-Host "Existing 1Panel files were detected in the target directory." -ForegroundColor Yellow
    Write-Host "Detected path: $installDir"
    Write-Host ""
    if (-not (Read-YesNo -Prompt "Continue and reinstall into this directory? (Y/N)" -DefaultValue "N")) {
        Write-Host ""
        Write-Host "Installation cancelled."
        exit 1
    }
}

$panelPort = Read-ValidatedValue -Prompt "Panel port" -DefaultValue $defaults.PanelPort -Resolver { param($v) Resolve-PanelPort $v }
if (Test-PortInUse -Port $panelPort) {
    Write-Host ""
    Write-Host "Warning: port $panelPort is already in use on this machine." -ForegroundColor Yellow
    if (-not (Read-YesNo -Prompt "Continue with this port anyway? (Y/N)" -DefaultValue "N")) {
        Write-Host ""
        Write-Host "Please rerun the installer and choose another port."
        exit 1
    }
}

$panelUsername = Read-ValidatedValue -Prompt "Panel username" -DefaultValue $defaults.PanelUsername -Resolver { param($v) Resolve-PanelUsername $v }
$panelPassword = Read-ValidatedValue -Prompt "Panel password" -DefaultValue $defaults.PanelPassword -Resolver { param($v) Resolve-PanelPassword $v }

Write-Host ""
Write-Host "Installation summary:" -ForegroundColor Cyan
Write-Host "  Install directory: $installDir"
Write-Host "  Panel port:        $panelPort"
Write-Host "  Panel username:    $panelUsername"
if ($installState.HasExistingInstall) {
    Write-Host "  Install mode:      Reinstall existing directory"
} else {
    Write-Host "  Install mode:      Fresh install"
}
Write-Host "  Panel URL:         http://127.0.0.1:$panelPort"
Write-Host ""

if (-not (Read-YesNo -Prompt "Continue installation? (Y/N)" -DefaultValue "Y")) {
    Write-Host ""
    Write-Host "Installation cancelled."
    exit 1
}

$createDesktopShortcuts = Read-YesNo -Prompt "Create desktop shortcuts? (Y/N)" -DefaultValue "Y"
$installerLogDir = Join-Path $installDir "installer-logs"
Ensure-Dir $installerLogDir
$installerLogPath = Join-Path $installerLogDir ("install-{0}.log" -f (Get-Date -Format "yyyyMMdd-HHmmss"))
$transcriptStarted = $false

$installScript = Join-Path $scriptDir "install.ps1"
$installArgs = @{
    InstallDir = $installDir
    PanelVersion = "__APP_VERSION__"
    PanelPort = $panelPort
    PanelUsername = $panelUsername
    PanelPassword = $panelPassword
}

try {
    Start-Transcript -Path $installerLogPath -Append | Out-Null
    $transcriptStarted = $true
    & $installScript @installArgs
} finally {
    if ($transcriptStarted) {
        Stop-Transcript | Out-Null
    }
}

if ($createDesktopShortcuts) {
    $commonDesktop = Get-CommonDesktopPath
    if (-not [string]::IsNullOrWhiteSpace($commonDesktop)) {
        Ensure-Dir $commonDesktop
        New-DesktopUrlShortcut -DesktopPath $commonDesktop -FileName "1Panel.url" -TargetUrl ("http://127.0.0.1:{0}" -f $panelPort)
        New-DesktopFileShortcut -DesktopPath $commonDesktop -FileName "1Panel Install Directory.lnk" -TargetPath (Join-Path $installDir "open-install-dir.cmd") -WorkingDirectory $installDir
        New-DesktopFileShortcut -DesktopPath $commonDesktop -FileName "1Panel Uninstall.lnk" -TargetPath (Join-Path $installDir "uninstall.cmd") -WorkingDirectory $installDir
    }
}

Write-Host ""
Write-Host "1Panel install completed." -ForegroundColor Green
Write-Host "Install directory: $installDir"
Write-Host "Panel URL: http://127.0.0.1:$panelPort"
Write-Host "Installer log: $installerLogPath"
Write-Host "Open directory helper: $(Join-Path $installDir 'open-install-dir.cmd')"
Write-Host ""
if (Read-YesNo -Prompt "Open panel in browser now? (Y/N)" -DefaultValue "Y") {
    Start-Process ("http://127.0.0.1:{0}" -f $panelPort) | Out-Null
}
if (Read-YesNo -Prompt "Open install directory now? (Y/N)" -DefaultValue "Y") {
    & (Join-Path $installDir "open-install-dir.cmd")
}
Write-Host ""
Read-Host "Press Enter to close"
