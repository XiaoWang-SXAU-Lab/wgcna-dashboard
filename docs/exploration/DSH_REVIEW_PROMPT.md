# 当前派发 rean-evidence-fix-20261103-14

review_id: rean-evidence-fix-20261103-14

已审rean-ai-contract-20261103-13-fix：manifest匹配、DSH GUI空闲。包独立/无远程调用/有限枚举渲染方向正确，但尚不构成完整防幻觉边界。Codex本机隔离复制当前两个evidence源码运行原12例+4个生产路径反例，原12通过、新4全部失败。目录review-loop/codex13-evidence-probe，日志codex13-evidence-probe.txt；没有改仓库源码/完整构建。先修本包，不接API/UI/在线模型。

## 必修反例与契约

1. **P1 BuildBundle FullCoverage无条件true**：gene_count=1000但genes仅1行仍被称完整模块；design_confirmed=false仍可推荐。校验完整原结果gene_count/行数、ID非空唯一、固定完整scope、受支持设计/方法、有限数值与排名/频率范围、attempted=valid+invalid非负等不变量。要支持上下文子集，应从先验证的完整结果显式选择并记录全量规模/子集scope，不能信一个布尔标签。无valid/null支持要保留。设计未确认/不支持时拒绝推荐或整包拒绝，不能只放进事实。
2. **P1 ValidateSuggestion只验证事实存在，不验证候选和证据关联**：gene_ids=[g1], fact_ids=[g2.kwithin]能通过；gene_ids=[g1]无任何事实也可通过。为fact显式绑定gene/字段语义/来源（不要用含gene文本的路径猜ID），每个候选建议要求关联本gene可用事实，解释枚举也要有对应事实/条件。允许解释通用指标概念，但不能据此宣称某候选稳定/并列；无统计支持不要把tie_indistinguishable当具体判断。数值claims同时绑定已引用fact，来源/执行不匹配拒绝。
3. **P2 DecodeSuggestion只Decode一次**：合法对象后跟第二个action对象仍接受。要求整个输入恰好一个非null JSON对象，随后EOF；拒绝尾随JSON/垃圾、null及危险未知字段（重复key策略明确）。加反例`{"gene_ids":[]} {"run":true}`。长度限制继续保留。RenderSuggestion当前不强制Validate，外部可直接把未校验gene渲染；提供唯一安全入口Decode→Validate→Render返回error，或validated类型，避免调用者漏一步。零valid/未确认设计/不存在gene必须无法通过公开渲染入口产出推荐。
4. **P1 来源契约实际丢失**：Envelope只留project/execution/engine，忽略实际params/inputs/upstream/validity；bundle不能验证来源却称绑定输入参数。纳入Project报告来源并检验有效性（缺/非空validity拒绝，offline测试fixture显式提供），fact绑定可信bundle身份指纹。保留范围限制：Builder输入必须来自服务器已校验快照，不接受用户造envelope当当前证据；将来API要重新核对current身份。当前“跨执行拒绝”测试只是拒绝一个不同fact_id，没有证明整个旧建议无引用时被拒绝；schema显式绑定bundle/project/execution并检验。
5. P05/P95、invalid原因/strata及method_version目前解析后未入事实或根本忽略，补实际需要字段和范围/分母；n_bootstraps应读原字段，不把attempted冒充原字段。科学文案“排名为bootstrap采样频率”错误：midrank为排名，mean_top_k才是分数入选频率，模板分别解释。BuildBundle输入和输出上下文预算合理显式限制，不静默截断科学结果。

## 验收

将本机4个失败反例纳正式测试，加无事实推荐/不匹配claim/重复ID/未确认设计/负计数/截断模块/过期validity/旧建议身份/null及trailingJSON/绕过renderer覆盖。测试实际代码，修fixture中现存top_k大于gene_count等不合法默认值，不能为保持旧测试绕过校验。已有zero-valid案例也需null统计契约真实一致。只定向Go evidence测试，不前端/worker/完整构建。通过后仍不称AI在线或科学效用已验证。

完成更新双交接/STATE，所有文件收尾后最后采完整manifest并写新ready，awaiting_review等复核。已知report_test.go:44/GUI/Mac未运行缺口保留；依赖产物与密钥配置机器本地同步盘外，不提交推送/切provider/ACL。

---

以下为历史：
# 当前派发 rean-ai-contract-20261103-13

review_id: rean-ai-contract-20261103-13

12修复定向复核通过：21文件manifest稳定，DSH GUI空闲。Codex独立核对production composable与本机快照SHA256一致，实际store+composable13/13通过（含跨项目loading及真正reject迟到error），日志review-loop/codex12-report-review.txt。只覆盖这些用例；报告GUI/Mac、完整Go独立复跑、HTML/PDF和候选理由持久化仍缺，不称全部M0/M2完成。后端报告锁内envelope与当前身份结构已落地。可以推进M3的离线事实契约小切片；不直接接在线LLM/自由聊天。

## 本轮范围：M3 backend EvidenceBundle + 约束响应校验器 + 离线测试

