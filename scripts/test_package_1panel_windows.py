from __future__ import annotations

import unittest
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import package_1panel_windows


REPLACEMENTS = {
    "__APP_VERSION__": "v2.1.10",
    "__INSTALL_DIR__": r"C:\1Panel",
    "__PANEL_PORT__": "9999",
    "__PANEL_USERNAME__": "admin",
    "__PANEL_PASSWORD__": "admin123",
}


class WindowsPackageTemplateTests(unittest.TestCase):
    def render(self, template_name: str) -> str:
        return package_1panel_windows.render_template(template_name, REPLACEMENTS)

    def test_installer_persists_actual_install_directory_for_uninstall(self) -> None:
        content = self.render("install.ps1.tpl")

        self.assertIn("Save-InstallLocation", content)
        self.assertIn(r"HKLM:\Software\1Panel", content)
        self.assertIn("InstallDir", content)

    def test_interactive_uninstaller_detects_actual_install_directory(self) -> None:
        content = self.render("uninstall-interactive.ps1.tpl")

        self.assertIn("Get-DefaultInstallDir", content)
        self.assertIn("Get-InstallDirFromRegistry", content)
        self.assertIn("Get-InstallDirFromService", content)
        self.assertIn("LooksLikeInstallDir", content)

    def test_uninstaller_resolves_default_directory_before_uninstalling(self) -> None:
        content = self.render("uninstall.ps1.tpl")

        self.assertIn("Resolve-EffectiveInstallDir", content)
        self.assertIn("Get-InstallDirFromRegistry", content)
        self.assertIn("Get-InstallDirFromService", content)
        self.assertIn("Clear-InstallLocation", content)

    def test_uninstaller_does_not_treat_unpacked_package_bin_as_install_dir(self) -> None:
        for template_name in ("uninstall.ps1.tpl", "uninstall-interactive.ps1.tpl"):
            content = self.render(template_name)

            self.assertNotIn(r"bin\1panel-core.exe", content)

    def test_interactive_uninstaller_prompts_for_managed_windows_services(self) -> None:
        content = self.render("uninstall-interactive.ps1.tpl")

        self.assertIn("Get-ManagedServiceDirectories", content)
        self.assertIn("Remove managed Windows services", content)
        self.assertIn("RemoveManagedServices", content)

    def test_uninstaller_supports_removing_managed_windows_services(self) -> None:
        content = self.render("uninstall.ps1.tpl")

        self.assertIn("[switch]$RemoveManagedServices", content)
        self.assertIn("Get-ManagedServiceDirectories", content)
        self.assertIn("Remove-ManagedServiceArtifacts", content)


if __name__ == "__main__":
    unittest.main()
