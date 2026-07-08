#!/usr/bin/env bash
# 全量构建验证：官方同步 / 定制改动提交前运行。
# 用法: ./scripts/verify-build.sh
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> agent: build (linux + windows) / vet / test"
(cd agent && go build ./... && GOOS=windows go build ./... && go vet ./app/service/ && go test ./app/service/ -count=1)

echo "==> core: build (linux + windows)"
# core/cmd/server/web 通过 go:embed 内嵌前端构建产物（官方不入库）。
# 没跑过前端构建时垫两个占位文件让 embed 能解析（两者都在 .gitignore 内）。
[ -f core/cmd/server/web/index.html ] || touch core/cmd/server/web/index.html
[ -e core/cmd/server/web/assets ] || { mkdir -p core/cmd/server/web/assets && touch core/cmd/server/web/assets/placeholder; }
(cd core && go build ./... && GOOS=windows go build ./...)

if command -v npm >/dev/null 2>&1 && [ -d frontend/node_modules ]; then
    echo "==> frontend: type-check"
    (cd frontend && npm run type-check)
else
    echo "!! 跳过前端 type-check（缺 npm 或 frontend/node_modules 未安装）"
fi

echo "==> 全部验证通过"
