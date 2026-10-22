<p align="center"><img src="frontend/public/rsWGCNA_icon.png" alt="rsWGCNA logo" width="180"></p>

# wgcna-dashboard

WGCNA 共表达网络分析的 Web dashboard：**Go 后端 + Vue3 前端 + Rust worker**（复用
[`rs-wgcna`](https://github.com/XiaoWang-SXAU-Lab/rs-wgcna) 内核）。

支持 one-step（一键全流程）与 step-by-step（分步，改参只重算下游）两种模式、交互式
可视化（ECharts 树状图/热图/散点 + AG Grid 表格）、单二进制部署，并内置 AI 领航员
（OpenAI 兼容多厂商 LLM + 本地向量 RAG + 错误诊断 + 加密 API key）。

> ✅ 实现已完整（Phase 0–6 + AI 领航员）。后端已端到端验证；前端代码已写完，
> 需在有 Node 的环境 `pnpm install && pnpm build` 后验证构建与渲染。

## 已实现功能

- **7 步管线**：导入质检 → 清洗过滤 → 软阈值 β → 网络与模块 → 模块-性状 → 模块深入 → hub 基因
- **两种模式**：one-step（一键全流程）+ step-by-step（分步，改参只重算下游，断点缓存）
- **作业管理**：`POST /api/jobs` 创建、`/run-all`、`/steps/{step}/run`、`/trait`、`/cancel`、`/results/{step}`
- **可视化**：ECharts 树状图（模块上色）、模块-性状热图、GS/MM 散点、AG Grid 基因表
- **AI 领航员**：多厂商 LLM（OpenAI 兼容）+ 本地向量 RAG + 加密 API key + 流式问答 + 错误诊断
- **单二进制**：`make build` 内嵌前端 + Rust worker 于一个可执行文件

## 架构

```
Web 前端 (Vue3+ECharts+AG Grid)
   │ HTTP / SSE
Go 后端 (API + 作业调度 + 文件管理 + AI 领航员)
   │ spawn + 文件/进度
Rust worker (rs-wgcna-worker --job-dir --step)
```

## 目录

```
backend/    # Go 后端（job/api/ai/web/worker）
frontend/   # Vue3 前端
schemas/    # JSON 契约（参数 / 7 步状态机 / 结果）
```

## 契约（schemas/）

- [`params.schema.json`](schemas/params.schema.json) — 7 步参数定义（A/B/C 三档）
- [`pipeline.schema.json`](schemas/pipeline.schema.json) — 7 步状态机 + 依赖 + one-step 顺序
- [`results.schema.json`](schemas/results.schema.json) — 每步输出结构（对齐 worker）

## 构建

```bash
# 单二进制（内嵌前端 + worker，需要 Node + Rust + Go）
make build            # 产出 ./wgcna-dashboard

# 或分步
cd backend && go build ./cmd/server            # 后端（dev，worker 走 WGCNA_WORKER）
cd frontend && pnpm install && pnpm build      # 前端
cd ../rs_WGCNA && cargo build --release --bin rs-wgcna-worker   # worker
```

## License

[MIT](LICENSE) © 2026 Shawn Wang (Xiao Wang-SXAU-Lab) and contributors
