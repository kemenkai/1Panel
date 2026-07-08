# 1Panel Windows Binary Package

本文档说明如何为 Windows 最终用户生成并交付当前版本的 `1Panel` 安装包。

当前推荐交付方式已经调整为：

1. `zip` 交互安装包
2. 包内自带 `install.cmd` / `uninstall.cmd`
3. 用户不需要安装 `Go`、`Node.js`、`npm` 或源码仓库

`MSI` 和 `EXE` 仍然可以构建，但当前不再作为首选交付方式。

如果需要单独给最终用户使用的安装说明，请参考：

- [docs/windows-1panel-zip-install-guide.md](./windows-1panel-zip-install-guide.md)

## 1. 当前推荐方式

推荐直接发布：

- `1panel-windows-vX.Y.Z.zip`

原因：

- 交互安装逻辑完全由仓库内脚本控制，行为更稳定
- 不依赖 `MSI` 安装向导的目录展示体验
- 更容易定位问题，安装日志直接落盘
- 更适合测试机、客户环境和多次重装场景

## 2. 打包脚本

主打包脚本：

- [scripts/package_1panel_windows.py](../scripts/package_1panel_windows.py)

这个脚本面向“发布最终安装包”，不是面向开发机本地编译调试。

## 3. 产物目录

默认输出目录：

```text
build/packages/windows/
├─ 1panel-windows-v2.1.10/
│  ├─ install.cmd
│  ├─ install-interactive.ps1
│  ├─ uninstall.cmd
│  ├─ uninstall-interactive.ps1
│  ├─ install.ps1
│  ├─ uninstall.ps1
│  ├─ open-install-dir.cmd
│  ├─ README.txt
│  ├─ bin/
│  │  ├─ 1panel-core.exe
│  │  └─ 1panel-agent.exe
│  ├─ runtime/
│  │  └─ zulu21.50.19-ca-jdk21.0.11-win_x64.zip
│  └─ tools/
│     └─ WinSW.exe
└─ 1panel-windows-v2.1.10.zip
```

如果显式启用了可选构建参数，还可以额外得到：

- `1panel-windows-vX.Y.Z-sfx.exe`
- `1panel-windows-vX.Y.Z.msi`

但这两类不是当前默认推荐交付物。

## 4. 打包前置条件

打包前请准备：

- `build/windows/1panel-core.exe`
- `build/windows/1panel-agent.exe`
- `WinSW.exe`
- 可选但推荐：JDK ZIP，例如 `zulu21.50.19-ca-jdk21.0.11-win_x64.zip`

当前包会把 `WinSW.exe` 和 JDK ZIP 一起打进去，所以最终用户不需要单独准备 WinSW 或安装 JDK。

## 5. 基本打包命令

只生成当前推荐的 `zip`：

```powershell
python ".\scripts\package_1panel_windows.py" `
  --version "v2.1.10" `
  --winsw-path "C:\tools\WinSW.exe" `
  --jdk-zip "D:\临时\1Panel\zulu21.50.19-ca-jdk21.0.11-win_x64.zip"
```

如果需要可选产物：

生成 `sfx.exe`：

```powershell
python ".\scripts\package_1panel_windows.py" `
  --version "v2.1.10" `
  --winsw-path "C:\tools\WinSW.exe" `
  --build-exe
```

生成 `msi`：

```powershell
python ".\scripts\package_1panel_windows.py" `
  --version "v2.1.10" `
  --winsw-path "C:\tools\WinSW.exe" `
  --build-msi `
  --wix-path "C:\Tools\wix\wix.exe" `
  --dotnet-root "C:\Tools\dotnet"
```

## 6. 最终用户安装方式

### 方式 A：推荐，交互式 ZIP 安装

1. 解压 `1panel-windows-vX.Y.Z.zip`
2. 双击 `install.cmd`
3. 脚本会自动申请管理员权限
4. 按提示填写：
   - 安装目录
   - 面板端口
   - 初始用户名
   - 初始密码
5. 安装前查看摘要并确认
6. 安装完成后，可选：
   - 打开面板
   - 打开安装目录
   - 创建桌面快捷方式

### 方式 B：管理员 PowerShell 直接安装

如果需要手工指定参数，也可以执行：

```powershell
powershell -ExecutionPolicy Bypass -File ".\install.ps1" `
  -InstallDir "D:\1Panel" `
  -PanelVersion "v2.1.10" `
  -PanelPort "18888" `
  -PanelUsername "admin" `
  -PanelPassword "Admin123!"
```

## 7. 安装器当前支持能力

交互安装器当前已经支持：

- 自动申请管理员权限
- 安装目录必须为绝对路径
- 面板端口范围校验
- 用户名、密码格式校验
- 检测目标目录内是否已有旧版 1Panel
- 检测端口是否已被占用
- 安装前显示配置摘要
- 可选创建桌面快捷方式
- 可选安装完成后自动打开浏览器
- 生成安装日志

## 8. 安装完成后的位置与入口

默认访问地址：

```text
http://127.0.0.1:9999
```

如果安装时修改了端口，则按自定义端口访问。

默认安装目录：

```text
C:\1Panel
```

安装后常见文件位置：

- `<install-dir>\1panel\conf\app.yaml`
- `<install-dir>\1panel\conf\1pctl.env`
- `<install-dir>\service`
- `<install-dir>\installer-logs`

安装后常见入口：

- `<install-dir>\install.cmd`
- `<install-dir>\uninstall.cmd`
- `<install-dir>\open-install-dir.cmd`

如果安装时选择创建桌面快捷方式，还会生成：

- `1Panel`
- `1Panel Install Directory`
- `1Panel Uninstall`

## 9. 卸载方式

推荐直接双击：

- `uninstall.cmd`

卸载脚本会：

- 自动申请管理员权限
- 让用户确认安装目录
- 停止并卸载 `1panel-core-service`
- 停止并卸载 `1panel-agent-service`
- 清理 1Panel 公共桌面快捷方式

## 10. 安装日志

交互安装时会自动生成安装日志：

```text
<install-dir>\installer-logs\install-YYYYMMDD-HHMMSS.log
```

如果用户反馈安装失败，优先查看这里。

## 11. 当前建议发布内容

当前建议统一交付给最终用户的文件只有一个：

1. `1panel-windows-vX.Y.Z.zip`

如果内部测试需要额外验证，也可以附带：

2. `1panel-windows-vX.Y.Z-sfx.exe`
3. `1panel-windows-vX.Y.Z.msi`

但正式对外仍以 `zip` 为准。
