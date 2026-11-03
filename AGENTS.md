# 探索分支协作规则

本规则适用于本仓库内的人类开发者、Codex 和 DSH。研究方案见 `docs/exploration/PLAN.md`，接续入口见 `docs/exploration/HANDOFF.md`。

- 探索工作在 `explore/project-copilot`，不得将探索改动混入 `main`。不要自动提交、推送或合并。
- 开始工作先读工作区根目录 `STATE.md`（一屏当前状态）和 `wgcna-dev` 技能（如可用），再检查分支及工作区；保留未知来源的未提交改动。用户当前指令优先。
- 同步目录只保存源码、锁文件、文档及小型测试 fixture。禁止在其中安装依赖、编译、运行生成产物，或放置完整 R 环境。
- Windows 使用 `%LOCALAPPDATA%/rsWGCNA-build/windows`；macOS 使用 `~/Library/Caches/rsWGCNA-build/macos`。使用 `scripts/build-windows.ps1` / `scripts/build-macos.sh` 的本机源码快照构建。
- Node/pnpm、Cargo、Go、R 的依赖、缓存、运行时和产物必须位于本机非同步目录。先检查实际路径，不能只依据默认值推断安全；`.gitignore` 不能隔离同步。
- 探索版不得覆盖同步目录相邻的 `releases/rsWGCNA-current`，也不得覆盖稳定版安装目录。旧目录不自动删除。
- API key、机器绝对路径、端口/PID、机器配置留在本地。共享项目内部引用使用相对路径；不要将机器配置写入项目。
- 家里与办公室同一时间仅一个写入者。优先通过 Git 远端交接源码历史，禁止实时同步 `.git`；不要在两台机器同时修改同一共享项目。
- 每次开发结束同时更新 `docs/exploration/HANDOFF.md` 和 `docs/exploration/DSH_PROMPT.md`：实际改动、验证、未解决事项、精确下一步及 dashboard/worker 提交。未运行的检查必须明确标记。Prompt 必须对应最新实际状态，不能只保留泛化模板。
- 未实现、未验证的能力不得写成已完成。恢复分析不能凭文件存在判定缓存有效。

## 自动与手动交接触发（用户要求，2026-10-31）

- 每个开发回合开始、完成主要阶段、开始可能耗时的工作前，如有 `get_usage_limits` 工具则检查 Codex 账户额度。优先 rateLimitsByLimitId 中适用 Codex 桶；不存在时用 legacy rateLimits，不把不相关模型桶混入。剩余比例为 100-usedPercent，未知值不是 0。额度百分比不等同精确剩余 token。
- 任一适用额度窗口剩余 <=20%，或收到明确额度不足/上下文不足信号，立即先同步 HANDOFF 与 DSH_PROMPT，再决定下一小步工作。不要等额度耗尽才保存；没有额度工具时也保持阶段性快照，不声称存在可靠余量检测。
- 用户说“交接”“换机器”“去办公室”“回家继续”“交给 DSH”等明确接续意图时，不等定时检查，立即核对现场状态并同时更新两个文件；有歧义时不要擅自结束当前开发。
- 交接至少包含：两仓库 branch/HEAD/未提交清单，完成项与未完成项，实际检查和未运行检查，基线失败，本轮进行中的操作，本地进程/项目写入者的记录位置，精确下一步和验收场景。共享文档不保存密钥、账户 ID、额度原始响应或机器敏感配置。
- DSH prompt 给出当前任务与限制，指向最新 HANDOFF；保持 Mac/Windows 依赖、缓存、R、产物在各自机器同步盘外。
- 交接不等于自动 commit/push、切 provider、运行 DSH 或向其他人发送消息。用户未授权这些动作时，只更新可审查文件。
- 当前聊天配置了每小时低额度检查（heartbeat）。这是调度兜底，依赖本机可运行和剩余额度，不能保证额度耗尽、应用关闭或主机离线后仍能执行；不是账户额度变化的实时事件钩子。无实质进展时不重写交接、不重复提醒。
