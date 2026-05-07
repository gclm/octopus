APP_NAME    := octopus
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
AUTHOR      := gclm
BINARY      := $(APP_NAME)
LD_FLAGS    := -X github.com/gclm/octopus/internal/conf.Version=$(VERSION) \
               -X github.com/gclm/octopus/internal/conf.Commit=$(COMMIT) \
               -X github.com/gclm/octopus/internal/conf.BuildTime=$(BUILD_TIME) \
               -X github.com/gclm/octopus/internal/conf.Author=$(AUTHOR)

.PHONY: dev build build-be build-fe run clean test lint

# 构建全部（前端 + 后端）
build: build-fe build-be

# 构建 Go 二进制（嵌入 static/out）
build-be:
	CGO_ENABLED=1 go build -ldflags "$(LD_FLAGS)" -o $(BINARY) .

# 构建前端并复制到 static/out
build-fe:
	cd web && pnpm install && pnpm build
	rm -rf static/out
	cp -r web/out static/out

# 运行编译后的二进制
run: clean build
	./$(BINARY) start

# 启动开发环境（前后端热重载）
dev:
	@make -j2 dev-fe dev-be

dev-fe:
	cd web && pnpm dev

dev-be:
	go run . start

# 运行测试
test:
	go test ./...

# 代码检查
lint:
	go vet ./...

# 清理构建产物
clean:
	rm -rf static/out web/out web/.next $(BINARY)