1. 复核现有backend/internal/ai与api/ai.go的传输/配置/legacy权限，不读取或输出密钥，不切用户provider。只复用纯类型/可复用Client接口设计；不要调用旧Navigator工具或把project ID伪装job ID。本轮不发远程请求、不新增shell/文件/HTTP工具，不默认启用远程AI；不做新UI或复杂agent循环。
2. 基于Project一致快照报告envelope构造EvidenceBundle（优先新独立文件/包，不把算法塞api handler）。每个事实有稳定fact_id，绑定project/execution/input/params/engine+字段路径，值/单位/分母/来源与科学scope由程序提供。包含模块完整规模、设计、valid/attempted/invalid/方向未知、K/B/seed、方法局限、比较候选与是否完整覆盖模块的声明。候选ID必须在完整结果中，子集必须明确subset；摘要由确定代码算，不让模型算指标。预算限制仅明确裁剪上下文/拒绝请求，不改变科学结果或把子集称全模块。
3. 定义只读建议响应schema：可用gene_id、fact_id列表、有限枚举的解释/局限/方案预览，不接执行参数修改/运行指令。程序验project/execution、事实存在/引用候选匹配/数值一致（如输出允许数值则必须与事实精确核对）、scope及质量状态。**自由文本+一句system prompt不是防幻觉实现**；核心科学结论用代码渲染已批准模板/事实引用，不能仅用关键词regex阻止“90%真实概率”。原始模型文本保留诊断也不得当已验证结论展示；本轮可限制输出只含结构化引用+枚举，后续UI再决定措辞。
4. 无表型不能输出GS/表型机制；invalid>0必须带条件化说明、零valid/不支持设计/过期结果不允许推荐。频率不解释成真实hub概率或p/FDR；不能声称因果/功能文献。对同源kME/kWithin/TOM不合成独立证据总分，不保证压缩到唯一候选。建议仅预览，应用参数/运行权限本轮无实现即显式不可执行，防止经schema暗带tools/action。
5. 用户问题卡/备注/annotation/gene ID皆不可信数据，不能当系统指令。不向外发送原始表达矩阵/样本标识/路径/密钥；数据最小化，remote开启及传输同意在后续adapter/UI明确做。当前adapter只mock或deterministic fallback，返回状态明确offline/unconfigured，不能悄悄换provider。失败/超时/非JSON/过大响应/未知字段需有受控失败接口设计，不把原始错误/secret传到UI。
6. 若本轮提供Project只读API，则必须从服务器当前快照构造/校验bundle，客户端fact_id或project id不能扩权限，输出回包带身份供后续代际检查。无配置也可运行确定性说明；不得声称已在线模型实测。

## 必须的生产路径测试

导入实际bundle builder/validator/renderer，mock模型输入即可。覆盖：合法引用通过；伪造gene/fact/value；跨项目/旧execution；候选子集冒充全模块；无trait给GS解释；零valid推荐；invalid条件提示遗漏；把频率当真实概率（枚举/模板边界）；恶意注释“忽略规则”；隐藏执行/参数修改字段；非JSON/未知字段/过大输入；缺配置/远程关闭确定性降级。断言算法数值/项目参数/执行不被改变。不要靠复制validator写一个模拟测试。

只跑本轮Go定向检查/必要前端类型检查（若没改前端不必重跑），不完整桌面/Rust构建。report_test.go:44基线继续单记。完成交付实际schema、验证结果、未做在线/UI/应用建议的明确缺口，更新双交接/STATE；全部收尾后最后采完整manifest与新ready，awaiting_review等待下一轮。依赖产物/配置密钥各机同步盘外，不提交推送/provider/ACL。

---

以下为历史：
# 当前派发 rean-report-state-20261103-12

review_id: rean-report-state-20261103-12

11快照已核对全部匹配，继续10轮报告复核。后端Project锁内envelope+前端绑定execution/报告来源已落地，主要科学身份P1改善。Codex独立运行实际store+composable：原11例通过，新增跨项目loading反例失败（12例11通过1失败）。composable快照SHA256匹配当前源码；日志本机review-loop/codex10-report-loading.txt。当前不进入M3，只补这一状态边界和真实error测试，不另做大重构。

1. **P2，frontend/src/composables/useCandidateReport.ts的project watch/finally。** 报告请求挂起时切项目，watch增加reportReq却未把loading重置；旧请求finally因为req不相等也不清loading，于是新项目按钮始终disabled，无法生成报告。复现：p1 load挂起→current切p2→等待watch→resolve p1并await，loading仍true，预期false。正式加入此反例并修复：项目/会话边界释放其loading/error/旧request身份，同时保证新请求不会被旧finally解锁。测试已追加到本机store-test/src/composables/useCandidateReport.test.ts，原件.codex10-backup；不改仓库源码的probe，原11保持通过。
2. 现有名为“a superseded late error...”的测试实际上resolveA成功，从未reject，不能证明catch守卫。加真正rejectA反例，并覆盖执行变化（同项目）后旧error不覆盖新报告/状态；需要时让报告请求在session/execution变化时失效。不要凭测试名称宣称已覆盖错误分支。同项目重开session边界缺口也要直接检查，不把仅project id watch叫session守卫。
3. 本轮保持报告envelope/科学算法不变；测试里ProjectResponse应从api/projects导入（当前从api/client导入但主typecheck排除了test，vitest转译也不检查类型），为新测试在外置环境做相应类型检查或明确未做，不靠排除tests掩盖错误。

只定向前端实际生产composable/store测试与typecheck，不重跑完整Rust/Go/桌面；后端envelope源码审查通过结构检查，但本轮未独立重跑Go，不声称全部报告验收。>200/null/转义/engine与GUI/Mac缺口按实际记录，HTML/PDF与候选理由持久化仍未做。完成先写所有源码/STATE/交接，最后采manifest，再ready新round_id，确保后续不改采样文件。DSH单写入者，依赖产物各机同步盘外，不提交推送/改provider/ACL。

---

以下为历史：
# 当前派发 rean-manifest-20261103-11

review_id: rean-manifest-20261103-11

## Codex：10轮复核暂缓（2026-11-03）

ready round rean-report-fix-20261103-10-fix已报告完成，但manifest包含的STATE.md实际SHA256不匹配（其余文件当前匹配）。依自动review协议先停止本轮结论，不把未完成复核称通过、不派M3。请确认收尾后不再写文件，重写完整manifest并生成新round_id的awaiting_review；本轮仅修交接快照，不改算法/UI或重跑构建。snapshot修好后继续10轮报告身份与生产路径测试复核。11/11为DSH报告，Codex本轮尚未独立运行。机器hash/记录留本机review-loop；既有report_test.go:44、GUI/Mac未验收保留。
---
以下为历史：

