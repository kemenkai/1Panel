#!/usr/bin/env python3
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tempfile
import uuid
import zipfile
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
TEMPLATE_ROOT = REPO_ROOT / "scripts" / "windows-installer"
DEFAULT_OUTPUT_DIR = REPO_ROOT / "build" / "packages" / "windows"

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Build a Windows binary package for 1Panel without requiring end users to compile from source."
    )
    parser.add_argument("--version", default="", help="Package version, for example v2.1.10")
    parser.add_argument(
        "--core-binary",
        default=str(REPO_ROOT / "build" / "windows" / "1panel-core.exe"),
        help="Path to 1panel-core.exe",
    )
    parser.add_argument(
        "--agent-binary",
        default=str(REPO_ROOT / "build" / "windows" / "1panel-agent.exe"),
        help="Path to 1panel-agent.exe",
    )
    parser.add_argument("--winsw-path", default="", help="Path to WinSW.exe to bundle into the package")
    parser.add_argument("--jdk-zip", default="", help="Path to bundled JDK zip to include in the package")
    parser.add_argument("--output-dir", default=str(DEFAULT_OUTPUT_DIR), help="Output directory")
    parser.add_argument("--skip-build", action="store_true", help="Skip rebuilding frontend assets and Windows binaries before packaging")
    parser.add_argument("--install-dir", default=r"C:\1Panel", help="Default install directory")
    parser.add_argument("--panel-port", default="9999", help="Default panel port")
    parser.add_argument("--panel-mode", default="release", help="Panel mode written into app.yaml")
    parser.add_argument("--panel-language", default="zh", help="Default panel language")
    parser.add_argument("--panel-edition", default="cn", help="Default panel edition")
    parser.add_argument("--panel-username", default="admin", help="Default panel username")
    parser.add_argument("--panel-password", default="admin123", help="Default panel password")
    parser.add_argument("--clean", action="store_true", help="Delete any existing output directory before packaging")
    parser.add_argument("--build-exe", action="store_true", help="Build an Inno Setup installer when ISCC is available")
    parser.add_argument("--iscc-path", default="", help="Path to Inno Setup compiler ISCC.exe")
    parser.add_argument("--build-msi", action="store_true", help="Build an MSI package when WiX Toolset is available")
    parser.add_argument("--wix-path", default="", help="Path to wix.exe")
    parser.add_argument("--dotnet-root", default="", help="Path to dotnet root used to run WiX tool")
    return parser.parse_args()


def run_command(command: list[str], cwd: Path | None = None, env: dict[str, str] | None = None) -> None:
    if os.name == "nt" and command:
        executable = command[0]
        if executable.lower() in {"npm", "npx"}:
            resolved = shutil.which(f"{executable}.cmd") or shutil.which(executable)
            if resolved:
                command = [resolved, *command[1:]]
    print("[package_1panel_windows]", " ".join(command))
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    subprocess.run(command, cwd=str(cwd) if cwd else None, check=True, env=merged_env)


def resolve_version(version: str) -> str:
    if version:
        return version
    env_version = os.environ.get("PANEL_VERSION", "").strip()
    if env_version:
        return env_version
    try:
        result = subprocess.run(
            ["git", "describe", "--tags", "--abbrev=0"],
            cwd=str(REPO_ROOT),
            capture_output=True,
            text=True,
            check=True,
        )
        git_version = result.stdout.strip()
        if git_version:
            return git_version
    except Exception:
        pass
    installed_env = Path(r"C:\1Panel\1panel\conf\1pctl.env")
    if installed_env.exists():
        for line in installed_env.read_text(encoding="utf-8", errors="ignore").splitlines():
            if line.startswith("ORIGINAL_VERSION="):
                value = line.split("=", 1)[1].strip()
                if value:
                    return value
    return "v0.0.0-dev"


