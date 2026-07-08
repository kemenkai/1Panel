# 1Panel Windows Package Smoke Test

本文档用于在另一台 Windows 电脑上快速验证当前 `1Panel` Windows 安装包。

## 1. 当前测试基线

当前默认测试对象已经调整为：

- `1panel-windows-v2.1.10.zip`

说明：

- `zip` 是当前推荐交付物
- `MSI` 只保留为可选构建产物
- 如无特殊要求，测试时不再以 `MSI` 为主流程

## 2. 测试文件

从以下目录拷贝测试产物：

- [build/packages/windows](/D:/临时/1Panel/build/packages/windows)

至少带上：

- `1panel-windows-v2.1.10.zip`

如需兼容性补充验证，可额外带上：

- `1panel-windows-v2.1.10-sfx.exe`
- `1panel-windows-v2.1.10.msi`

## 3. 测试环境建议

- Windows 10 或 Windows 11
- 具备管理员权限
- 机器上未安装旧版 `1Panel`
- 预留一个空闲端口，例如 `9999` 或 `18888`

## 4. ZIP 安装测试

1. 解压 `1panel-windows-v2.1.10.zip`
2. 双击 `install.cmd`
3. 验证会自动申请管理员权限
4. 按提示输入以下测试值：

```text
InstallDir: C:\1PanelTest
PanelPort: 18888
PanelUsername: panel.user
PanelPassword: Admin123!
```

5. 安装前确认摘要信息正确
6. 选择创建桌面快捷方式
7. 安装完成后选择打开面板

## 5. 安装后检查

检查目录：

- `C:\1PanelTest\bin`
- `C:\1PanelTest\1panel\conf`
- `C:\1PanelTest\service`
- `C:\1PanelTest\installer-logs`

检查文件：

- `C:\1PanelTest\1panel\conf\app.yaml`
- `C:\1PanelTest\1panel\conf\1pctl.env`

应包含：

- 端口：`18888`
- 用户名：`panel.user`
- 密码：`Admin123!`

## 6. 服务检查

打开服务管理器或 PowerShell，确认以下服务存在：

- `1panel-core-service`
- `1panel-agent-service`

PowerShell 可执行：

```powershell
Get-Service "1panel-core-service","1panel-agent-service"
```

状态应为：

- `Running`

## 7. Web 访问检查

浏览器打开：

```text
http://127.0.0.1:18888
```

验证：

1. 页面可以正常打开
2. 可用 `panel.user / Admin123!` 登录
3. 登录后首页正常显示

## 8. 桌面快捷方式检查

如果安装时选择创建桌面快捷方式，验证桌面存在：

- `1Panel`
- `1Panel Install Directory`
- `1Panel Uninstall`

验证：

1. 双击 `1Panel` 可打开面板地址
2. 双击 `1Panel Install Directory` 可打开安装目录
3. 双击 `1Panel Uninstall` 可进入卸载流程

## 9. 安装日志检查

检查目录：

- `C:\1PanelTest\installer-logs`

验证：

1. 存在 `install-*.log`
2. 日志中能看到安装过程输出

## 10. 非法输入校验

重新运行 `install.cmd`，至少验证以下场景中的一个：

- 用户名输入 `ab`
- 密码输入 `bad=pass`
- 端口输入 `70000`
- 端口填一个已占用端口

期望结果：

- 安装器给出明确提示
- 不会继续写入错误配置

## 11. 卸载检查

双击：

- `C:\1PanelTest\uninstall.cmd`

验证：

1. 卸载前会要求确认安装目录
2. 卸载过程无报错
3. 服务被移除
4. 桌面上的 1Panel 快捷方式被清理

## 12. 记录建议

建议测试时记录以下内容：

- 测试机器系统版本
- 使用的安装包文件名
- 安装时填写的参数
- 是否成功启动服务
- 是否成功访问 Web
- 是否成功生成桌面快捷方式
- 是否成功生成安装日志
- 是否成功卸载
