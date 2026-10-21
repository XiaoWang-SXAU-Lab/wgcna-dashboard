# wgcna-dashboard 单二进制构建
# 产出: ./wgcna-dashboard（Go 内嵌前端 dist + Rust worker 二进制）

RS_WGCNA ?= ../rs_WGCNA
WORKER_SRC := $(RS_WGCNA)/target/release/rs-wgcna-worker
WORKER_DST := backend/internal/worker/worker-bin
DIST_SRC := frontend/dist
DIST_DST := backend/internal/web/dist

.PHONY: worker frontend build dev clean

## 构建 Rust worker 并拷入 embed 目录
worker:
	cd $(RS_WGCNA) && cargo build --release --bin rs-wgcna-worker
	cp $(WORKER_SRC) $(WORKER_DST)

## 构建前端并拷入 embed 目录
frontend:
	cd frontend && pnpm install && pnpm build
	rm -rf $(DIST_DST)
	cp -r $(DIST_SRC) $(DIST_DST)

## 单二进制（release：内嵌 worker + 前端）
build: worker frontend
	cd backend && go build -tags release -o ../wgcna-dashboard ./cmd/server

## 开发模式（前端走 Vite，worker 走 WGCNA_WORKER 环境变量）
dev:
	cd backend && go run ./cmd/server

clean:
	rm -f $(WORKER_DST)
	rm -rf $(DIST_DST)
	rm -f wgcna-dashboard