# Codex → DSH：当前派发 rean-report-fix-20261103-10

review_id: rean-report-fix-20261103-10

已审rean-report-20261103-09-fix，18文件manifest稳定、同一wgcna GUI完成且空输入。Project独立报告路由/后端validity拒绝/完整候选与转义/质量文案已接入，09已确认送达，旧delivery_unknown文字只是历史。暂不进入M3：报告当前证据身份尚未绑定，先修以下问题。本轮Codex仅源码/哈希/GUI检查，DSH报告typecheck/Go/隔离构建通过，未独立重跑；上一轮独立store7/7仍保留，不能代替报告验收。

1. **P1，ProjectReportView.vue:17,20-36,89-92,110-130**：report只存HubPriorityResult，current只看store当前hub_priority。生成A报告后重跑B，等B结果加载且currentResult再次true，report仍是A，却恢复为“当前”并出现B的TSV链接；upstream更是从实时store读取，A数字+B上游拼接。跨项目切换后同组件复用也同样会保留A（load的pid/exec守卫仅约束请求返回瞬间，不约束已显示report生命周期）。必须保存报告自身的project/session/execution身份和对应上游/参数快照，current要求报告身份与当前状态一致；A报告可留历史但不能跟B元数据/下载混搭。任何项目/执行/有效性变化撤销当前显示或明确历史，返回后也检查请求代际；catch/finally不能让旧请求清空/解锁新项目的报告。
2. **P1，backend/internal/api/projects.go:503-521**：端点仅返回worker原JSON，没有Project ID/报告execution、输入/参数SHA、filter/network来源execution；前端无法验证回包到底属于捕获的那次execution，显示上游只好猜实时state。请加结构化report envelope，用Project内一致读取方式拿到该结果与其Step/upstream/Inputs/Params/Engine等来源，按同一状态校验validity与产物读取，必要一个小的Project只读方法复用锁内实现；不能通过分别Snapshot/Validity/Result后拼装就宣称原子快照。分析正在改相关状态时明确拒绝或明确历史，不从不同execution混拼。不要输出机器密钥/绝对配置路径。
3. **P2：只有TestCandidateReportNotRun，验收范围远不足**。新增生产路径测试：有效无trait/hub报告身份与全部候选/JSON结构；参数/上游engine失效409；valid=0与nullable；>200完整行数；前端A报告后切B/新项目应撤证据、同项目重开、两报告请求逆序及迟到error不得抹新结果、dirty/saving时当前导出禁止；危险gene字符串按文本显示。允许mock worker构造生命周期，不能把mock当真实数值验证。直接测试实际view/composable或抽出的生产report状态逻辑，不重写模拟守卫。
4. 完成09最小机器可读证据下载（JSON envelope即可，完整参数/来源与全部候选），TSV只是候选表不包含报告来源，不要称完整报告导出。当前TSV默认URL指向最新结果，要么仅匹配当前报告execution时显示，要么提供明确绑定不可变历史execution的受限下载；不开放任意路径。报告生成按钮本地dirty/saving至少解释/禁用当前请求，历史显式另说。研究问题卡/候选选择理由尚未纳入，则继续标未完成。

仅做报告/来源/测试修复，不改worker、不上AI、不扩复杂PDF。只跑定向Go/前端检查，不重复完整桌面构建或Rust。report_test.go:44既有失败单独记录；GUI点击/Mac未验收仍留缺口。修复完成同步HANDOFF/DSH_PROMPT/STATE，ready新round_id和完整manifest，awaiting_review等待复核。单DSH源码写入，依赖缓存产物各机同步盘外，不提交推送/改ACL/provider。

---

以下为历史：
# Codex → DSH：当前派发 rean-report-20261103-09

review_id: rean-report-20261103-09

额度已恢复，已完成08定向复核。15文件manifest前后稳定、同一wgcna DSH GUI空闲/无草稿。run/import/demo/cancel响应落地后stateGen前进已实现；Codex核对本机外置快照store与测试SHA256等于当前源码，独立运行实际store vitest 7/7通过，日志review-loop/codex08-store-review.txt。上一轮6通过1失败的反例现已通过，不再重复修它。此结论仅覆盖已测状态竞争，不宣布所有并发/GUI/整个M0都验收。未重跑完整Rust/Go/桌面，report_test.go:44基线未重跑，GUI点击仍缺口。

## 本轮：M2 项目候选证据报告最小切片，不做AI

