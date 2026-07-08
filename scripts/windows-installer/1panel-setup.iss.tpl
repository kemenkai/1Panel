#define MyAppName "1Panel"
#define MyAppVersion "__APP_VERSION__"
#define MyAppPublisher "1Panel"
#define MyAppURL "https://github.com/1Panel-dev/1Panel"
#define MyDefaultDir "__INSTALL_DIR__"
#define MyPackageRoot "__PACKAGE_ROOT__"
#define MyOutputDir "__OUTPUT_DIR__"
#define MyOutputBaseFilename "__OUTPUT_BASENAME__"

[Setup]
AppId={{A2D5A80A-062D-4A6B-B2A4-9E8A9E6DBA0B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={#MyDefaultDir}
DisableProgramGroupPage=yes
AppVerName={#MyAppName} {#MyAppVersion}
OutputDir={#MyOutputDir}
OutputBaseFilename={#MyOutputBaseFilename}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
UninstallDisplayIcon={app}\bin\1panel-core.exe

[Files]
Source: "{#MyPackageRoot}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\1Panel\Open Install Directory"; Filename: "{app}\open-install-dir.cmd"; WorkingDir: "{app}"

[Run]
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\install.ps1"" -InstallDir ""{app}"" -PanelVersion ""{#MyAppVersion}"""; Flags: runhidden waituntilterminated; StatusMsg: "Installing 1Panel services..."

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\uninstall.ps1"" -InstallDir ""{app}"""; Flags: runhidden waituntilterminated
