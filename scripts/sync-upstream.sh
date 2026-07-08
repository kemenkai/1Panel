#!/usr/bin/env bash
# 同步官方 1Panel 更新到定制分支。
#
# 用法:
#   ./scripts/sync-upstream.sh              # 拉取官方并列出尚未合并的新 tag
#   ./scripts/sync-upstream.sh v2.2.3       # 把官方 v2.2.3 合并进当前分支
#   ./scripts/sync-upstream.sh v2.2.3 --skip-verify   # 合并但跳过构建验证
#
# 前置: remote `upstream` 指向官方仓库 (github.com/1Panel-dev/1Panel)。
# 冲突排查: 见 docs/UPSTREAM-SYNC.md 与 docs/upstream-sync/ 下的定制点位清单。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"

echo "==> git fetch ${UPSTREAM_REMOTE} --tags"
git fetch "$UPSTREAM_REMOTE" --tags --prune

if [ $# -eq 0 ]; then
    BASE_TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)"
    echo ""
    echo "==> 当前基线: ${BASE_TAG}；尚未合并的更新官方 tag（新 → 旧，最多 15 个）:"
    count=0
    for t in $(git tag -l 'v*' --sort=-v:refname); do
        # 只看比当前基线更新的 tag，跳过发布分支上的历史 tag
        [ "$(printf '%s\n%s\n' "$BASE_TAG" "$t" | sort -V | tail -1)" = "$t" ] || continue
        [ "$t" = "$BASE_TAG" ] && continue
        if ! git merge-base --is-ancestor "$t" HEAD 2>/dev/null; then
            echo "    $t"
            count=$((count + 1))
            [ "$count" -ge 15 ] && break
        fi
    done
    [ "$count" -eq 0 ] && echo "    （无 —— 已与官方最新 tag 同步）"
    echo ""
    echo "选择一个 tag 后运行: $0 <tag>"
    exit 0
fi

TAG="$1"
git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null || { echo "!! tag ${TAG} 不存在（先运行不带参数的本脚本查看可用 tag）"; exit 1; }
BRANCH="$(git branch --show-current)"
[ -n "$BRANCH" ] || { echo "!! 当前处于 detached HEAD，请先切回定制分支（如 custom）"; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "!! 工作区有未提交改动，请先提交或 stash"; exit 1; }

echo "==> 合并官方 ${TAG} 到分支 ${BRANCH}"
if ! git merge --no-ff "$TAG" -m "chore: merge upstream ${TAG}"; then
    echo ""
    echo "!! 合并冲突，冲突文件:"
    git diff --name-only --diff-filter=U | sed 's/^/    /'
    echo ""
    echo "排查提示:"
    echo "  1. 对照 docs/upstream-sync/modified-official-files.list —— 我们改过的官方文件，冲突通常在这里，需人工合并双方改动。"
    echo "  2. docs/upstream-sync/custom-new-files.list 中的文件是纯定制文件，一般不会冲突。"
    echo "  3. 解决全部冲突后: git add -A && git commit，然后运行 ./scripts/verify-build.sh。"
    exit 1
fi

if [ "${2:-}" != "--skip-verify" ]; then
    ./scripts/verify-build.sh
fi

echo "==> 同步 ${TAG} 完成。检查无误后 push 到私有远程。"