def resolve_winsw_path(explicit_path: str) -> Path | None:
    candidates: list[Path] = []
    if explicit_path:
        candidates.append(Path(explicit_path))
    candidates.extend(
        [
            Path(r"C:\tools\WinSW.exe"),
            Path(r"C:\1Panel\tools\WinSW.exe"),
        ]
    )
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return None


def resolve_jdk_zip(explicit_path: str) -> Path | None:
    candidates: list[Path] = []
    if explicit_path:
        candidates.append(Path(explicit_path))
    candidates.extend(sorted(REPO_ROOT.glob("zulu*-win_x64.zip")))
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return None


def sanitize_version(version: str) -> str:
    return version.replace("/", "-").replace("\\", "-")


def to_msi_version(version: str) -> str:
    text = version.strip()
    if text.startswith(("v", "V")):
        text = text[1:]
    numeric_parts: list[str] = []
    current = ""
    for ch in text:
        if ch.isdigit():
            current += ch
        else:
            if current:
                numeric_parts.append(current)
                current = ""
            if len(numeric_parts) >= 3:
                break
    if current and len(numeric_parts) < 3:
        numeric_parts.append(current)
    while len(numeric_parts) < 3:
        numeric_parts.append("0")
    return ".".join(numeric_parts[:3])


def render_template(template_name: str, replacements: dict[str, str]) -> str:
    template_path = TEMPLATE_ROOT / template_name
    content = template_path.read_text(encoding="utf-8")
    for key, value in replacements.items():
        content = content.replace(key, value)
    return content


def escape_rtf(text: str) -> str:
    return (
        text.replace("\\", "\\\\")
        .replace("{", r"\{")
        .replace("}", r"\}")
        .replace("\r\n", "\n")
        .replace("\r", "\n")
        .replace("\n", r"\par " + "\n")
    )


def build_license_rtf() -> str:
    license_path = REPO_ROOT / "LICENSE"
    if license_path.exists():
        license_text = license_path.read_text(encoding="utf-8", errors="ignore").strip()
    else:
        license_text = "1Panel Windows Installer License Information"
    return "{\\rtf1\\ansi\\deff0\n" + escape_rtf(license_text) + "\n}\n"


def ensure_file(path: Path, label: str) -> None:
    if not path.exists():
        raise FileNotFoundError(f"missing {label}: {path}")


def ensure_frontend_dependencies() -> None:
    node_modules_dir = REPO_ROOT / "frontend" / "node_modules"
    package_lock = REPO_ROOT / "frontend" / "package-lock.json"
    if node_modules_dir.exists():
        return
    if package_lock.exists():
        run_command(["npm", "install"], cwd=REPO_ROOT / "frontend")
        return
    run_command(["npm", "install"], cwd=REPO_ROOT / "frontend")


def rebuild_frontend_assets() -> None:
    web_root = REPO_ROOT / "core" / "cmd" / "server" / "web"
    assets_dir = web_root / "assets"
    index_html = web_root / "index.html"
    if assets_dir.exists():
        shutil.rmtree(assets_dir)
    if index_html.exists():
        index_html.unlink()
    ensure_frontend_dependencies()
    run_command(["npm", "run", "build:pro"], cwd=REPO_ROOT / "frontend")


def rebuild_windows_binaries(core_binary: Path, agent_binary: Path, version: str = "") -> None:
    build_env = {
        "CGO_ENABLED": "0",
        "GOOS": "windows",
        "GOARCH": "amd64",
    }
    core_binary.parent.mkdir(parents=True, exist_ok=True)
    agent_binary.parent.mkdir(parents=True, exist_ok=True)
    # 编译期注入版本号，使 core 启动自愈能将 SystemVersion 写回数据库。
    core_ldflags = "-s -w"
    if version:
        core_ldflags += f" -X github.com/1Panel-dev/1Panel/core/buildinfo.Version={version}"
    run_command(
        ["go", "build", "-trimpath", "-ldflags", core_ldflags, "-o", str(core_binary), "./cmd/server/main.go"],
        cwd=REPO_ROOT / "core",
        env=build_env,
    )
    run_command(
        ["go", "build", "-trimpath", "-ldflags", "-s -w", "-o", str(agent_binary), "./cmd/server/main.go"],
        cwd=REPO_ROOT / "agent",
        env=build_env,
    )


