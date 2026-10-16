# Magicmail 编译与部署
# 用法: make build / make run / make install

APP_NAME := magicmail
VERSION := 1.0.0
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")

# Go 参数
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

.PHONY: all build run clean install uninstall help

all: build

## build: 编译二进制文件
build:
	@echo "🔨 编译 $(APP_NAME) v$(VERSION)..."
	go build $(LDFLAGS) -o bin/$(APP_NAME) .
	@echo "✅ 编译完成: bin/$(APP_NAME)"

## run: 直接运行（开发模式）
run:
	@echo "🚀 运行开发服务器..."
	go run .

## dev: 热重载开发（需要安装 air，go install github.com/cosmtrek/air@latest）
dev:
	air

## clean: 清理构建产物
clean:
	@echo "🧹 清理构建产物..."
	rm -rf bin/
	rm -rf data/
	@echo "✅ 已清理"

## tidy: 整理依赖
tidy:
	go mod tidy
	go mod verify

## install: 安装到系统路径 (systemd 服务)
install: build
	@echo "📦 安装 $(APP_NAME)..."
	sudo cp bin/$(APP_NAME) /usr/local/bin/
	sudo chmod 755 /usr/local/bin/$(APP_NAME)
	sudo mkdir -p /var/lib/magicmail/data
	sudo mkdir -p /etc/magicmail
	[ -f magicmail.service ] && sudo cp magicmail.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "✅ 安装完成"

## uninstall: 从系统移除
uninstall:
	@echo "🗑️ 卸载 $(APP_NAME)..."
	sudo systemctl stop $(APP_NAME) 2>/dev/null || true
	sudo systemctl disable $(APP_NAME) 2>/dev/null || true
	sudo rm -f /usr/local/bin/$(APP_NAME)
	sudo rm -f /etc/systemd/system/$(APP_NAME).service
	sudo systemctl daemon-reload
	@echo "✅ 已卸载"

## start: 启动 systemd 服务
start:
	sudo systemctl start $(APP_NAME)
	sudo systemctl status $(APP_NAME) --no-pager

## stop: 停止 systemd 服务
stop:
	sudo systemctl stop $(APP_NAME)

## restart: 重启服务
restart:
	sudo systemctl restart $(APP_NAME)
	sudo systemctl status $(APP_NAME) --no-pager

## logs: 查看日志
logs:
	journalctl -u $(APP_NAME) -f --no-pager

## help: 显示帮助信息
help:
	@echo "$(APP_NAME) v$(VERSION) - 邮件代收服务"
	@echo ""
	@echo "可用命令:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
