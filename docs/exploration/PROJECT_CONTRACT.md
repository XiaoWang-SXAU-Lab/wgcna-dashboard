# 项目持久化契约 v1（核心、HTTP 与首版项目界面已接入）

## 当前交付边界

`backend/internal/project` 是独立于旧 Job Manager 的项目核心。新 `/projects` 工作区已经调用它，复用原有图表组件显示六个主要步骤的结果。首页主入口进入项目页。旧 Job 页面仍保留，但不是项目数据的另一视图；项目打开时隐藏旧导航和旧 AI 面板，避免混用上下文。未改 worker 数学算法，未覆盖稳定版发行包。

项目页提供新建/打开/最近项目、输入导入、研究问题卡、分步运行/取消、草稿自动保存、保存失败提示、离开路由前保存和重开恢复。UI 标记旧结果有效性并保留独立执行参数。新工作区暂未接入富集、综合 Overview、报告和 Copilot，不宣称已替换全部旧页面。

支持的 Go 接口：

- `Create(root, name)`：要求父目录存在、项目目录不存在；拒绝覆盖。
- `Open(root)` / `Close()`：打开/关闭项目，操作系统文件锁约束同机写入者。持锁进程退出会释放锁，无需按 PID 猜测过期锁。运行中的项目不允许直接 Close，调用者先取消并等待。
- `Snapshot()`：返回隔离副本。
- `Save(revision, draft, ui, questionCard)`：乐观并发检查；旧 revision 返回 ErrConflict。只允许编辑草稿参数、UI、问题卡，不接收客户端伪造的步骤完成状态。
- `Import(role, reader)`：以唯一目录复制输入并记录 SHA256；角色仅限现有 worker 接口需要的六种输入。
- `Run(ctx, step, worker, stagingRoot, threads)`：在本机独立暂存目录执行原始 worker，验证成功后复制输出并提交清单。仅支持六个主要分析步骤。
- `Validity(engineFingerprint)`：每步返回空字符串（有效）或具体原因。读取旧结果和判断可复用性须分别处理。

所有调用者必须为 stagingRoot 提供本机非同步目录；本轮测试脚本明确使用本机目录。不可将该核心直接暴露为不受限制的远程文件访问接口。

## 保存格式

为先实现跨文件一致性，首版使用单个 `project.json` 保存项目、问题卡、输入索引、草稿、UI 和一个 baseline 分析的步骤记录；没有把状态拆成多份独立写入的 JSON。每次写先保存有效上一版到 `project.json.bak`，再通过同目录临时文件替换主清单。写入失败返回错误，内存 revision 不前进。没有承诺断电/文件系统故障下的完整事务保证。

输入：`inputs/<随机ID>/<role>.tsv`。
输出：`analyses/baseline/executions/<executionID>/step*.{json,tsv,txt}`。
清单使用相对 slash 路径和校验和；拒绝绝对路径、冒号、反斜线、上级目录和符号链接组件。磁盘输出先写完，随后才在主清单中标为完成；失败留下的未引用文件不视为有效结果，暂不自动清理。

不把机器绝对路径、进程、运行时或 API key 写入 project.json。多分析分支浏览、执行历史界面及研究问题卡来源/变更历史仍待后续实现。

## 缓存与中断

执行记录持有实际执行参数，与当前 draft 分离。依次检查：状态、引擎、参数、输入内容、上游有效性与 execution ID、输出内容。主结果 JSON 缺失/损坏、过滤矩阵/基因文件缺失不得提交成功。

首版保守地把所有输入都纳入每一步的指纹（仅更新注释也会导致重新验证/重算），并严格匹配 worker 二进制 SHA256。未来应细化按步骤输入依赖，并定义可跨 OS 复用的算法/实现兼容标识；目前跨 OS 能打开和校验文件，但不同二进制会使缓存失效，不应宣称已实现跨 OS 免重算。

Open 取得锁后把遗留 running 改为 interrupted，并保存；有效上游保留。执行不写共享项目，仅在父进程成功返回后提交，因此父进程退出后的孤儿 worker 只能写本机暂存目录。孤儿的清理和自动取消仍需服务生命周期接入。

OS 锁只协调同机、支持相应锁语义的文件系统，不保证两个同步副本或网络文件系统的分布式互斥；跨机器仍实行交接后单一写入者。

## 已实现 HTTP 契约

- `POST /api/projects`：名称与父目录；后端创建新子目录。
- `POST /api/projects/open`：用户选中的项目目录。
- `GET /api/projects/{id}`：state、validity 与保存状态；绝对路径仅属于本机会话。
- `PUT /api/projects/{id}/draft`：revision、draft、UI、问题卡；冲突用 409，保存失败不报成功。
- `POST /api/projects/{id}/inputs/{role}`：导入数据并返回新 revision。
- `POST /api/projects/{id}/steps/{step}/run`：运行已保存参数，返回 busy 项目快照；执行启动后可读 execution_id。
- `POST /api/projects/{id}/close`：先处理未保存修改/运行，再释放句柄。

run 请求包含 revision 与 threads，响应 202 项目快照并设置 busy；执行启动后步骤记录提供 execution_id。GET 项目轮询状态；`POST /api/projects/{id}/cancel` 取消，等 busy=false 后才能关闭。执行预检错误在 error 字段返回，不伪造完成。结果通过 `GET /api/projects/{id}/results/{step}` 读取并重新校验内容。`GET /api/projects/recent` 返回本机索引。

registry 校验项目身份：同路径重复打开复用会话，同 ID 的另一副本拒绝同时打开。最近项目与绝对路径保存在本机应用数据目录；Windows 默认 LOCALAPPDATA/rsWGCNA-explore，Mac 默认 UserConfigDir/rsWGCNA-explore（Application Support）。staging 在 UserCacheDir/rsWGCNA-explore/staging。拒绝跨 Origin 和 cross-site 文件系统请求，服务仍仅绑定 loopback。

前端 hydration 抑制保存与参数联动；自动保存串行处理修改并保留 revision。409 不覆盖新版本，显示错误并保留草稿；冲突合并界面尚未实现。运行期间禁止编辑；加载 SFT 结果不会覆盖保存的 beta 草稿。

## 已验证

Windows：8 个核心顶层测试及 3 个 HTTP 顶层测试通过（另有 5 个失效子场景、2 个取消/关闭子场景）。真实 Rust worker 运行 100 基因 × 24 样本、保留 80 基因的过滤/SFT/网络，通过服务重建后的重开验证。包含独立进程打开、进程退出释放锁、异常 schema 释放锁、备份失败、revision 冲突、搬迁、草稿与执行参数分离、HTTP 取消和服务 Shutdown。

界面实测：新建中文名称项目 → 导入合成矩阵 → 自动保存问题卡/过滤参数 → filter/SFT → 真正重启应用进程 → 最近项目重开，图表及步骤位置恢复。随后 network(beta=6) → 草稿改 8 → 保存/关闭/刷新/重开，beta 草稿为 8，执行参数为 6，旧图显示待更新。中英文均检查。未对带表型数据的 module_trait/detail/hub 新工作区做完整 UI 验收。

`go vet ./internal/project ./internal/api`、新增 HTTP 测试的 race 检查通过；Windows 隔离构建通过（Rust、vue-tsc、Vite、Go）。darwin/arm64 的 project/API 测试二进制可交叉编译，但不能替代 Mac 实机验收。

全量 `go test ./internal/...` 仍在原有 `TestReportWithoutAllModuleIndexAndCurrentSchemas` 报富集断言失败；已用 HEAD=c9409e3 干净源码在本机独立目录复现同一失败，本轮未顺带修改旧报告逻辑。
