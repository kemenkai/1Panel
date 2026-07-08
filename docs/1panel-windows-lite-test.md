# 1Panel Windows Lite 测试文档

这份文档用于指导在 Windows 机器上验证当前 `1Panel Windows Lite` 第一版骨架是否可用。

## 测试目标

本轮测试重点不是追求全功能通过，而是先验证以下链路：

1. Windows 下是否可以成功构建 `1panel-core.exe` 和 `1panel-agent.exe`
2. 是否可以生成 `app.yaml`、`1pctl.env`、`WinSW xml`
3. 是否可以注册并启动 `1panel-core` / `1panel-agent` Windows 服务
4. 是否可以打开面板并进入 `Windows 服务` 页面
5. 是否可以创建一个最小 Java 服务定义并生成对应服务包装文件

## 环境准备

请先在 Windows 机器上准备：

- `Git`
- `Go`，建议 `1.22+`
- `Node.js`，建议 `18+` 或 `20+`
- `npm`
- `PowerShell`
- `WinSW.exe`
- 如果要测试 Docker，再额外安装 `Docker Desktop`

建议把仓库放在类似目录：

```powershell
D:\work\1Panel
```

## 测试步骤

### 1. 仅构建，不安装服务

进入仓库根目录后执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_install_1panel_windows.ps1 -SkipServiceInstall
```

预期结果：

- 成功生成：
  - `C:\1Panel\bin\1panel-core.exe`
  - `C:\1Panel\bin\1panel-agent.exe`
- 成功生成配置：
  - `C:\1Panel\1panel\conf\app.yaml`
  - `C:\1Panel\1panel\conf\1pctl.env`
- 成功生成服务包装定义：
  - `C:\1Panel\service\1panel-core-service.xml`
  - `C:\1Panel\service\1panel-agent-service.xml`

### 2. 安装 Windows 服务

准备好 `WinSW.exe` 后执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_install_1panel_windows.ps1 -WinSWPath "C:\tools\WinSW.exe"
```

然后执行：

```powershell
Get-Service *1panel*
```

或：

```powershell
sc.exe query 1panel-core-service
sc.exe query 1panel-agent-service
```

预期结果：

- 两个服务都存在
- 至少 `1panel-core-service` 能正常启动

### 3. 打开面板

默认访问地址：

```text
http://127.0.0.1:9999
```

预期结果：

- 登录页可以正常打开
- 登录后不报 500/502
- `工具箱` 页面在 Windows 下会自动落到 `Windows 服务`

### 4. 测试 Windows 服务页面

进入 `工具箱 -> Windows 服务` 后：

1. 点击创建
2. 选择 `Java 服务模板`
3. 填最小必要字段：
   - `name`
   - `displayName`
   - `execPath`
   - `jarPath`
   - `winswPath`
4. 保存

预期结果：

- 能保存成功
- 生成：
  - `C:\1Panel\service\<name>\<name>.xml`
  - `C:\1Panel\service\<name>\<name>.cmd`
  - `C:\1Panel\service\<name>\<name>.env`

### 5. 测试启停操作

在 `Windows 服务` 列表中，依次测试：

- 启动
- 停止
- 重启
- 编辑后保存
- 删除

预期结果：

- 页面操作不报错
- 服务状态能回写到列表
- 删除时有确认框

## DLL 场景专项测试

如果本次要验证 `Java + DLL` 场景，建议先不要直接上交付包，而是先准备一个最小测试程序：

- 一个最小 Java 程序
- 一个第三方 DLL 目录

创建服务时重点填写：

- `serviceType = dll` 或 `java`
- `dllDir`
- `workDir`
- `execPath`
- `jarPath`

重点检查：

- 生成的 `.cmd` 是否把 `DLL_DIR` 注入到环境中
- 运行时 Java 是否能找到 DLL

## 交付包场景专项测试

如果验证交付包：

1. 先创建 `交付包服务模板`
2. 检查预填参数是否合理
3. 根据实际目录修改：
   - `jarPath`
   - `dllDir`
   - `configPath`
   - `workDir`

重点检查：

- 是否能生成完整服务包装文件
- 是否能正常启动服务

## 回传信息清单

如果测试失败，请优先回传这些内容：

1. 执行的 PowerShell 命令
2. PowerShell 完整输出
3. `C:\1Panel\1panel\conf\app.yaml`
4. `C:\1Panel\1panel\conf\1pctl.env`
5. `C:\1Panel\service\<service-name>\*.xml`
6. `C:\1Panel\service\<service-name>\*.cmd`
7. `Get-Service *1panel*` 输出
8. 浏览器报错截图

## 当前已知边界

当前版本已经支持 Windows 服务创建、配置编辑、日志查看、Java/交付包上传和增强功能入口。以下能力仍需要按客户环境继续验证：

- Docker Desktop / Docker Compose 环境下的完整交付包编排启动
- Linux 主控对 Windows 节点的正式纳管链路
- 文件选择器辅助录入
- Windows 概览页在不同 Windows Server 版本上的硬件信息完整性

交付包模板包上传已改为按上传批次生成独立模板目录，避免覆盖删除正在运行或被 Docker、杀毒软件、资源管理器占用的旧目录。

## 建议测试顺序

建议按以下顺序执行，便于定位问题：

1. 构建脚本
2. 服务安装
3. 面板访问
4. Windows 服务页面
5. Java 服务
6. DLL 服务
7. 交付包服务
