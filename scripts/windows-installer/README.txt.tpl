1Panel Windows Binary Package
=============================

Package version: __APP_VERSION__

This package is intended for end users who should not need Go, Node.js, or the source repository.

Installation methods:

1. Preferred:
   Extract the zip package, right click install.cmd, then choose "Run as administrator".
   The installer will interactively ask for:
   - install directory
   - panel port
   - initial username
   - initial password
   It will also:
   - auto request administrator privileges
   - validate the input values
   - detect an existing install directory
   - show an installation summary before applying changes
   - optionally create desktop shortcuts
   - optionally open the panel in a browser after install

2. Fallback:
   Open PowerShell as administrator, switch to the extracted directory, then run:

   powershell -ExecutionPolicy Bypass -File .\install.ps1

Locate the installed directory:

   After installation, use the Start Menu shortcut:
   1Panel -> Open Install Directory

   If you installed from the zip package, you can also run:
   .\open-install-dir.cmd

Default install directory:

   __INSTALL_DIR__

Installer log output:

   After installation, the installer transcript is stored under:
   <install-dir>\installer-logs\

Bundled files:

   bin\1panel-core.exe
   bin\1panel-agent.exe
   tools\WinSW.exe   (if included during packaging)
   install.cmd
   install-interactive.ps1
   open-install-dir.cmd
   install.ps1
   uninstall.cmd
   uninstall-interactive.ps1
   uninstall.ps1

After installation, open:

   http://127.0.0.1:__PANEL_PORT__

Default account:

   username: __PANEL_USERNAME__
   password: __PANEL_PASSWORD__
