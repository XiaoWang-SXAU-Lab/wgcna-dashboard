# wgcna-dashboard

WGCNA 共表达网络分析的 Web dashboard：**Go 后端 + Vue3 前端 + Rust worker**（复用
[`rs-wgcna`](https://github.com/XiaoWang-SXAU-Lab/rs-wgcna) 内核）。

支持 one-step（一键全流程）与 step-by-step（分步，改参只重算下游）两种模式、交互式
可视化（ECharts 树状图/热图/散点 + AG Grid 表格）、单二进制部署，并内置 AI 领航员
（OpenAI 兼容多厂商 LLM + 本地向量 RAG + 错误诊断 + 加密 API key）。

> ⚠️ 项目进行中（Phase 0）。核心计算已完成（rs-wgcna），本仓库当前只含契约与骨架。

## 架构

```
Web 前端 (Vue3+ECharts+AG Grid)
   │ HTTP / SSE
Go 后端 (API + 作业调度 + 文件管理)
   │ spawn + 文件/进度
Rust worker (rs-wgcna-worker --job-dir --step)
```

## 目录

```
backend/    # Go 后端
frontend/   # Vue3 前端
schemas/    # JSON 契约（参数 / 7 步状态机 / 结果）
```

## 契约（schemas/）

- [`params.schema.json`](schemas/params.schema.json) — 7 步参数定义（A/B/C 三档）
- [`pipeline.schema.json`](schemas/pipeline.schema.json) — 7 步状态机 + 依赖 + one-step 顺序
- [`results.schema.json`](schemas/results.schema.json) — 每步输出结构（对齐 worker）

## 构建

```bash
# 后端
cd backend && go build ./cmd/server

# 前端（需 Node/pnpm）
cd frontend && pnpm install && pnpm build

# worker（在 rs-wgcna 仓库）
cargo build --release --bin rs-wgcna-worker
```

## License

[MIT](LICENSE) © 2026 Shawn Wang (Xiao Wang-SXAU-Lab) and contributors
