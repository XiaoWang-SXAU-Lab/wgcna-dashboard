# M1基础评审（2026-11-03，Codex）

结论：结构方向正确，但不能按“已完成可靠基础”验收。先补输入契约、数值后置检查与正确性测试，再接hub_priority。未改worker或前端源码。

## 发现

1. **P1：分层缺失静默回退。** `resampling.rs:48` 用 `stratum.get(i).copied().unwrap_or(0)`，空数组也返回正常索引，实际变成单层iid；短数组缺失样本加入0组，长数组尾部被忽略。函数应返回Result，Stratified要求长度严格等于n_samples且n_samples满足计算前置条件，非法输入明确报错。下游仍需校验缺失标签、用户设计确认。`below(0)`返回0不是合法随机索引，应拒绝零上界。固定随机序列需增加参考向量与跨多轮调用测试；模运算的微小偏差非本轮主要阻塞，如改无偏采样需记录版本。

2. **P1：输入值有限且不恒定，不保证数值计算有限。** `kwithin.rs:52–64`未检查cor/adj/out，可能Ok(NaN)，signed-hybrid还会将NaN相关通过c>0分支掩盖为0。实际当前源码隔离编译复现：两行均[1e-200,2e-200,3e-200,4e-200]，unsigned/power6得到Ok([NaN,NaN])；signed-hybrid得到Ok([0,0])。平方下溢导致sd=0，这不是原始序列恒定。必须在进入adjacency前检查cor有限性，在转换及累加后检查输出；对仅浮点舍入超出[-1,1]的相关规定容差及clamp策略，明显异常报错，不通过替换NaN恢复。极小数不是声称真实转录组常见问题，而是检查“错误必须返回Err”契约的反例。finite power>0也需校验（NaN power在全相同表达的两行上实际返回Ok([1,1])）；样本数、空模块和threads的边界策略应显式定义。

3. **P2：测试尚未验证公式。** 现有kWithin只验证两基因结果相等/非负和常数拒绝，全返回0也能通过。需要三基因显式期望值与三种network_type：A=[-1,0,1,0]，B=[0,1,0,-1]，C=-A，power2，unsigned期望[1,0,1]、signed期望[0.25,0.5,0.25]、signed-hybrid期望[0,0,0]；当前实现这些正常案例实际通过。补小数power、行置换、非有限/数值下溢、非法分层、单例层、固定seed多轮序列、线程差异与run_hub原始字段数值一致性。

## 范围与性能

run_hub确实调用共享函数，非TOM非对角adjacency行和正确；分层用首次出现顺序分组，没有HashMap随机迭代改变组序的问题。当前仍分配完整g×g相关和adjacency及中间buffer，没有实现计划中的分块；相关复杂度O(g²n)，adjacency/求和O(g²)，内存O(g²)。不阻塞先做小型纯算法原型，但在完整大模块×1000重采样交付前需资源估计/上限，不得把全模块默默截400基因。

本次只检查基础两模块和相关调用，不代表M0全部关闭：当前run_rean的无效重复continue绕过进度更新；原始输入预检/无效原因稳定汇总及端到端分母测试仍需单独核对。worker已有两项validate_round_vectors测试不等于完整bootstrap反例回归。

## 实际验证

用当前kw/in/resampling及当前adjacency/correlation/network/error源码复制到同步盘外，通过rustc链接已有第三方依赖编译隔离probe；最终有效probe没有复用旧rs_wgcna实现。复现空分层、tiny数值、NaN power及三种网络类型显式期望值。前期探针链接配置失败后修正，不计为测试通过。

材料在本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-m1-foundation-*`，指针review-m1-foundation-latest.txt。没有在同步盘构建，没有安装依赖，没有重跑完整cargo/Go/前端套件或完整应用构建。DSH报告的34 tests/完整构建为对方记录，非本轮重跑。已有report_test.go:44基线失败未复核。

下一步：DSH先修上述P1并加P2针对性测试，然后写hub_priority纯函数及排名/并列测试；不需重新讨论架构、不扩展UI。保持单写入者与依赖产物外置，不提交推送。