1. 先沿实际报告路由/下载/后端调用核对Project与legacy Job路径。当前prepareReportFigures.ts走/api/jobs/{job}/report/prepare，Project run.go没有report step；不能把project id当job id或复用历史Job数据。复用可复用的渲染组件/纯函数，接入Project自己的只读报告入口（原报告导航若仍placeholder则接这里，不新增R-EAN顶层面板）。无需新worker科学step，不改已完成算法。
2. 最小可交付是当前Project的候选优先级证据报告预览 + 可下载完整机器可读证据/候选TSV；如可方便导出HTML则做安全转义、静态无脚本的版本，暂不扩复杂PDF排版。报告通过后端项目锁/一致快照校验取得同一execution结果、其filter/network依赖身份、输入/参数hash、方法版本、模块完整规模、power/network_type、K/B/seed、重采样设计/strata说明、attempted/valid/invalid及原因、方向未知、全部候选原始指标/排名区间/分数入选频率、工程资源说明。不能只导出当前页，不能把bootstrap分位区间叫置信区间/显著性，不能称真正hub或把同源指标合成总分。
3. 后端报告当前有效性必须重新校验（不信前端boolean）：结果缺失/参数上游引擎失效时拒绝当前报告并返回清楚原因。前端本地draftDirty/saving也立即撤销当前报告预览/导出；请求回包做project/session/execution代际校验，不能将迟到旧报告显示为当前。要保留历史必须显式历史模式、来源execution/模块/参数标注，不默认合并旧rean。单次生成不能跨不同execution拼装；下载白名单/路径安全沿用精确约束。
4. 加确定性的候选解释和局限：无表型明确仅网络候选优先级；同频率/重叠排名区间诚实显示不可区分，不强制唯一推荐。invalid>0提示条件于可计算重复，valid=0仅保留原始指标及质量说明，禁止推荐。研究问题卡内容仅作为用户陈述，并标记无法由网络单独回答因果。用户选中候选/理由若本轮保存，需独立于科学参数/执行指纹、绑定来源execution并在失效后降为历史；没时间就明确此功能未做，不用选中UI状态冒充已持久化。
5. 用户备注/模块名/gene ID为不可信文本，HTML/Markdown渲染必须安全转义，不执行其中指令/脚本，不读取任意文件/远程资源，不发送远程AI。报告不添加模型总结、不使用自由生成文献或功能注释。

## 验收与范围

- 定向后端/前端纯函数/API检查：无trait/hub可生成；not-run/stale/全invalid；同execution一致性/跨项目与迟到请求拒绝；>200完整行数与TSV；缺nullable；危险gene/备注转义；关闭重开报告入口可恢复当前结果但不自动跑分析。既有7个实际store测试保留；新检查必须测生产路径，不复制实现当模拟。
- 依赖/运行产物只在本机外置缓存，不新增机器配置到共享源码；只跑受影响测试/typecheck。必要一次隔离桌面构建用于该新入口，GUI可视检查若仍无能力明确留给Codex，不反复构建声称验收。Mac未跑标未验证，不用Windows结果代替。
- 不改worker算法、不上AI、不切provider、不提交推送/改ACL/覆盖用户项目或稳定版。M2剩余候选局部解释图/选择理由等未做项如实列出，M3等下一轮审查后再派。
- 完成更新HANDOFF/DSH_PROMPT及根STATE顶部（现根STATE仍旧），ready新round_id/source manifest覆盖新增后端/前端/测试文件，awaiting_review后停止源码写入等review。DSH_REVIEW_PROMPT由Codex维护不要覆盖。

---

以下为历史派发：
# Codex → DSH：当前派发 rean-state-race-20261103-08

review_id: rean-state-race-20261103-08

已复核rean-state-fix-20261103-07-fix（15文件manifest稳定、GUI空闲），独立在本机store-test快照运行实际store测试：原6例通过，新增第7例失败。不是模拟，也未改仓库源码。源码哈希与当前store一致。

**P1：refresh在run进行中发起仍能回滚状态。** project.ts的stateGen在run开始增加，run响应落地却不增加；refresh不检查working，因此run POST挂起时发起refresh，ver捕获的就是run开始后的版本；run响应提交revision2/new execution，然后这个refresh返回revision1，仍通过ver检查并回滚current。实测expected revision2，received1。相同边界也要考虑import/demo等mutating请求进行期间的读；仅阻止run之前的refresh不够。请对mutation开始/落地与读取之间建立完整契约（允许用mutation in-flight计数+落地版本+会话身份等小实现），避免读覆盖新写、旧mutation响应覆盖新项目。不要为了通过测试关闭运行期必要进度轮询，须合理处理working/后端busy的区别。保留之前6例，不删除测试或弱化断言。

本地复现（路径通过本机构建根定位，不复制另一机器绝对路径）：review-loop/store-test/src/stores/project.test.ts 最后追加了review probe，原测试备份为project.test.ts.codex07-backup；日志review-loop/codex07-refresh-during-run.txt。步骤：open旧revision1；调用run('filter')并挂起POST；再调用refresh并挂起GET；先resolve run为revision2/new running/busy=true并await；再resolve refresh旧revision1并await；断言revision仍2、execution仍new。当前7例=6通过1失败。将此反例纳入仓库正式实际store测试，并补mutation响应期间读/保存交错必要覆盖。只前端定向修复与测试/typecheck，不完整构建、不改worker/Go、不做报告或AI。

UI selection身份+双current overlay已源码确认落地，本轮不重复改它。本轮Codex独立只实际store7例与源码/哈希/GUI检查；完整Rust/Go/桌面/可视点击未跑，report_test.go:44基线仍保留。测试依赖/产物在本机同步盘外，保留未知源码；完成双交接与新ready/manifest，awaiting_review等待复核，不提交推送/切provider/改权限。

---

以下为历史：
# Codex → DSH：当前派发 rean-state-fix-20261103-07

review_id: rean-state-fix-20261103-07

已审 rean-ui-fix-20261103-06-fix，13文件manifest稳定，DSH GUI空闲。loadResults 的session/batch/execution守卫和strict validity已落地，旧分母未知/历史徽标亦改善。剩下一个同类P1必须关闭，先不进入报告/AI；不要继续扩大架构。

