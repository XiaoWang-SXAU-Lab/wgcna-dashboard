<p align="center"><img src="frontend/public/rsWGCNA_icon.png" alt="rsWGCNA logo" width="180"></p>

# rsWGCNA Dashboard

**开箱即用的加权基因共表达网络分析（WGCNA）Web dashboard：Go 后端 + Vue 3 前端 + Rust worker。**

[![English](https://img.shields.io/badge/English-README-blue.svg)](README.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-42b883.svg)](https://vuejs.org)
[![Rust worker](https://img.shields.io/badge/Rust-worker-rs--wgcna-orange.svg)](https://github.com/XiaoWang-SXAU-Lab/rs-wgcna)
[![单二进制](https://img.shields.io/badge/部署-单二进制-lightgrey.svg)](#构建)

> [`rs-wgcna`](https://github.com/XiaoWang-SXAU-Lab/rs-wgcna) 的图形界面：上传数据、调参，直接得到可发表的图和表——无需 R、无需命令行。

---

## 下载 / 安装

预编译的桌面安装包发布在
[Releases](https://github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/releases) 页面，
自带 R 运行时，无需另外安装 R。

- **Windows** — `rsWGCNA-setup-0.5.0.exe`：标准 Inno Setup 安装器，无需管理员
  权限，安装到 `%LOCALAPPDATA%\rsWGCNA`，创建开始菜单 / 桌面快捷方式，自带卸载程序。
  窗口使用 Win10/11 内置的 WebView2。
- **macOS（Apple Silicon）** — `rsWGCNA-0.5.0.dmg`：打开镜像，把 `rsWGCNA.app`
  拖进“应用程序”。应用做了临时签名但未做 Apple 公证，首次打开若被 Gatekeeper 拦截，可右键（或
  Control 点击）应用选择“打开”，或在终端执行
  `xattr -cr /Applications/rsWGCNA.app` 后再双击。

从源码构建见下文[构建](#构建)。

---

## 概览

`rsWGCNA Dashboard` 是一个 **B/S（浏览器/服务器）应用**，把纯 Rust 的
[`rs-wgcna`](https://github.com/XiaoWang-SXAU-Lab/rs-wgcna) 内核包进友好的 Web
界面。它以**单个自包含二进制**发布（前端资源 + Rust worker 全部内嵌），用户只需运行
一个可执行文件。

| 层 | 技术 |
|---|---|
| 前端 | Vue 3 + Vite + ECharts + AG Grid + Bootstrap + Pinia + Vue Router |
| 后端 | Go（REST API + 作业调度 + 文件管理 + AI 领航员） |
| 计算 | Rust worker（`rs-wgcna-worker`）——rs-wgcna 内核 |

---

## 功能

- **7 步管线**：导入质检 → 清洗过滤 → 软阈值 β → 网络与模块 → 模块-性状 →
  模块深入 → hub 基因。
- **两种模式**：
  - *one-step*——一键跑完全流程；
  - *step-by-step*——分步运行；改某步参数只重算该步及其下游（脏级联），结果缓存复用。
- **交互式可视化**：ECharts 树状图（模块上色样本树）、模块-性状热图、GS/MM 散点、
  AG Grid 基因表；每个图都可缩放、换色、下载 PNG/SVG。
- **可发表级绘图**：SFT 与树状图导出 PDF/SVG。
- **AI 领航员**：多厂商 LLM（OpenAI 兼容）+ 本地向量 RAG + 加密 API key +
  流式富文本问答 + 一键错误诊断。
- **数据类型与归一化**：FPKM/TPM（`log10(x+1)`）、raw count（`vst (DESeq2)`，
  纯 Rust）、峰面积（`log10(x)`）、蛋白丰度（`log2(x)`）；支持 TSV/CSV/XLSX。
- **i18n**：English / 中文 切换。

---

## 架构

```
Web 前端 (Vue3 + ECharts + AG Grid)
   │ HTTP / SSE
Go 后端 (API + 作业调度 + 文件 + AI 领航员)
   │ spawn + 进度/文件
Rust worker (rs-wgcna-worker --job-dir --step)
```

后端在 `WGCNA_JOBS_DIR` 下管理作业，通过 SSE 推送进度，并按步调用内嵌的
`rs-wgcna-worker`。AI 领航员基于本地向量索引回答代码库与 WGCNA 原理相关问题。

---

## 目录

```
wgcna-dashboard/
├── backend/             # Go 后端（api / job / ai / plot / web / worker）
│   └── internal/...
├── frontend/            # Vue 3 前端
│   └── src/views, components, stores, utils
├── schemas/             # JSON 契约（参数 / 管线 / 结果）
├── NEWS.md              # 更新日志
└── Makefile             # 构建任务
```

---

## 构建

Windows 与 macOS 使用各自的本机构建目录，避免同步依赖和引擎时相互覆盖。
完整环境及输出位置见 [BUILDING.md](BUILDING.md)。需要 Node、pnpm、Go（版本以 `backend/go.mod` 为准）和 Rust。

```bash
# macOS：在同步盘外构建，内嵌前端 + Rust worker
bash scripts/build-macos.sh
```

Windows（PowerShell）：

```powershell
.\scripts\build-windows.ps1 -Launch
```

---

## 路线图

- [x] 完整 7 步管线 + one-step / step-by-step 模式
- [x] AI 领航员（LLM + RAG + 错误诊断）
- [x] raw count 的 vst 归一化（纯 Rust）
- [ ] 去掉 R 绘图依赖（原生 Rust/SVG 图），实现完全自包含、无 R 的 DMG/EXE 打包
- [ ] 桌面打包（macOS `.dmg` / Windows `.exe`）

---

## 贡献者

- [**Shawn Wang**](https://github.com/ShawnWx2019) — 设计与实现
- [**DeepSeek**](https://github.com/deepseek-ai) — AI 协同开发

## License

[MIT](LICENSE) © 2026 Shawn Wang (Xiao Wang-SXAU-Lab) and contributors
