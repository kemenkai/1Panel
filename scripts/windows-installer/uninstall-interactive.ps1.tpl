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

function LooksLikeInstallDir {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $false
    }
    try {
        $fullPath = [System.IO.Path]::GetFullPath($Path.Trim())
    } catch {
        return $false
    }
    return (Test-Path -LiteralPath (Join-Path $fullPath "1panel\conf\1pctl.env")) -or
        (Test-Path -LiteralPath (Join-Path $fullPath "service\1panel-core-service.exe"))
}

function Get-InstallDirFromRegistry {
    foreach ($registryPath in @("HKLM:\Software\1Panel", "HKCU:\Software\1Panel")) {
        try {
            $installDir = (Get-ItemProperty -Path $registryPath -Name "InstallDir" -ErrorAction Stop).InstallDir
            if (-not [string]::IsNullOrWhiteSpace($installDir)) {
                return [System.IO.Path]::GetFullPath($installDir.Trim())
            }
        } catch {}
    }
    return ""
}

function Get-ServiceExecutablePath {
    param([string]$PathName)

    if ([string]::IsNullOrWhiteSpace($PathName)) {
        return ""
    }
    $trimmed = $PathName.Trim()
    if ($trimmed.StartsWith('"')) {
        $endQuote = $trimmed.IndexOf('"', 1)
        if ($endQuote -gt 1) {
            return $trimmed.Substring(1, $endQuote - 1)
        }
    }
    $match = [regex]::Match($trimmed, '^[^\s]+\.exe')
    if ($match.Success) {
        return $match.Value
    }
    return ""
}

function Get-InstallDirFromService {
    foreach ($serviceName in @("1panel-core-service", "1panel-agent-service")) {
        try {
            $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='$serviceName'" -ErrorAction Stop
            $executablePath = Get-ServiceExecutablePath -PathName $service.PathName
            if ([string]::IsNullOrWhiteSpace($executablePath)) {
                continue
            }
            $serviceDir = Split-Path -Parent $executablePath
            $installDir = Split-Path -Parent $serviceDir
            if (LooksLikeInstallDir -Path $installDir) {
                return [System.IO.Path]::GetFullPath($installDir)
            }
        } catch {}
    }
    return ""
}

function Get-DefaultInstallDir {
    param(
        [string]$ScriptDir,
        [string]$PackagedDefault
    )

    if (LooksLikeInstallDir -Path $ScriptDir) {
        return [System.IO.Path]::GetFullPath($ScriptDir)
    }
    $registryInstallDir = Get-InstallDirFromRegistry
    if (-not [string]::IsNullOrWhiteSpace($registryInstallDir)) {
        return $registryInstallDir
    }
    $serviceInstallDir = Get-InstallDirFromService
    if (-not [string]::IsNullOrWhiteSpace($serviceInstallDir)) {
        return $serviceInstallDir
    }
    return [System.IO.Path]::GetFullPath($PackagedDefault.Trim())
}

function Get-ManagedServiceDirectories {
    param([string]$InstallDir)

    $serviceDir = Join-Path $InstallDir "service"
    if (-not (Test-Path -LiteralPath $serviceDir)) {
        return @()
    }
    return @(Get-ChildItem -Path $serviceDir -Directory -ErrorAction SilentlyContinue | Sort-Object Name)
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

if (-not (Test-IsAdministrator)) {
    Write-Host ""
    Write-Host "Requesting administrator privileges..." -ForegroundColor Yellow
    Restart-Elevated
    exit 0
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$defaultInstallDir = Get-DefaultInstallDir -ScriptDir $scriptDir -PackagedDefault "__INSTALL_DIR__"

Write-Host ""
Write-Host "1Panel Windows Uninstaller" -ForegroundColor Cyan
Write-Host "Press Enter to accept the default value shown in brackets." -ForegroundColor DarkGray
Write-Host ""

$installDir = Read-ValidatedValue -Prompt "Install directory" -DefaultValue $defaultInstallDir -Resolver { param($v) Resolve-InstallDir $v }
$configPath = Join-Path $installDir "1panel\conf\1pctl.env"
$managedServiceDirs = Get-ManagedServiceDirectories -InstallDir $installDir

Write-Host ""
Write-Host "Uninstall summary:" -ForegroundColor Cyan
Write-Host "  Install directory: $installDir"
if (Test-Path -LiteralPath $configPath) {
    Write-Host "  Existing config:   $configPath"
} else {
    Write-Host "  Existing config:   not found" -ForegroundColor Yellow
}
if ($managedServiceDirs.Count -gt 0) {
    Write-Host "  Managed services:  $($managedServiceDirs.Count)"
} else {
    Write-Host "  Managed services:  not found"
}
Write-Host ""

if (-not (Read-YesNo -Prompt "Continue uninstall? (Y/N)" -DefaultValue "N")) {
    Write-Host ""
    Write-Host "Uninstall cancelled."
    exit 1
}

$removeManagedServices = $false
if ($managedServiceDirs.Count -gt 0) {
    $removeManagedServices = Read-YesNo -Prompt "Remove managed Windows services created by 1Panel? (Y/N)" -DefaultValue "Y"
}

$uninstallScript = Join-Path $scriptDir "uninstall.ps1"
$uninstallArgs = @{
    InstallDir = $installDir
}
if ($removeManagedServices) {
    $uninstallArgs.RemoveManagedServices = $true
}
& $uninstallScript @uninstallArgs

Write-Host ""
Write-Host "1Panel uninstall completed." -ForegroundColor Green
Write-Host ""
Read-Host "Press Enter to close"
