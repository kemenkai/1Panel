[Version]
Class=IEXPRESS
SEDVersion=3

[Options]
PackagePurpose=InstallApp
ShowInstallProgramWindow=1
HideExtractAnimation=1
UseLongFileName=1
InsideCompressed=0
CAB_FixedSize=0
CAB_ResvCodeSigning=0
RebootMode=N
InstallPrompt=
DisplayLicense=
FinishMessage=1Panel installation completed.
TargetName=__TARGET_NAME__
FriendlyName=1Panel Windows Installer __APP_VERSION__
AppLaunched=cmd /c powershell.exe -ExecutionPolicy Bypass -File install.ps1
PostInstallCmd=<None>
AdminQuietInstCmd=cmd /c powershell.exe -ExecutionPolicy Bypass -File install.ps1
UserQuietInstCmd=cmd /c powershell.exe -ExecutionPolicy Bypass -File install.ps1
SourceFiles=SourceFiles

[SourceFiles]
SourceFiles0=__PACKAGE_ROOT__

[SourceFiles0]
__SOURCE_FILE_KEYS__

[Strings]
__STRING_FILE_KEYS__