def create_zip_archive(source_dir: Path, archive_path: Path) -> None:
    archive_path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(archive_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for file_path in source_dir.rglob("*"):
            if file_path.is_file():
                zf.write(file_path, file_path.relative_to(source_dir.parent))


def find_iscc(explicit_path: str) -> Path | None:
    candidates: list[Path] = []
    if explicit_path:
        candidates.append(Path(explicit_path))
    which_path = shutil.which("ISCC")
    if which_path:
        candidates.append(Path(which_path))
    candidates.extend(
        [
            Path(r"C:\Program Files (x86)\Inno Setup 6\ISCC.exe"),
            Path(r"C:\Program Files\Inno Setup 6\ISCC.exe"),
        ]
    )
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return None


def build_inno_installer(
    stage_dir: Path,
    output_dir: Path,
    version: str,
    install_dir: str,
    iscc_path: Path,
) -> Path:
    output_basename = f"1panel-windows-{sanitize_version(version)}-setup"
    script_content = render_template(
        "1panel-setup.iss.tpl",
        {
            "__APP_VERSION__": version,
            "__INSTALL_DIR__": install_dir.replace("\\", "\\\\"),
            "__PACKAGE_ROOT__": str(stage_dir).replace("\\", "\\\\"),
            "__OUTPUT_DIR__": str(output_dir).replace("\\", "\\\\"),
            "__OUTPUT_BASENAME__": output_basename,
        },
    )
    script_path = output_dir / f"{output_basename}.iss"
    script_path.write_text(script_content, encoding="utf-8")
    run_command([str(iscc_path), str(script_path)])
    return output_dir / f"{output_basename}.exe"


def find_iexpress() -> Path | None:
    iexpress = shutil.which("iexpress.exe")
    if iexpress:
        return Path(iexpress)
    fallback = Path(r"C:\Windows\System32\iexpress.exe")
    if fallback.exists():
        return fallback
    return None


def find_wix(explicit_path: str) -> Path | None:
    candidates: list[Path] = []
    if explicit_path:
        candidates.append(Path(explicit_path))
    which_path = shutil.which("wix.exe") or shutil.which("wix")
    if which_path:
        candidates.append(Path(which_path))
    candidates.extend(
        [
            Path(r"C:\Tools\wix\wix.exe"),
            Path(r"C:\Program Files\WiX Toolset v4\bin\wix.exe"),
            Path(r"C:\Program Files (x86)\WiX Toolset v4\bin\wix.exe"),
        ]
    )
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return None


def resolve_dotnet_root(explicit_path: str) -> Path | None:
    candidates: list[Path] = []
    if explicit_path:
        candidates.append(Path(explicit_path))
    if os.environ.get("DOTNET_ROOT"):
        candidates.append(Path(os.environ["DOTNET_ROOT"]))
    candidates.extend(
        [
            Path(r"C:\Tools\dotnet"),
            Path(r"C:\Program Files\dotnet"),
        ]
    )
    for candidate in candidates:
        if (candidate / "dotnet.exe").exists():
            return candidate
    return None


def build_iexpress_installer(stage_dir: Path, output_dir: Path, version: str) -> Path:
    output_basename = f"1panel-windows-{sanitize_version(version)}-sfx"
    exe_path = output_dir / f"{output_basename}.exe"
    iexpress_path = find_iexpress()
    if iexpress_path is None:
        raise FileNotFoundError("iexpress.exe not found")
    temp_root = Path(tempfile.gettempdir()) / "1panel-iexpress-build"
    if temp_root.exists():
        shutil.rmtree(temp_root)
    ascii_stage_dir = temp_root / "package"
    ascii_output_dir = temp_root / "output"
    ascii_output_dir.mkdir(parents=True, exist_ok=True)
    shutil.copytree(stage_dir, ascii_stage_dir)

    source_keys: list[str] = []
    string_keys: list[str] = []
    file_index = 0
    for file_path in sorted(ascii_stage_dir.rglob("*")):
        if not file_path.is_file():
            continue
        relative_path = str(file_path.relative_to(ascii_stage_dir)).replace("/", "\\")
        key = f"FILE{file_index}"
        source_keys.append(f"%{key}%=")
        string_keys.append(f'{key}="{relative_path}"')
        file_index += 1

    ascii_exe_path = ascii_output_dir / f"{output_basename}.exe"
    sed_path = ascii_output_dir / f"{output_basename}.sed"
    sed_content = render_template(
        "1panel-iexpress.sed.tpl",
        {
            "__TARGET_NAME__": str(ascii_exe_path),
            "__APP_VERSION__": version,
            "__PACKAGE_ROOT__": str(ascii_stage_dir),
            "__SOURCE_FILE_KEYS__": "\n".join(source_keys),
            "__STRING_FILE_KEYS__": "\n".join(string_keys),
        },
    )
    sed_path.write_text(sed_content, encoding="utf-8")
    run_command([str(iexpress_path), "/N", str(sed_path)])
    shutil.copy2(ascii_exe_path, exe_path)
    return exe_path


def build_wix_components(stage_dir: Path) -> str:
    lines: list[str] = []
    file_index = 0
    for file_path in sorted(stage_dir.rglob("*")):
        if not file_path.is_file():
            continue
        relative_path = file_path.relative_to(stage_dir)
        relative_windows = str(relative_path).replace("/", "\\")
        subdirectory = str(relative_path.parent).replace("/", "\\")
        component_id = f"Cmp{file_index}"
        file_id = f"Fil{file_index}"
        attrs = [f'Id="{component_id}"', 'Directory="INSTALLFOLDER"', 'Guid="*"']
        if subdirectory not in ("", "."):
            attrs.append(f'Subdirectory="{subdirectory}"')
        lines.append(f'      <Component {" ".join(attrs)}>')  # noqa: B950
        lines.append(f'        <File Id="{file_id}" Source="{relative_windows}" KeyPath="yes" />')
        lines.append("      </Component>")
        file_index += 1
    return "\n".join(lines)


def ensure_wix_util_extension(wix_path: Path, dotnet_root: Path | None) -> None:
    wix_env: dict[str, str] = {}
    if dotnet_root is not None:
        wix_env["DOTNET_ROOT"] = str(dotnet_root)
        wix_env["DOTNET_ROOT_X64"] = str(dotnet_root)
        wix_env["PATH"] = str(dotnet_root) + os.pathsep + str(wix_path.parent) + os.pathsep + os.environ.get("PATH", "")
    run_command(
        [
            str(wix_path),
            "extension",
            "add",
            "-g",
            "WixToolset.Util.wixext/7.0.0",
            "-acceptEula",
            "wix7",
        ],
        env=wix_env,
    )


def ensure_wix_ui_extension(wix_path: Path, dotnet_root: Path | None) -> None:
    wix_env: dict[str, str] = {}
    if dotnet_root is not None:
        wix_env["DOTNET_ROOT"] = str(dotnet_root)
        wix_env["DOTNET_ROOT_X64"] = str(dotnet_root)
        wix_env["PATH"] = str(dotnet_root) + os.pathsep + str(wix_path.parent) + os.pathsep + os.environ.get("PATH", "")
    run_command(
        [
            str(wix_path),
            "extension",
            "add",
            "-g",
            "WixToolset.UI.wixext/7.0.0",
            "-acceptEula",
            "wix7",
        ],
        env=wix_env,
    )


def build_msi_with_wix(
    stage_dir: Path,
    output_dir: Path,
    version: str,
    install_dir: str,
    panel_port: str,
    panel_username: str,
    panel_password: str,
    wix_path: Path,
    dotnet_root: Path | None,
) -> Path:
    temp_root = Path(tempfile.gettempdir()) / "1panel-wix-build"
    if temp_root.exists():
        shutil.rmtree(temp_root)
    ascii_stage_dir = temp_root / "package"
    ascii_output_dir = temp_root / "output"
    ascii_output_dir.mkdir(parents=True, exist_ok=True)
    shutil.copytree(stage_dir, ascii_stage_dir)

    version_slug = sanitize_version(version)
    msi_version = to_msi_version(version)
    wxs_path = ascii_output_dir / "1panel-package.wxs"
    wixproj_path = ascii_output_dir / "1panel-package.wixproj"
    license_path = ascii_output_dir / "license.rtf"
    output_msi = output_dir / f"1panel-windows-{version_slug}.msi"
    upgrade_code = str(uuid.uuid5(uuid.NAMESPACE_URL, "https://github.com/1Panel-dev/1Panel/windows-msi-upgrade-code"))
    license_path.write_text(build_license_rtf(), encoding="utf-8")

    wxs_path.write_text(
        render_template(
            "1panel-package.wxs.tpl",
            {
                "__MSI_VERSION__": msi_version,
                "__UPGRADE_CODE__": upgrade_code,
                "__DEFAULT_INSTALL_DIR__": install_dir,
                "__DEFAULT_PANEL_PORT__": panel_port,
                "__DEFAULT_PANEL_USERNAME__": panel_username,
                "__DEFAULT_PANEL_PASSWORD__": panel_password,
                "__LICENSE_RTF__": str(license_path),
                "__WIX_COMPONENTS__": build_wix_components(ascii_stage_dir),
            },
        ),
        encoding="utf-8",
    )
    wixproj_path.write_text(render_template("1panel-package.wixproj.tpl", {}), encoding="utf-8")

    wix_env: dict[str, str] = {}
    if dotnet_root is not None:
        wix_env["DOTNET_ROOT"] = str(dotnet_root)
        wix_env["DOTNET_ROOT_X64"] = str(dotnet_root)
        wix_env["PATH"] = str(dotnet_root) + os.pathsep + str(wix_path.parent) + os.pathsep + os.environ.get("PATH", "")

    ensure_wix_util_extension(wix_path=wix_path, dotnet_root=dotnet_root)
    ensure_wix_ui_extension(wix_path=wix_path, dotnet_root=dotnet_root)
    run_command(
        [
            str(wix_path),
            "build",
            "-acceptEula",
            "wix7",
            str(wxs_path),
            "-ext",
            "WixToolset.Util.wixext",
            "-ext",
            "WixToolset.UI.wixext",
            "-arch",
            "x64",
            "-o",
            str(output_msi),
        ],
        cwd=ascii_stage_dir,
        env=wix_env,
    )
    return output_msi


def main() -> int:
    args = parse_args()

    version = resolve_version(args.version)
    core_binary = Path(args.core_binary)
    agent_binary = Path(args.agent_binary)
    winsw_path = resolve_winsw_path(args.winsw_path)
    jdk_zip_path = resolve_jdk_zip(args.jdk_zip)
    output_dir = Path(args.output_dir)

    if not args.skip_build:
        rebuild_frontend_assets()
        rebuild_windows_binaries(core_binary=core_binary, agent_binary=agent_binary, version=version)

    ensure_file(core_binary, "core binary")
    ensure_file(agent_binary, "agent binary")

    if args.clean and output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    version_slug = sanitize_version(version)
    stage_dir = output_dir / f"1panel-windows-{version_slug}"
    if stage_dir.exists():
        shutil.rmtree(stage_dir)

    (stage_dir / "bin").mkdir(parents=True, exist_ok=True)
    (stage_dir / "tools").mkdir(parents=True, exist_ok=True)
    (stage_dir / "runtime").mkdir(parents=True, exist_ok=True)

    shutil.copy2(core_binary, stage_dir / "bin" / "1panel-core.exe")
    shutil.copy2(agent_binary, stage_dir / "bin" / "1panel-agent.exe")
    if winsw_path:
        shutil.copy2(winsw_path, stage_dir / "tools" / "WinSW.exe")
    if jdk_zip_path:
        shutil.copy2(jdk_zip_path, stage_dir / "runtime" / jdk_zip_path.name)

    replacements = {
        "__APP_VERSION__": version,
        "__INSTALL_DIR__": args.install_dir,
        "__PANEL_PORT__": args.panel_port,
        "__PANEL_USERNAME__": args.panel_username,
        "__PANEL_PASSWORD__": args.panel_password,
    }
    (stage_dir / "open-install-dir.cmd").write_text(
        render_template("open-install-dir.cmd.tpl", replacements),
        encoding="utf-8",
    )
    (stage_dir / "install.cmd").write_text(
        render_template("install.cmd.tpl", replacements),
        encoding="utf-8",
    )
    (stage_dir / "install-interactive.ps1").write_text(
        render_template("install-interactive.ps1.tpl", replacements),
        encoding="utf-8",
    )
    (stage_dir / "uninstall.cmd").write_text(
        render_template("uninstall.cmd.tpl", replacements),
        encoding="utf-8",
    )
    (stage_dir / "uninstall-interactive.ps1").write_text(
        render_template("uninstall-interactive.ps1.tpl", replacements),
        encoding="utf-8",
    )
    (stage_dir / "install.ps1").write_text(render_template("install.ps1.tpl", replacements), encoding="utf-8")
    (stage_dir / "uninstall.ps1").write_text(render_template("uninstall.ps1.tpl", replacements), encoding="utf-8")
    (stage_dir / "README.txt").write_text(render_template("README.txt.tpl", replacements), encoding="utf-8")

    zip_path = output_dir / f"1panel-windows-{version_slug}.zip"
    create_zip_archive(stage_dir, zip_path)
    print(f"[package_1panel_windows] created zip package: {zip_path}")

    if args.build_exe:
        iscc_path = find_iscc(args.iscc_path)
        if iscc_path:
            exe_path = build_inno_installer(
                stage_dir=stage_dir,
                output_dir=output_dir,
                version=version,
                install_dir=args.install_dir,
                iscc_path=iscc_path,
            )
            print(f"[package_1panel_windows] created setup exe with Inno Setup: {exe_path}")
        else:
            exe_path = build_iexpress_installer(stage_dir=stage_dir, output_dir=output_dir, version=version)
            print(f"[package_1panel_windows] created self-extracting exe with IExpress: {exe_path}")
    else:
        print("[package_1panel_windows] skipped exe build")

    if args.build_msi:
        wix_path = find_wix(args.wix_path)
        if not wix_path:
            raise FileNotFoundError("WiX Toolset wix.exe not found; install WiX Toolset v7 or pass --wix-path")
        dotnet_root = resolve_dotnet_root(args.dotnet_root)
        msi_path = build_msi_with_wix(
            stage_dir=stage_dir,
            output_dir=output_dir,
            version=version,
            install_dir=args.install_dir,
            panel_port=args.panel_port,
            panel_username=args.panel_username,
            panel_password=args.panel_password,
            wix_path=wix_path,
            dotnet_root=dotnet_root,
        )
        print(f"[package_1panel_windows] created msi package: {msi_path}")
    else:
        print("[package_1panel_windows] skipped msi build")

    if winsw_path is None:
        print("[package_1panel_windows] warning: WinSW.exe was not bundled; end users must provide it when installing")
    if jdk_zip_path is None:
        print("[package_1panel_windows] warning: bundled JDK zip was not found; Java service templates will need a runtime in the install directory")

    return 0


if __name__ == "__main__":
    sys.exit(main())
