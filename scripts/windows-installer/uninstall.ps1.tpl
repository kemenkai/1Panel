param(
    [string]$InstallDir = "__INSTALL_DIR__",
    [switch]$RemoveManagedServices,
    [switch]$RemoveInstallDir
)

$ErrorActionPreference = "Stop"

function Remove-ShortcutIfExists {
    param([string]$Path)
    if (Test-Path -LiteralPath $Path) {
        Remove-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-WinSW {
    param(
        [string]$WrapperPath,
        [string]$Operation
    )
    if (-not (Test-Path $WrapperPath)) {
        return
    }
    try {
        & $WrapperPath $Operation | Out-Null
    } catch {}
}

function Get-ManagedServiceDirectories {
    param([string]$ServiceDir)

    if (-not (Test-Path -LiteralPath $ServiceDir)) {
        return @()
    }
    return @(Get-ChildItem -Path $ServiceDir -Directory -ErrorAction SilentlyContinue | Sort-Object Name)
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

function Resolve-EffectiveInstallDir {
    param(
        [string]$RequestedInstallDir,
        [string]$ScriptDir,
        [bool]$InstallDirWasExplicit
    )

    if ($InstallDirWasExplicit -and -not [string]::IsNullOrWhiteSpace($RequestedInstallDir)) {
        return [System.IO.Path]::GetFullPath($RequestedInstallDir.Trim())
    }
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
    return [System.IO.Path]::GetFullPath($RequestedInstallDir.Trim())
}

function Clear-InstallLocation {
    param([string]$Path)

    $normalizedPath = [System.IO.Path]::GetFullPath($Path.Trim())
    foreach ($registryPath in @("HKLM:\Software\1Panel", "HKCU:\Software\1Panel")) {
        try {
            $currentValue = (Get-ItemProperty -Path $registryPath -Name "InstallDir" -ErrorAction Stop).InstallDir
            if (-not [string]::IsNullOrWhiteSpace($currentValue) -and
                ([System.IO.Path]::GetFullPath($currentValue.Trim()) -ieq $normalizedPath)) {
                Remove-ItemProperty -Path $registryPath -Name "InstallDir" -Force -ErrorAction SilentlyContinue
            }
        } catch {}
    }
}

function Remove-ManagedServiceArtifacts {
    param([string]$ServiceDir)

    foreach ($serviceDirItem in Get-ManagedServiceDirectories -ServiceDir $ServiceDir) {
        $wrapperPath = Join-Path $serviceDirItem.FullName ($serviceDirItem.Name + ".exe")
        Invoke-WinSW -WrapperPath $wrapperPath -Operation "stop"
        Invoke-WinSW -WrapperPath $wrapperPath -Operation "uninstall"
        Remove-Item -LiteralPath $serviceDirItem.FullName -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$InstallDir = Resolve-EffectiveInstallDir `
    -RequestedInstallDir $InstallDir `
    -ScriptDir $ScriptDir `
    -InstallDirWasExplicit ($PSBoundParameters.ContainsKey("InstallDir"))

$ServiceDir = Join-Path $InstallDir "service"
$coreWrapper = Join-Path $ServiceDir "1panel-core-service.exe"
$agentWrapper = Join-Path $ServiceDir "1panel-agent-service.exe"

Invoke-WinSW -WrapperPath $coreWrapper -Operation "stop"
Invoke-WinSW -WrapperPath $coreWrapper -Operation "uninstall"
Invoke-WinSW -WrapperPath $agentWrapper -Operation "stop"
Invoke-WinSW -WrapperPath $agentWrapper -Operation "uninstall"

if ($RemoveManagedServices) {
    Remove-ManagedServiceArtifacts -ServiceDir $ServiceDir
}

$commonDesktop = [Environment]::GetFolderPath("CommonDesktopDirectory")
if (-not [string]::IsNullOrWhiteSpace($commonDesktop)) {
    Remove-ShortcutIfExists -Path (Join-Path $commonDesktop "1Panel.url")
    Remove-ShortcutIfExists -Path (Join-Path $commonDesktop "1Panel Install Directory.lnk")
    Remove-ShortcutIfExists -Path (Join-Path $commonDesktop "1Panel Uninstall.lnk")
}

Clear-InstallLocation -Path $InstallDir

if ($RemoveInstallDir -and (Test-Path $InstallDir)) {
    Remove-Item -LiteralPath $InstallDir -Recurse -Force
}
