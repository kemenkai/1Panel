# 官方代码同步指南

本仓库是官方 [1Panel](https://github.com/1Panel-dev/1Panel) 的私有定制 fork。

## 仓库结构

- **基线**：官方 `v2.1.10`（经 blob 哈希逐 tag 比对确认）
- **`custom` 分支**：基线 + 全部定制（一个导入提交起步，此后正常迭代）
- **remote 约定**：
  - `upstream` → 官方仓库（只 fetch，永不 push）
  - `origin` → 我们的私有仓库

定制足迹（权威清单随每次同步用 `git diff --name-status <基线>..custom` 重新生成）：

| 清单 | 说明 |
|---|---|
| `docs/upstream-sync/modified-official-files.list` | 被我们改写过的官方文件 —— **合并冲突几乎只会出现在这里** |
| `docs/upstream-sync/custom-new-files.list` | 纯新增的定制文件 —— 官方不存在，不会冲突 |

定制主体：服务管理跨平台（Windows WinSW / Linux systemd）、交付包服务类型（compose/JAR 自动展开）、enhance 扩展（simple-node、本地应用发布）、Windows 平台支持、zh/en 文案。

## 日常同步流程

```bash
# 1. 查看官方有哪些新版本
./scripts/sync-upstream.sh

# 2. 合并选定的官方 tag（自动跑构建验证）
./scripts/sync-upstream.sh v2.2.3

# 3. 无冲突且验证绿 → push
git push origin custom
```

## 冲突处理

冲突文件几乎必然在 `modified-official-files.list` 里（约 80 个，其中大半是 ≤10 行的挂载点：路由注册、migration 注册、i18n 段落）。处理原则：

1. **挂载点类**（router/common.go、migrate.go、lang loader 等一两行注册）：保留双方——官方的新内容 + 我们的注册行。
2. **i18n（zh.ts / en.ts）**：官方改动和我们的 `windowsService` / `enhance*` 段落通常不重叠，两边都保留。
3. **重改文件**（dashboard.go、api/app.go、core files.go、home/index.vue 等）：逐块人工裁决，优先保住定制语义，再吸收官方修复。
4. 解决后：`git add -A && git commit`，然后 `./scripts/verify-build.sh` 必须全绿。

## 大版本合并注意

- 官方在 v2.2.x 对 xpack 挂钩、setting 子页面做过删除/重构；merge 时若出现 modify/delete 冲突，先看官方对应功能去哪了，再决定跟随删除还是移植定制。
- 合并跨度大时（隔多个 minor 版本），建议逐 tag 递进合并（v2.1.13 → v2.2.1 → …），每步都跑验证，问题好定位。
- 合并后除构建验证外，人工冒烟：服务管理页（创建/启停/日志）、enhance 三个入口。

## 环境备注

- 本仓库位于 Linux 原生分区；**不要**把仓库放回 NTFS 挂载（权限位混乱会制造假 diff）。
- 克隆用了 `--filter=blob:none`（partial clone）：历史完整、按需取 blob，首次 blame/checkout 旧版本时需要联网。
- 本地构建产物、交付包（`*.tar.xz`）、AI 工具目录等均已在 `.gitignore` 排除，不要强行 `git add -f`。
