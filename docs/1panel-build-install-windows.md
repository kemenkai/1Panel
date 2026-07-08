# 1Panel Windows Lite 构建与安装说明

> 注意：本文档面向开发机或测试机。
>
> 如果目标是给最终用户交付无需编译环境的安装包，请优先查看：
>
> [`docs/1panel-windows-binary-package.md`](./1panel-windows-binary-package.md)

## 目标

当前仓库内的 Windows Lite 安装脚本用于：

- 构建 `1panel-core.exe`
- 构建 `1panel-agent.exe`
- 生成 `app.yaml`
- 生成 `1pctl.env`
- 生成 `WinSW` 服务包装配置
- 可选注册并启动 Windows 服务

## 脚本位置

[`scripts/build_install_1panel_windows.ps1`](../scripts/build_install_1panel_windows.ps1)

## 基本用法

```powershell
powershell -ExecutionPolicy Bypass -File ".\scripts\build_install_1panel_windows.ps1"
```

如果已经准备好了 `WinSW.exe`，可直接一起传入并安装服务：

```powershell
powershell -ExecutionPolicy Bypass -File ".\scripts\build_install_1panel_windows.ps1" `
  -WinSWPath "C:\tools\WinSW.exe"
```

## 主要参数

- `-InstallDir`：安装目录，默认 `C:\1Panel`
- `-PanelPort`：面板端口，默认 `9999`
- `-PanelUsername`：初始用户名，默认 `admin`
- `-PanelPassword`：初始密码，默认 `admin123`
- `-PanelVersion`：面板版本，未传时自动解析
- `-SkipFrontend`：跳过前端构建
- `-SkipNpmInstall`：跳过 `npm install`
- `-SkipBuild`：跳过 Go 编译
- `-SkipServiceInstall`：只生成服务配置，不安装服务

## 输入校验

脚本当前已增加以下校验：

- 端口必须是 `1-65535`
- 用户名必须是 `3-32` 位，仅允许字母、数字、`.`、`_`、`@`、`-`
- 密码必须是 `6-64` 位，仅允许字母、数字、`.`、`_`、`!`、`@`、`#`、`$`、`%`、`^`、`&`、`*`、`(`、`)`、`-`
- 写入 `1pctl.env` 的值不能包含换行或 `=`

这样可以避免生成损坏的配置文件。

## 输出目录

默认输出到：

- 安装目录：`C:\1Panel`
- 配置目录：`C:\1Panel\1panel\conf`
- 二进制目录：`C:\1Panel\bin`
- 服务目录：`C:\1Panel\service`

## 输出内容

脚本会生成：

- `C:\1Panel\bin\1panel-core.exe`
- `C:\1Panel\bin\1panel-agent.exe`
- `C:\1Panel\1panel\conf\app.yaml`
- `C:\1Panel\1panel\conf\1pctl.env`
- `C:\1Panel\service\1panel-core-service.xml`
- `C:\1Panel\service\1panel-agent-service.xml`

如果提供了 `WinSW.exe`，还会安装：

- `C:\1Panel\service\1panel-core-service.exe`
- `C:\1Panel\service\1panel-agent-service.exe`

## 当前边界

这份脚本解决的是 Windows Lite 的本地构建和安装问题，不等同于完整的最终用户安装器。

最终用户交付请使用：

- `package_1panel_windows.py`
- `zip`
- `exe`
- `msi`
