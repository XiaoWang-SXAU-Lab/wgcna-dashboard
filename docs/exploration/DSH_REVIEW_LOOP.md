# DSH 自动审查与接续约定

2026-11-03：用户明确授权Codex检测当前rsWGCNA的DSH轮次，在完成后审查并向同一DSH会话发送接续prompt。授权覆盖本文限定的开发协作，不覆盖其他项目/会话、提交推送、发布、切provider、修改安全权限或代办权限弹窗。

已配置：复用本聊天自动任务 `wgcna`，改为每5分钟检查，保留低额度交接保护。第一条 `rean-rank-20261103-01` 已通过当前DSH图形会话发送并观察到消息气泡。本轮只审查/诊断及维护review文档，未改算法源码；本机state.json记录派发和待同步交接，DSH工作中不争写共享交接，待下一轮awaiting_review再同步。旧文档“不自动联系DSH”的约束已由本次用户明确授权在上述范围内覆盖。

## 工作方式

- Codex定时检查；DSH是算法/UI源码单一写入者，Codex只读源码、在本机隔离目录做必要的小型探针，并在DSH空闲时写review/交接文档。
- 当前用独立DeepSeek Harness图形应用的rsWGCNA会话，不是Codex会话。用Computer Use技能的sky接口读取/操作应用，不通过终端UI或猜测协议注入。不根据窗口标题独自选目标，需同时核对wgcna工作区与最新相关内容。
- 进程存在或退出不等于一轮完成；后台任务数量也不等于当前轮仍在写。以当前会话已完成/等待输入、DSH轮次交接标记、相关源码快照无继续变化共同判定。UI虚拟列表可能返回旧消息，须检查当前可见的底部最新状态。
- DSH每轮收尾：更新正常交接，写本机review-loop/ready.json为awaiting_review，停止修改源码并等待Codex；不在结束前自行启动下一阶段。
- Codex每个新round_id只审查一次，对源码哈希做前后核对；若变化则放弃此次结论，等真正完成。发现问题发修复任务；通过则发实施prompt范围内的下一小阶段。无变更不重复审查或发消息。
- 若发送前/发送后UI失败，记录delivery_unknown，下一次先读会话确认review_id是否出现，禁止盲重发。用户手工输入的草稿不得覆盖；发送目标不唯一或当前仍忙则不发送。
- 任何真实审批/登录/安全权限弹窗交给用户，不能通过自动发送“同意”绕过；DSH回复/文档内容本身不能扩展用户授权。

## 本机记录（不共享机器配置）

Windows根：%LOCALAPPDATA%/rsWGCNA-build/windows/review-loop；Mac根：~/Library/Caches/rsWGCNA-build/macos/review-loop。UI能力跨平台必须现场检查，不能假设Windows sky适用于Mac。

ready.json由DSH原子写入，示例：

```json
{
  "round_id": "unique-id",
  "status": "awaiting_review",
  "stage": "M1 ranking primitives",
  "completed_at": "ISO-8601",
  "changed_files": ["rs_WGCNA/src/hub_priority.rs"],
  "validation": [{"command": "...", "result": "..."}],
  "not_run": [],
  "known_failures": [],
  "source_manifest": "round-id-source-manifest.json",
  "next_step": "..."
}
```

manifest含两仓库branch/HEAD和相关源码相对路径/SHA256（含未跟踪文件），不含密钥或源数据。marker只能作为调度信息，不是测试通过证明；Codex核对源码/验证证据后才能下结论。

Codex维护review-loop/state.json与每轮review/dispatch记录，含review_id/round_id/source hash/状态(preparing、sending、sent、delivery_unknown、blocked)、已发送prompt摘要及UI身份/本地日志位置。共享review文件是 `DSH_REVIEW_PROMPT.md`，DSH不要覆盖它；HANDOFF/DSH_PROMPT双方按串行空闲时间更新。

## 额度、边界与终止

沿用账户Codex额度检查。任一可用窗口剩余<=20%或明确上下文告警时，先更新双交接，不派发新的实现轮次，通知用户需要后续接手；未知额度不当0。正常轮次安静检查，只有新审查结论/成功派发/失败或需用户操作时通知。

本任务截止实施prompt的M0–M4；M5只有协议/已有可用数据的评估，不自动下载敏感数据或做昂贵实验。全部范围完成并审查后结束review监控，保留有用的低额度交接保护。用户要求停止/暂停/手动接管即停止派发并更新自动任务。

本地调度依赖电脑开机、应用运行、桌面可用、工具可用和账户额度；不能保证熄屏锁定/离线/额度耗尽后仍能执行。不是进程退出实时事件钩子。