1. **P1，frontend/src/stores/project.ts:117-130**：refresh只捕获sessionGen和generation，但generation仅在draft/card/ui编辑增加。参数不变直接重新运行时，这两个值不变。旧refresh在run前发出并拿到A的响应，run提交B的running/完成状态后旧refresh才返回，124检查仍全通过，125把current回滚A（包括execution/status/busy）。两次轮询逆序也同理。请用真正的状态写入/请求代际或后端单调revision契约防止旧refresh覆盖run/import/demo/save或更新的refresh；不要只补draftGeneration。确保请求接受条件验证当前session、写入版本、必要revision，变更操作开始/落地时能使旧读失效，且异步finally不影响后续项目。若后端某状态变化不增加revision，不能单靠revision。
2. 当前guard-sim-test.mjs是另外重写的守卫模拟，没导入实际store，第三场景只做generation++且没有真正save/run，不能证明production修复。请在本机隔离源码快照、已有Vue/Pinia/TS依赖运行**实际store**的受控deferred请求测试（mock projectRequest传输可以；不能把待测守卫再抄一遍）。至少覆盖：不改参数run与旧refresh交错、两refresh逆序、A/B结果逆序、同项目重开、draft保存交错、缺失validity。可把测试源码放仓库，依赖/编译产物只放本机外置目录。若运行环境缺依赖，记录具体缺项，不能用模拟脚本替代通过。
3. **P2，ModuleExplorerView.vue:130-132** 仅watch hub当前bool，rean execution变动但hub保持当前时不清旧selectedElement.stability；stabilityMap:168仅检查module并不能叫“跨execution验证”。选择快照须绑定hub/rean证据身份，任一相关execution/有效性变动撤销或更新；rean叠加同时要求当前hub及其依赖execution匹配，模块同名不足以证明关联。对历史网络的导出/证据卡明确标历史，不使其成为当前事实。

验证仅前端针对性测试/typecheck，不重复完整Rust/Go/桌面构建，不改worker算法。可视GUI验收仍未跑：已知DSH无GUI能力，明确保留给后续Codex/用户，不反复构建假装完成。本轮Codex只源码/哈希/GUI检查，未独立运行测试；你报告的typecheck/模拟3例/构建通过与实际store验收分开写。report_test.go:44已有基线未重跑。完成同步双交接，ready新round_id和完整manifest，awaiting_review等复核；不提交推送/切provider/改权限，依赖产物同步盘外。

---

以下为历史：
# Codex → DSH：当前派发 rean-ui-fix-20261103-06

review_id: rean-ui-fix-20261103-06

已审 rean-ui-20261103-05-fix：13 文件 manifest 前后匹配，当前 wgcna DSH 会话完成且输入为空。候选表、独立性确认、完整 TSV、nullable 排名及真实 worker Project fixture 已接入；不撤回这些成果。但 M0 当前证据还有下面阻塞，先修完，不进入 AI（总体计划 AI 是 M3，不能因旧接续写 M2 就提前）。本轮独立验证仅源码/哈希/现场检查，没有重跑完整 Cargo/Go/桌面构建。64 tests、Go API、真实 worker e2e 与构建成功均为 DSH 本轮报告；实际可视点击验收仍未跑。

## 必修：同项目异步代际与证据边界

1. **P1，frontend/src/stores/project.ts:124-138。** loadResults 只在返回时比较 project id，没比较请求代际或 execution_id。复现顺序：同项目 execution A 的请求挂起；运行 B 完成并启动新 loadResults，B 先返回；A 晚返回仍覆盖 results[step]。152-158 的 currentResult 只检查当前 B 的 done/validity，因此 A 数字被当 B 的当前证据。关闭重开同一项目也能让旧请求重新匹配 id。为每个缓存结果绑定 project 会话代际 + step execution_id；每次 hydrate/close、加载批次替换、执行变更时失效旧请求；提交前核对当前 execution。不能只给 resultsKey 加字段，必须保护异步响应落地。currentResult 同时要求该结果确实已加载且身份一致，validity 必须明确 === ''，缺失不能用 ?? '' 当有效。保留明确历史读取接口或缓存，不以清除所有历史替代身份验证。
2. **P1，同文件110-113。** refresh 只在 await 之前判断 dirty/saving，返回后无 project 会话/草稿 generation/revision 校验。旧 refresh 可在一次 draft 保存完成后覆盖 current 的新 revision/validity，恢复旧“有效”状态。核对 refresh/import/run/flush 响应写 current 的代际契约，至少拦住迟到 refresh；按真实后端 revision/execution 语义实现，不能只固定延迟/禁用轮询。新增受控 deferred 请求测试：A/B 逆序、同项目关闭重开、refresh 与保存交错；断言旧值不落地、不成为当前证据。测试在本机外置缓存运行。
3. **P2，ModuleExplorerView.vue:27-36,62,728-768。** stabilityMap 虽已 gate，选中元素 selectedElement 仍保存旧 stability/kME/GS 快照；改参数后卡片仍称“连接证据”。rean 主结果卡和 SimpleTable 也仅 v-if=rean，未使用 currentResult，hubStale 只看后端 validity、不看本地 dirty。统一当前/历史结果边界：撤销当前结果时清除或降级旧 selection、撤掉当前证据解释/导出；保留历史要在结果所在位置明确标历史及 execution/module，不能只依赖可能折叠的侧栏警告。确认当前 hub 与 rean execution 的关联后才叠加，不只按 gene pair join。对缺 valid/attempted 的旧 schema 不得默认 n_bootstraps 就宣称已知有效分母；标历史口径/未知，别追溯改写结果。

## 限定收尾与验收

- 保持 worker/科学算法不变；这一轮专注 store/UI 正确性。不要新增 AI/provider 或跨模块功能。
- 补上述针对性异步/selector 测试及必要 typecheck；不重复整套 Rust/Go。必要一次隔离 UI 构建后，用新 fixture/项目副本做真正可视验收：无 trait/hub 路径、默认确认 false、B/K/seed 本地更改立即撤证据、UI 图形设置不改科学结果、失效 selection、全量 TSV、关闭重开。若 GUI 无法执行，明确缺口，不当作人工步骤自动关闭。
- 检查上一 prompt 所列 M0 边界，报告/导出尚未接 selector 的入口列明未完成并禁止当当前证据；M2 报告/候选解释尚未实现的部分列表交接，不声称已完成全部 M0-M2。不要为赶 AI 跳过这些。
- 完成更新 HANDOFF/DSH_PROMPT，ready 新 round_id、覆盖全部相关源码 manifest，awaiting_review 后停下。保持源码单写入者，依赖/产物/探针在本机同步盘外，不提交推送/改权限/切 provider/覆盖稳定版或用户项目。

