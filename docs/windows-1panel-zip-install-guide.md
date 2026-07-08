# Windows 版 1Panel ZIP 安装手册

本文档面向最终使用人员，说明如何在 Windows 电脑上通过 `ZIP` 安装包安装、登录和卸载 `1Panel`。

本文档不涉及源码编译，不需要安装 `Go`、`Node.js` 或其他开发环境。

如果需要带截图版本，请参考：

- [windows-1panel-zip-install-guide-with-screenshots.md](./windows-1panel-zip-install-guide-with-screenshots.md)

## 1. 需要准备的文件

请准备以下安装包：

- `1panel-windows-v2.1.10.zip`

## 2. 安装前准备

安装前请确认：

- 当前电脑为 Windows 10 或 Windows 11
- 当前账号具备管理员权限
- 电脑可以正常打开浏览器
- 计划使用的面板端口没有被其他程序占用

建议默认使用：

- 安装目录：`C:\1Panel`
- 面板端口：`9999`
- 初始用户名：`admin`

## 3. 开始安装

### 步骤 1：解压安装包

把 `1panel-windows-v2.1.10.zip` 解压到任意目录，例如：

```text
D:\install\1panel-windows-v2.1.10
```

解压后可以看到这些文件：

- `install.cmd`
- `install-interactive.ps1`
- `install.ps1`
- `uninstall.cmd`
- `uninstall-interactive.ps1`
- `uninstall.ps1`
- `open-install-dir.cmd`
- `README.txt`
- `bin`
- `tools`

### 步骤 2：运行安装入口

双击：

- `install.cmd`

脚本会自动申请管理员权限。  
如果系统弹出权限确认窗口，请选择“是”。

### 步骤 3：按提示填写安装信息

安装程序会依次询问：

1. 安装目录
2. 面板端口
3. 初始用户名
4. 初始密码

如果直接按回车，则使用默认值。

推荐示例：

```text
Install directory: C:\1Panel
Panel port: 9999
Panel username: admin
Panel password: Admin123!
```

### 步骤 4：确认安装摘要

安装前会显示摘要信息，包括：

- 安装目录
- 面板端口
- 用户名
- 安装模式
- 面板访问地址

确认无误后输入：

```text
Y
```

然后继续安装。

### 步骤 5：可选创建桌面快捷方式

安装过程中会询问是否创建桌面快捷方式。

如果选择 `Y`，桌面会生成：

- `1Panel`
- `1Panel Install Directory`
- `1Panel Uninstall`

建议选择 `Y`，后续使用更方便。

## 4. 安装完成后如何登录

安装完成后，程序会提示是否立即打开面板。

如果选择 `Y`，浏览器会自动打开。

如果没有自动打开，也可以手工访问：

```text
http://127.0.0.1:9999
```

如果安装时修改了端口，请把 `9999` 换成实际填写的端口。

登录账号使用安装时填写的：

- 用户名
- 密码

## 5. 安装完成后文件在哪里

如果使用默认目录，安装位置在：

```text
C:\1Panel
```

常见目录说明：

- `C:\1Panel\bin`
  - 程序主文件
- `C:\1Panel\1panel\conf`
  - 面板配置文件
- `C:\1Panel\service`
  - Windows 服务包装文件
- `C:\1Panel\installer-logs`
  - 安装日志

安装后也可以双击：

- `open-install-dir.cmd`

直接打开安装目录。

## 6. 安装日志在哪里

安装程序会自动生成日志，目录为：

```text
<安装目录>\installer-logs\
```

例如：

```text
C:\1Panel\installer-logs\
```

日志文件名称类似：

```text
install-20260430-153000.log
```

如果安装失败，请优先查看这里。

## 7. 如何卸载

### 方式 1：桌面快捷方式卸载

如果安装时创建了桌面快捷方式，直接双击：

- `1Panel Uninstall`

### 方式 2：安装目录内卸载

进入安装目录后，双击：

- `uninstall.cmd`

卸载脚本会：

- 自动申请管理员权限
- 让你确认安装目录
- 停止并移除 1Panel 服务
- 清理 1Panel 桌面快捷方式

## 8. 常见问题

### 1. 双击 `install.cmd` 没反应

请确认：

- 安装包已经完整解压
- 当前电脑允许弹出管理员授权窗口
- 杀毒软件没有拦截脚本执行

### 2. 浏览器打不开面板

请依次检查：

- 是否安装完成
- 端口是否填写正确
- 服务是否已启动
- 是否被防火墙或其他程序占用了端口

默认访问地址：

```text
http://127.0.0.1:9999
```

### 3. 忘记安装目录

可通过以下方式查找：

- 双击桌面的 `1Panel Install Directory`
- 双击安装包中的 `open-install-dir.cmd`
- 在常见默认目录 `C:\1Panel` 下查看

### 4. 安装时提示端口已被占用

请重新运行安装程序，并改用其他端口，例如：

- `18888`
- `19999`

## 9. 建议交付方式

如果需要把安装包交给其他同事或客户，建议同时交付：

1. `1panel-windows-v2.1.10.zip`
2. 本文档

这样对方不需要了解源码仓库，也不需要自己拼命令。
