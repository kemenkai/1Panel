# 官方源码同步：一劳永逸方案（非 AI 机械升级）

目标：让**普通开发者（不靠 AI）**能按 checklist 安全跟进官方 1Panel 的 patch/minor 升级；自己的定制功能不受影响。

## 一、诚实的可达性评估

"完全零冲突"不现实——官方持续演进新行为，无法预测。但可以把**需要人判断**的冲突压到很低：

| 场景 | 现状（本次同步经验）| 目标（做完重构后）|
|---|---|---|
| 被改官方文件数 | ~85 | ~10±3 |
| 常规 patch（如 v2.2.x→v2.2.y）需人判断 | 全靠 AL | **0–3 处** |
| minor（v2.1→v2.2）需人判断 | 全靠 AI | 3–8 处 |
| major（前端整栈迁移等）| 全靠 AI | 5–10 处，走独立 playbook |

结论：**patch/minor 可做到「脚本机械完成 + 人复核几处」，不需要 AI**；major 版本（tailwind/vite 大迁移那种）无法避免，但单独流程处理、频率低。

## 二、四类反复冲突的消灭/降级手段

过去每次 merge 必须 AI，是这四类痛点造成的：

### 1. 跨平台 syscall 适配（最高频）
官方新代码用 unix-only 调用（`syscall.Setpgid/Kill/Statfs/Stat_t`、`x/sys/unix`），我们要 Windows 编译就得改 build-tag 辅助。
- **消灭手段**：建稳定的 `internal/platform` 能力层（agent/core 各一套），已有雏形（`process_unix/windows.go`、`ownership_*.go`、`upgrade_space_*.go`、`socket_*.go`）——把它们收拢成命名一致的少数几个文件，官方新用法一律接到这层。
- **无法自动消失的部分**：官方每次新引入 syscall 用法，首次仍需把调用点换成辅助函数（一两行）。但可以**自动检测**：加一个 `go/analysis` AST 门禁，扫描本次 merge 引入的增量里有没有「未加 build-tag 的 syscall/x-sys-unix/Stat_t」，命中就明确报出文件:行 + 该用哪个辅助函数替换。`GOOS=windows go build ./...` 作兜底（现已在 CI）。→ 从「AI 找+改」降到「脚本指位置、人机械替换」。

### 2. 前端整栈大迁移（低频、巨大）
v2.2.1 的 tailwind3→4/vite7→8/pinia2→3/TS5→6 这类。
- **无法规避**：官方换工具链就是换。
- **降级**：把定制页面与工具链配置分离；sync 脚本检测到 `package.json` 有 major 依赖跳变时，自动标记「进入大版本前端流程」——单独分支跑 `npm ci`/type-check/build/冒烟，人工处理类型错误。频率约半年一次，接受为独立 playbook。

### 3. 挂载点常规冲突（路由/迁移/i18n 双方各加东西）
- **消灭手段**（把 ~49 处挂载改动收敛为 4–6 个稳定扩展点）：
  - 后端路由：显式 registry，`init()` 只登记不执行副作用；官方路由列表零改动。
  - 迁移：`RegisterMigration()` 追加到列表尾部 + 唯一 ID 校验 + 确定排序；官方迁移列表零改动。
  - 前端路由：已支持 `import.meta.glob('./modules/*.ts')` 自动扫描——enhance 路由本可零接触（本次仍改了，可回收）。
  - i18n：定制段搬独立文件，loader 做 deep-merge + 拒绝重复键；官方 zh.ts/en.ts 零改动。
- 这几个扩展点**一次建好，官方后续 merge 几乎不再碰**。

### 4. lockfile 平台坑
官方 lockfile 缺 linux 原生 binding、每次升依赖失步。
- **消灭手段**：固定 Node/npm 版本，在干净环境跑 `npm install --package-lock-only --include=optional`，断言含 linux 原生 binding 后再 `npm ci`——**全自动进 sync 流程**。仅当 lockfile 格式升级/包缺陷时才失败退出需人工。