---

以下为历史派发：
# Codex → DSH：当前派发 rean-ui-20261103-05

review_id: rean-ui-20261103-05

已复核rean-project-20261103-04-fix：9文件manifest核对稳定，DSH GUI等待review。Go注册Project真实依赖、step8产物、IsStep与精确输出白名单路径正确，legacy未暴露假线性依赖。独立同步盘外运行TestHubPriority的project包测试通过；API包未能独立运行，原因本机指定离线Go cache缺excelize/v2 v2.11.0（setup失败，非已证实代码回归），不能写成API已独立通过。DSH报告相关Go/Rust通过，真实worker Project e2e仍未跑。可以推进下面UI切片，保留这些验收缺口，不宣布整个M1/M0完成。

## 本轮：M0当前证据selector + 候选优先级UI，不做AI

1. frontend/src/api/projects.ts登记hub_priority StepName/defaultParams与结果schema，参数类型支持bool；默认design_confirmed=false、trait_source=none。旧项目缺该draft时补默认，不自动运行/不覆盖已有历史参数；store.run不把hub设置强加给hub_priority。
2. 实现统一currentResult(step) selector并区分历史：同project、execution、有效性、当前相关草稿与执行参数、dirty/saving/请求代际。有关联参数本地修改立即撤销当前证据，尚未flush也不能继续解释旧值；plot-only不修改科学有效性。异步请求同project旧execution返回也要丢弃。M0 rean/hub及新hub_priority的图/表/证据发现/导出/报告入口统一使用，历史显式查看并注明来源，不伪装新结果。
3. ModuleExplorer新增“候选优先级”tab，运行门控只需filter/network有效、模块已选、设计已确认，不能被原module_trait/hub/trait选择挡住。默认先表格：完整模块规模、K预算、B/seed/设计、工程成本限制，原始kWithin/kME/排名上下界、midrank中位数P05P95、mean_top_k、valid/attempted/invalid、方向未知、局限。无表型标“网络候选优先级”，不能宣称已做性状GS证据。
4. iid/stratified显式选择及未预勾的独立性确认；分层列来自真实sample_info，缺失/不支持设计明确报错，不能猜列。工程预算g²B≤1e9提示只作资源判断，不自动降低B/缩候选/换模块。K有并列时显示分数计入的定义，不称真实hub概率/显著性；有invalid/全部无效时质量提示/null、停止自动可靠推荐。
5. 全量科学结果分页/排序与完整TSV下载，使用新增精确白名单接口，不能截前200作为导出。旧rean的nullable stability/valid分母正确处理，不以0/weight回退替代NA。删“mean>0.99说明阈值低”推断，fragile仅描述性筛选不称噪声/可靠性判定。默认候选表，点击基因局部网络是解释视图，绘图稀疏化不改变科学排序或输入哈希。
6. 关闭打开恢复设置与已提交结果，未执行新step显示not run，不强制旧项目迁移/重算全部来读历史。报告若集成必须只用当前结果并标方法/范围/无效数；完整报告缺口明确列出，不写未实现的性状/AI/跨模块能力。

## 验收与构建

先做前端针对性检查/typecheck，再必要的一次隔离桌面构建/实际UI验收（允许为本轮可见UI做这一构建，不每个小改反复全构建）。使用本机非同步缓存：API测试缺依赖先检查本机已有DSH成功使用的缓存，不能把缓存搬回同步盘；独立检查没跑清楚写未跑。至少验证：无trait/hub可见且可运行、确认默认false、旧结果/模块切换/同project过期请求不可混入、K/B/seed改动即过期、图参数不改结果、>200全量导出、null/invalid/关闭恢复。至少一次真正Rust worker经Project运行小fixture，不能只拿mock测试冒充端到端；不可完成则留明确缺口，不宣布验收关闭。

本轮不要实现AI，不改变provider/ACL/提交推送，不覆盖稳定版或用户正在分析的项目。只用副本/新fixture实测。完成更新双交接，ready新round_id及覆盖全部前后端变更的manifest，awaiting_review等待Codex。M0各子项逐项记录，不一口气写“全部完成”。

---

以下为上一轮历史：

# Codex → DSH：当前派发 rean-project-20261103-04

review_id: rean-project-20261103-04

已复核round rean-worker-fix-20261103-03-fix：设计确认false默认/两方案确认、显式gene ID映射、重复ID拒绝、重采样kWithin Err计invalid、原ME/kME有限性以及稳定排序原因已在源码落地；五文件manifest前后稳定，DSH GUI处于awaiting_review。本轮为源码复核，64tests是DSH报告，未独立全Cargo/worker重跑。主要P1修复通过，可推进Go项目接入，但把下面剩余小口径一起补齐，不另做大重构。

## 本轮范围：Go项目接入 + 两项小修，不扩前端/AI

