<?xml version="1.0" encoding="utf-8"?>
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package
    Name="1Panel"
    Manufacturer="1Panel"
    Version="__MSI_VERSION__"
    UpgradeCode="__UPGRADE_CODE__"
    Scope="perMachine"
  >
    <SummaryInformation Description="1Panel Windows Installer" />
    <MajorUpgrade DowngradeErrorMessage="A newer version of 1Panel is already installed." />
    <MediaTemplate EmbedCab="yes" />

    <Property Id="INSTALLFOLDER" Value="__DEFAULT_INSTALL_DIR__" Secure="yes" />
    <Property Id="WIXUI_INSTALLDIR" Value="INSTALLFOLDER" />
    <Property Id="PANEL_PORT" Value="__DEFAULT_PANEL_PORT__" Secure="yes" />
    <Property Id="PANEL_USERNAME" Value="__DEFAULT_PANEL_USERNAME__" Secure="yes" />
    <Property Id="PANEL_PASSWORD" Value="__DEFAULT_PANEL_PASSWORD__" Secure="yes" Hidden="yes" />
    <WixVariable Id="WixUILicenseRtf" Value="__LICENSE_RTF__" />

    <StandardDirectory Id="ProgramFiles6432Folder">
      <Directory Id="INSTALLFOLDER" Name="1Panel" />
    </StandardDirectory>
    <StandardDirectory Id="ProgramMenuFolder">
      <Directory Id="ProgramMenuDir" Name="1Panel" />
    </StandardDirectory>

    <UI Id="PanelUI">
      <TextStyle Id="WixUI_Font_Normal" FaceName="Tahoma" Size="8" />
      <TextStyle Id="WixUI_Font_Bigger" FaceName="Tahoma" Size="12" />
      <TextStyle Id="WixUI_Font_Title" FaceName="Tahoma" Size="9" Bold="yes" />

      <Property Id="DefaultUIFont" Value="WixUI_Font_Normal" />
      <Property Id="ARPNOMODIFY" Value="1" />

      <DialogRef Id="BrowseDlg" />
      <DialogRef Id="DiskCostDlg" />
      <DialogRef Id="ErrorDlg" />
      <DialogRef Id="FatalError" />
      <DialogRef Id="FilesInUse" />
      <DialogRef Id="MsiRMFilesInUse" />
      <DialogRef Id="PrepareDlg" />
      <DialogRef Id="ProgressDlg" />
      <DialogRef Id="ResumeDlg" />
      <DialogRef Id="UserExit" />
      <DialogRef Id="WelcomeDlg" />
      <DialogRef Id="LicenseAgreementDlg" />
      <DialogRef Id="InstallDirDlg" />
      <DialogRef Id="VerifyReadyDlg" />
      <DialogRef Id="MaintenanceWelcomeDlg" />
      <DialogRef Id="MaintenanceTypeDlg" />

      <Dialog Id="PanelConfigDlg" Width="370" Height="270" Title="1Panel Setup">
        <Control Id="Next" Type="PushButton" X="236" Y="243" Width="56" Height="17" Default="yes" Text="!(loc.WixUINext)" />
        <Control Id="Back" Type="PushButton" X="180" Y="243" Width="56" Height="17" Text="!(loc.WixUIBack)" />
        <Control Id="Cancel" Type="PushButton" X="304" Y="243" Width="56" Height="17" Cancel="yes" Text="!(loc.WixUICancel)">
          <Publish Event="SpawnDialog" Value="CancelDlg" />
        </Control>

        <Control Id="Description" Type="Text" X="25" Y="23" Width="320" Height="15" Transparent="yes" NoPrefix="yes" Text="Configure the initial 1Panel settings." />
        <Control Id="Title" Type="Text" X="15" Y="6" Width="240" Height="15" Transparent="yes" NoPrefix="yes" Text="1Panel Initial Configuration" />
        <Control Id="BannerBitmap" Type="Bitmap" X="0" Y="0" Width="370" Height="44" TabSkip="no" Text="!(loc.InstallDirDlgBannerBitmap)" />
        <Control Id="BannerLine" Type="Line" X="0" Y="44" Width="373" Height="0" />
        <Control Id="BottomLine" Type="Line" X="0" Y="234" Width="373" Height="0" />

        <Control Id="PortLabel" Type="Text" X="20" Y="62" Width="140" Height="14" Text="Panel Port:" />
        <Control Id="PortValue" Type="Edit" X="170" Y="60" Width="160" Height="18" Property="PANEL_PORT" />

        <Control Id="UsernameLabel" Type="Text" X="20" Y="96" Width="140" Height="14" Text="Initial Username:" />
        <Control Id="UsernameValue" Type="Edit" X="170" Y="94" Width="160" Height="18" Property="PANEL_USERNAME" />

        <Control Id="PasswordLabel" Type="Text" X="20" Y="130" Width="140" Height="14" Text="Initial Password:" />
        <Control Id="PasswordValue" Type="Edit" X="170" Y="128" Width="160" Height="18" Property="PANEL_PASSWORD" Password="yes" />

        <Control Id="HintLine1" Type="Text" X="20" Y="168" Width="320" Height="14" Transparent="yes" NoPrefix="yes" Text="Username: 3-32 chars, letters, digits, dot, underscore, at, hyphen." />
        <Control Id="HintLine2" Type="Text" X="20" Y="184" Width="320" Height="28" Transparent="yes" NoPrefix="yes" Text="Password: 6-64 chars, letters, digits, and . _ ! @ # $ % ^ &amp; * ( ) - only." />
      </Dialog>

      <Publish Dialog="ExitDialog" Control="Finish" Event="EndDialog" Value="Return" Order="999" />

      <Publish Dialog="WelcomeDlg" Control="Next" Event="NewDialog" Value="LicenseAgreementDlg" Condition="NOT Installed" />
      <Publish Dialog="WelcomeDlg" Control="Next" Event="NewDialog" Value="VerifyReadyDlg" Condition="Installed AND PATCH" />

      <Publish Dialog="LicenseAgreementDlg" Control="Back" Event="NewDialog" Value="WelcomeDlg" />
      <Publish Dialog="LicenseAgreementDlg" Control="Next" Event="NewDialog" Value="InstallDirDlg" Condition="LicenseAccepted = &quot;1&quot;" />

      <Publish Dialog="InstallDirDlg" Control="Back" Event="NewDialog" Value="LicenseAgreementDlg" />
      <Publish Dialog="InstallDirDlg" Control="ChangeFolder" Property="_BrowseProperty" Value="[WIXUI_INSTALLDIR]" Order="1" />
      <Publish Dialog="InstallDirDlg" Control="ChangeFolder" Event="SpawnDialog" Value="BrowseDlg" Order="2" />
      <Publish Dialog="InstallDirDlg" Control="Next" Event="CheckTargetPath" Value="[WIXUI_INSTALLDIR]" Order="1" />
      <Publish Dialog="InstallDirDlg" Control="Next" Event="SetTargetPath" Value="[WIXUI_INSTALLDIR]" Order="3" />
      <Publish Dialog="InstallDirDlg" Control="Next" Event="NewDialog" Value="PanelConfigDlg" Order="4" Condition="NOT Installed" />
      <Publish Dialog="InstallDirDlg" Control="Next" Event="NewDialog" Value="VerifyReadyDlg" Order="4" Condition="Installed" />

      <Publish Dialog="BrowseDlg" Control="OK" Event="CheckTargetPath" Value="[WIXUI_INSTALLDIR]" Order="1" />
      <Publish Dialog="BrowseDlg" Control="OK" Event="SetTargetPath" Value="[_BrowseProperty]" Order="3" />
      <Publish Dialog="BrowseDlg" Control="OK" Event="EndDialog" Value="Return" Order="4" />

      <Publish Dialog="PanelConfigDlg" Control="Back" Event="NewDialog" Value="InstallDirDlg" />
      <Publish Dialog="PanelConfigDlg" Control="Next" Event="NewDialog" Value="VerifyReadyDlg" />

      <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="PanelConfigDlg" Order="1" Condition="NOT Installed" />
      <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="MaintenanceTypeDlg" Order="2" Condition="Installed AND NOT PATCH" />
      <Publish Dialog="VerifyReadyDlg" Control="Back" Event="NewDialog" Value="WelcomeDlg" Order="2" Condition="Installed AND PATCH" />

      <Publish Dialog="MaintenanceWelcomeDlg" Control="Next" Event="NewDialog" Value="MaintenanceTypeDlg" />

      <Publish Dialog="MaintenanceTypeDlg" Control="RepairButton" Event="NewDialog" Value="VerifyReadyDlg" />
      <Publish Dialog="MaintenanceTypeDlg" Control="RemoveButton" Event="NewDialog" Value="VerifyReadyDlg" />
      <Publish Dialog="MaintenanceTypeDlg" Control="Back" Event="NewDialog" Value="MaintenanceWelcomeDlg" />
    </UI>

    <UIRef Id="WixUI_Common" />

    <Feature Id="MainFeature" Title="1Panel" Level="1">
      <ComponentGroupRef Id="ProductComponents" />
      <ComponentRef Id="StartMenuShortcuts" />
    </Feature>

    <SetProperty
      Id="RunInstallScript"
      Value="&quot;[%ComSpec]&quot; /c powershell.exe -ExecutionPolicy Bypass -File &quot;[INSTALLFOLDER]install.ps1&quot; -InstallDir &quot;[INSTALLFOLDER]&quot; -PanelVersion &quot;[ProductVersion]&quot; -PanelPort &quot;[PANEL_PORT]&quot; -PanelUsername &quot;[PANEL_USERNAME]&quot; -PanelPassword &quot;[PANEL_PASSWORD]&quot;"
      Before="RunInstallScript"
      Sequence="execute"
      Condition="NOT Installed"
    />
    <CustomAction
      Id="RunInstallScript"
      BinaryRef="Wix4UtilCA_$(sys.BUILDARCHSHORT)"
      DllEntry="WixQuietExec"
      Execute="deferred"
      Impersonate="no"
      Return="check"
    />

    <SetProperty
      Id="RunUninstallScript"
      Value="&quot;[%ComSpec]&quot; /c powershell.exe -ExecutionPolicy Bypass -File &quot;[INSTALLFOLDER]uninstall.ps1&quot; -InstallDir &quot;[INSTALLFOLDER]&quot;"
      Before="RunUninstallScript"
      Sequence="execute"
      Condition="REMOVE~=&quot;ALL&quot; AND NOT UPGRADINGPRODUCTCODE"
    />
    <CustomAction
      Id="RunUninstallScript"
      BinaryRef="Wix4UtilCA_$(sys.BUILDARCHSHORT)"
      DllEntry="WixQuietExec"
      Execute="deferred"
      Impersonate="no"
      Return="ignore"
    />

    <InstallExecuteSequence>
      <Custom Action="RunInstallScript" After="InstallFiles" Condition="NOT Installed" />
      <Custom Action="RunUninstallScript" Before="RemoveFiles" Condition="REMOVE~=&quot;ALL&quot; AND NOT UPGRADINGPRODUCTCODE" />
    </InstallExecuteSequence>

    <ComponentGroup Id="ProductComponents">
__WIX_COMPONENTS__
    </ComponentGroup>

    <Component Id="StartMenuShortcuts" Directory="ProgramMenuDir" Guid="*">
      <Shortcut
        Id="OpenInstallDirShortcut"
        Name="Open Install Directory"
        Description="Open the installed 1Panel directory"
        Target="[INSTALLFOLDER]open-install-dir.cmd"
        WorkingDirectory="INSTALLFOLDER"
      />
      <RemoveFolder Id="RemoveProgramMenuDir" On="uninstall" />
      <RegistryValue Root="HKLM" Key="Software\1Panel" Name="StartMenuShortcut" Type="integer" Value="1" KeyPath="yes" />
    </Component>
  </Package>
</Wix>