## 三、非 AI 的机械升级 checklist（sync-upstream.sh 增强版职责）

普通开发者执行 `./scripts/sync-upstream.sh <tag>`，脚本按以下步骤，每步失败都给**明确的文件+规则+恢复命令**：

1. **前置校验**：工作区干净、upstream 已配、目标 tag 存在且是当前的后代、创建同步分支 + 记录回退点。
2. **预演分类**（`git merge-tree` 干跑）：把预期冲突分成四类并出报告——① 可脚本预解的挂载点；② 已知平台 patch（build-tag 适配）；③ 永久行为 patch（清单在 `docs/upstream-sync/permanent-patches.md`）；④ **未知**。**出现「未知」类→停止并要求人工介入**（这是唯一需要判断的入口，且已定位到具体文件）。
3. **合并 + 自动预解**：`merge --no-commit`；脚本按规则自动解已知挂载点冲突；已知平台 patch 用 `git apply --3way --check` 通过才重放，失败给出文件+规则+恢复命令。
4. **重建 + 门禁**：干净环境重建 lockfile；跑 AST 平台门禁、双模块 linux/windows build、vet/test、前端 type-check/build、升级脚本沙盘测试。
5. **人工只做两件事**：复核「永久行为 patch」是否还成立 + 冒烟服务管理/enhance 入口。通过后才 commit，并自动刷新基线与 `docs/upstream-sync/` 足迹清单。

配套产物：
- `docs/upstream-sync/permanent-patches.md`：每个永久 patch 一条（文件、为什么、merge 时怎么处理）。
- `docs/upstream-sync/mount-points.md` + 预解脚本：每个挂载点的固定解法。
- `scripts/platform-lint.go`（AST 门禁）：检测未加标签的平台专有调用。

## 四、fork 维护范式：继续 merge tag（不换）

- **merge 官方 tag（现状，保留）**：保留官方祖先、不改写已发布历史、共同 merge-base 让后续合并有依据。✅ 最适合。
- rebase 定制 commits：每次逐提交重复解冲突，更痛。✗
- git subtree：不适合覆盖同一源码树。✗
- quilt/语义 patch series：适合承载「少量永久 patch + 检测规则」，不宜作 137 个新增文件的主历史。→ 作为第三类冲突的**表达方式**采用，不作主范式。

## 五、分阶段落地与优先级（收益排序）

| 阶段 | 内容 | 消灭的痛点 | 收益 |
|---|---|---|---|
| **P1** | 代码搬迁（41 处 reducible→自有文件）+ 建 4–6 个扩展点（registry/glob/i18n-merge）| 痛点 3 挂载点 | 常规冲突大降，最高性价比 |
| **P2** | 稳定 `internal/platform` 层 + AST 平台门禁进 CI + sync 脚本增强（分类/预解/patch 重放）| 痛点 1 syscall | 跨平台适配从「AI 改」变「脚本指位置人替换」|
| **P3** | lockfile 容器化重生成进 sync 流程 | 痛点 4 | 全自动 |
| **P4** | 持续把 23 处通用修复提官方 PR | 痛点 1 的一部分 | 每合入一个永久归零 |
| **P5** | 前端大版本 playbook 文档化 | 痛点 2 | 低频，接受 |

先做 P1（挂载点+搬迁），做完观察一次官方 merge 实测冲突面收益，再推 P2。详细分离清单见 `.ccg/tasks/separate-custom-code/plan.md`。

## 六、一句话总结

**能做到「一劳永逸」的是：把可收敛的挂载点/跨平台适配一次性重构成扩展点+能力层，配一个会分类、会预解、会门禁、会在遇到未知时明确喊停的 sync 脚本**——之后 patch/minor 升级由脚本机械完成、人只复核几处，不需要 AI。**做不到的是**：官方真正的新行为（尤其前端大版本迁移）——这类固有冲突只能靠文档化 playbook + 人工处理，但频率低、有清单指引。