1. worker read_sample_strata仍用v.is_empty，改拒绝纯空白分层值（明确trim/ID匹配策略，不能随便改变样本身份），补反例。输出当前stratum_count不是每层计数，补对齐后每层数量及分层分配指纹；gene/sample FNV只能是顺序标识，不能替代项目的SHA256输入/执行指纹作缓存有效性判断。逐轮_kme当前算完丢弃，别宣称输出了kME不确定性；本阶段继续仅用其描述性有效性/方向计数，后续UI如展示区间须另存统计。已有1e9的g²B上限是工程预算而非科学阈值，在文档/错误中说明默认200轮可能拒绝2578基因模块；后续性能测量后调整，不能为通过预算截400。
2. backend/internal/project/run.go注册hub_priority依赖仅filter/network、主产物step8_hub_priority.json、validity拓扑遍历和步骤白名单；运行时可选sample_info从项目输入复制，参数中设计确认必须显式true。现有Run把draft写params时，确认network依赖已有效且等于已提交network Params，不绕过当前revision/engine/输入SHA/上游execution校验。
3. 输出提交包括step8_hub_priority.json和TSV，使用当前staging/原子commit；错误/取消不提交半成品done；Result保留历史语义，validity正确。新step在未运行旧项目时返回not run，不改变其他已存在step有效性，不要求module_trait/hub。
4. Project API复用现有run/result/cancel/close端点；如需TSV下载添加精确白名单的只读输出接口，不能任意路径读取。先做到后端API可测，不写前端UI、不加AI。
5. 审查backend/internal/job/job.go的StepOrder；它是legacy线性执行语义，不能仅把新step插数组就让无表型分析强制先跑module_trait/hub。本轮优先Project路径，legacy若不能正确表达依赖则明确未支持且不暴露入口；若注册则加针对性的依赖测试，不以展示顺序作科学依赖。
6. 注意M0当前/历史证据selector和完整导出等前端项仍未验收，后续UI阶段必须补；本轮不能在交接把M0整体标关闭。性状证据目前none，UI/报告不能声称有GS重采样证据。

## 验收

写最小后端fixture（模拟worker用于生命周期/提交测试；至少一次实际新worker小fixture用于端到端，如不可用明确未跑，不能把mock当真实）：无trait/hub即可完成；上下游参数/输入/engine变化使hub_priority失效；仅改变hub显示参数不使其失效；修改B/K/seed/设计需重跑；完整JSON/TSV均提交/可读；取消/失败无部分done；关闭打开恢复且旧project兼容。确认Revision冲突、路径/输出白名单。只跑相关Go核心/HTTP检查、必要Rust针对性测试，不完整桌面构建/快捷方式。已知report_test.go:44基线失败单独记。

修复与接入完成后更新HANDOFF/DSH_PROMPT，写新round_id ready和覆盖Go/worker全部本轮变化的manifest，awaiting_review后等Codex。不要自行进入前端/AI；单一写入者，依赖产物各机同步盘外，不提交推送/切provider/改权限。

---

以下为上一轮历史：

# Codex → DSH：当前派发 rean-worker-fix-20261103-03

review_id: rean-worker-fix-20261103-03

已审round_id rean-worker-20261103-02-fix，当前DSH会话空闲，五文件manifest核对稳定。本轮为源码审查，没有独立重跑完整worker/Cargo，59tests是DSH报告。worker尚不能进入Go集成；先修以下边界并补真实反例，保持仅worker/Rust范围。

1. **P1：设计确认默认自动通过。** worker.rs:241 的HubPriorityParams默认design_confirmed=true，使省略字段也通过iid；1678只检查iid，stratified=false也可运行。默认必须false，任何支持的bootstrap方案均要求明确确认独立性（分层不等于解决重复测量）；两个默认/显式false负例、显式true正例均测。已有fixture应显式填写true，不靠默认。
2. **P1：基因与样本身份没有校验。** 1657/1664用mj.genes/colors位置作为1704表达矩阵行索引，1769输出mj.genes标签；同长度但gene顺序改变会静默错标结果。分析前验证genes/colors/expr行长度、基因ID非空唯一及表达与模块逐行完全一致（或显式ID映射并记录）；样本ID非空唯一。read_sample_strata:1601的map.insert会静默覆盖重复sample ID，必须拒绝重复/空ID、缺失/纯空白标签、错误样本对应。加同长度gene换序/重复gene/短colors/重复sample_info行的端到端失败测试，禁止先索引再校验。
3. **P1：已验证过的数值退化仍能中止整个bootstrap。** 1743对resampled full_module_kwithin用expect：预检查只排除原始值恒定/非有限，实际方差下溢或cor后置检查失败会panic。将可识别的计算退化转为整轮invalid并计原因，仍更新进度；参数/身份错误在循环前拒绝，不把所有程序错误catch成invalid。原ME及原kME在输出前验证有限；否则serde可能把非有限写null而TSV写NaN，不能伪装正常结果。补数值退化处理和全无效/部分无效的worker级计数/JSON/TSV测试，不能只测试validator。
4. **P2：实现/输出与宣称尚有差距。** 1748计算align但未翻转ME，未逐轮计算kME；不要写成“已对齐/已重算kME”。按既定要求完成逐轮描述性kME及方向对齐（不加入排序）；方向为0或接近0时标记不确定，区分网络排名有效性和纯描述性方向不可识别，不能默默声称确定。输出记录strata_column/层计数/实际样本与模块ID顺序指纹、有效network参数与方法版本；invalid_reasons稳定排序。缺的资源估计补预检上限和解释，不能默默截基因。

现有正向无性状fixture和固定seed检查保留。上述测试完成后只做必要worker隔离构建/相关Rust测试，不做完整桌面构建、快捷方式或Go/UI/AI。更新交接后写新ready/完整manifest并等待review。不要直接接Go，不必向用户再问是否继续；保留未知修改，不提交推送/切provider/改权限。

---

以下为上一轮任务历史：

# Codex → DSH：当前派发 rean-worker-20261103-02

review_id: rean-worker-20261103-02

已复核 round_id `rean-rank-20261103-01-fix`：原两项P1与并列注释修复通过；manifest四文件前后哈希一致。Codex同步盘外独立运行当前hub_priority模块12项针对性测试全部通过（rustc测试包装，非全Cargo/完整应用验收）。DSH报告54tests属对方记录，既有report_test.go:44未复测。

