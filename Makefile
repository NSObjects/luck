# ===== 项目路径与变量 =====
APP_NAME       ?= ssq-app
BACKEND_DIR    ?= backend
FRONTEND_DIR   ?= web
EMBED_DIST     ?= $(BACKEND_DIR)/web/dist
BIN_DIR        ?= $(BACKEND_DIR)/bin
BIN_PATH       ?= $(BIN_DIR)/$(APP_NAME)

# 版本信息（可选）
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT         ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE           ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS        ?= -s -w \
  -X 'main.BuildVersion=$(VERSION)' \
  -X 'main.BuildCommit=$(COMMIT)' \
  -X 'main.BuildDate=$(DATE)'

# sqlite 驱动是 mattn/go-sqlite3 → 需要 CGO
CGO            ?= 1

.PHONY: help build run clean tidy web-install web-build embed-check backend-build \
        release-linux-amd64 release-darwin-arm64 vercel-static

help:
	@echo "常用命令："
	@echo "  make build            # 前端打包(到 backend/web/dist) + 后端编译二进制(内嵌前端)"
	@echo "  make run              # 运行编译好的二进制"
	@echo "  make clean            # 清理二进制与打包产物"
	@echo "  make tidy             # go mod tidy / npm install"
	@echo "  make vercel-static    # 仅构建前端到 web/dist（用于 Vercel 静态站点）"
	@echo ""
	@echo "发布（需本机满足对应平台 CGO 工具链）："
	@echo "  make release-linux-amd64"
	@echo "  make release-darwin-arm64"

# 一键构建（默认）
build: web-build backend-build
	@echo "✅ Build done: $(BIN_PATH)"

# 前端依赖
web-install:
	cd $(FRONTEND_DIR) && npm ci

# 前端打包到 backend/web/dist（供 go:embed 使用）
web-build: web-install
	cd $(FRONTEND_DIR) && npm run build
	@test -f "$(EMBED_DIST)/index.html" || (echo "❌ 找不到 $(EMBED_DIST)/index.html，请确认 web/vite.config.js 的 outDir 已设为 '../backend/web/dist'"; exit 1)

# 确保嵌入文件存在
embed-check:
	@test -f "$(EMBED_DIST)/index.html" || (echo "❌ 缺少前端产物，请先执行: make web-build"; exit 1)

# 后端编译二进制（内嵌前端）
backend-build: embed-check
	mkdir -p $(BIN_DIR)
	cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO) go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BIN_PATH)" .

# 本地运行
run: build
	./$(BIN_PATH)

# 清理
clean:
	rm -rf "$(BIN_DIR)"
	rm -rf "$(EMBED_DIST)"
	cd $(FRONTEND_DIR) && rm -rf dist

# 依赖整备
tidy:
	cd $(BACKEND_DIR) && go mod tidy
	cd $(FRONTEND_DIR) && npm install

# 发行版（注意：由于使用 mattn/go-sqlite3，需要 CGO 交叉编译工具链）
release-linux-amd64: web-build
	mkdir -p $(BIN_DIR)
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BIN_PATH)-linux-amd64" .

release-darwin-arm64: web-build
	mkdir -p $(BIN_DIR)
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BIN_PATH)-darwin-arm64" .

# —— 仅用于将前端部署到 Vercel（静态站点）——
# 结果输出到 web/dist；请在 Vercel 项目里把 Root 设为 'web'，Output Directory 设为 'dist'
vercel-static:
	cd $(FRONTEND_DIR) && npm ci && npm run build
	@echo "✅ 前端已构建到 $(FRONTEND_DIR)/dist"
	@echo "➡️  Vercel 设置：Root=web，Output Directory=dist；API 请部署到独立服务（本二进制不在 Vercel 常驻运行）"