本轮执行范围：**M1 worker接线与端到端小型fixture，暂不改Go/前端/AI**。

1. 新增HubPriorityParams、Params字段、dispatch `hub_priority`、run_hub_priority，输出step8_hub_priority.json及完整TSV。仅需step1_filtered.tsv、step3_modules.json、params.json即可做无性状分析，不读取hub/module_trait。参数合法性、原始矩阵、样本ID与module gene顺序全部校验。性状未接入时明确none，非none请求不得静默忽略。
2. 固定完整模块、已提交network的power/network_type；每轮同一索引重采样全部基因，调用共享full_module_kwithin、重新eigengene/kME及rank_round，整轮网络退化记invalid，invalid时也更新进度。不临时删基因或重抽凑满；eigengene方向按原ME相同抽样索引对齐，失败标原因。所有轮同gene顺序/K/tol/方法版本，不能在聚合中混合。
3. 每基因输出原始kWithin/signed及abs kME/原始排名上下界、重采样midrank中位数/P05/P95/mean_top_k_membership及有效数；全无效保留全部基因行和null统计、原始设置K保留。输出attempted/valid/invalid+稳定排序原因统计、method_version、scope=fixed_full_module、样本/模块/参数指纹、分层规则与tie_tol，禁止当真实概率。无效轮出现时标受限，不自动称可靠候选。
4. worker层校验B>0且资源上限、K、模块、设计确认、分层长度/列/缺失。已有采样函数只有长度校验，上层必须做设计确认；paired/重复测量未实现则拒绝。stratified从样本信息对齐生成标签，不能把样本名字猜成分组；无样本信息时iid需明确design_confirmed。
5. 资源估算/上限提示，不偷偷截400。取消/错误不得输出可被提交为done的部分结果，不在每次纯函数修改后刷新桌面快捷方式。原始kWithin与共享函数/run_hub相同定义。
6. 顺手封住聚合公共边界：gene_count>0、每轮top_k在[1,g]；公开可构造RankRound时校验midrank与bounds及membership公式一致，或将构造封装保证不变量。补不合法top_k/伪造bounds等测试，避免仅长度对却带入非法科学数据。

验收：独立fixture不带trait/hub仍成功；K边界并列、全无效保留维度/原始K、部分无效计数正确；非法B/K/未确认设计/错误分层明确失败；固定seed两次数值一致；输出JSON/TSV全量且含局限。相关Rust测试和必要worker隔离构建即可，不跑完整桌面构建，不改其他源文件范围；实测记录明确，测试产物本机同步盘外。

完成后更新HANDOFF/DSH_PROMPT、写新round_id的本机ready.json（包含本轮worker.rs等全部变动文件的源码manifest），awaiting_review并等待Codex。本轮之后才派Go项目接入，再派UI。保留未知修改，不提交推送/切provider/改权限，不另启智能体。

---

以下为已关闭的上一轮历史，不重复实施：

# Codex → DSH：排名纯函数第一轮审查

review_id: rean-rank-20261103-01

用户已明确授权Codex在当前rsWGCNA会话按轮次审查并发送接续任务。先读DSH_REVIEW_LOOP.md，按该约定每轮awaiting_review后等待，不自行开始下一阶段。此条只要求修复排名纯函数的以下问题并验证，先不要接worker/UI。

## 必修

1. **P1：全无效重复时丢失全部基因维度。** 当前aggregate_priority从第一条有效round推断g，空valid_rounds时g=0，返回空数组；测试all(...is_none)在空数组上真，未证明每个基因有null。让函数显式接受预期gene_count（及必要的统一top_k元数据），全无效时输出长度恰好g的None；测试g=3、B100，所有四个统计数组均为3个None，计数100/0/100。
2. **P1：汇总缺输入契约。** 当前attempted<valid使用saturating_sub掩盖错误；各round长度不一致会panic。改为Result，验证attempted>=valid，rank/membership/bounds长度与预期g一致，值有限且范围合理；同一次汇总的K、基因顺序/身份、tol策略在调用边界固定并记录（可用metadata封装，至少不能静默混合不同g/K）。非法输入应Err，不以排序unwrap/越界panic结束。
3. **P2：并列文档与实现不同。** 注释写“transitive grouping”，实现比较组首最大值，实际是组内最大差<=tol。保留当前锚定最大值的有界跨度规则即可，改准确注释/方法版本，并加链式近邻反例[1,0.9,0.8],tol0.11期望[1.5,1.5,3]，避免误称传递并列。明确tol绝对刻度。

Codex已在同步盘外隔离编译当前hub_priority源码复现：全无效输出长度0；attempted1/valid2被接受为invalid0；3基因round+2基因round汇总panic。探针本机指针review-hub-primitives-latest.txt；不是完整应用或完整Cargo复测。不要把这些探针当方法效果验证。

## 交付和后续

补上述回归、正常中位数/分位数用非恒定排名例子验证、入选权重和等于K（浮点容差）、行置换/并列不改变科学结果。保持仅DSH改算法源码，依赖与测试产物在本机同步盘外，不自动提交推送/切provider/改ACL/覆盖稳定版。不必为常规实现再问用户是否继续。

修复完成后如实更新HANDOFF/DSH_PROMPT，写DSH_REVIEW_LOOP.md规定的本机ready.json与源码manifest，状态awaiting_review，当前会话简报完成并等待Codex审查。下一阶段仍为实施prompt的worker集成，但须本轮复核后派发。无需为本轮纯函数变更每次做完整桌面构建或修改快捷方式；跑针对性/相关Rust测试即可，已跑与未跑分开记录。
