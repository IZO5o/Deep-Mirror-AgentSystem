# 待修复 Bug 与工程硬化 Backlog

本文档记录真实验证过程中暴露的问题、风险和后续修复建议。它不是功能规划文档；执行修复前仍需单独生成明确阶段提示词，并遵守 `PROJECT_PROGRAMMING_STANDARD.md`。

## 当前状态

最近一次 Step 12E-2B trace-based evaluation harness 实现结果：

- `go test ./...` 通过。
- `go build ./...` 通过。
- 新增 `coaching_sessions` / `coaching_session_turns` / `coaching_task_attempts`。
- 新增 coaching plan session start/resume、get、submit turn、pause、cancel API。
- 已覆盖 start/resume 幂等、正式回答通过/未通过、提示/解释、暂停/取消、非法状态和 parse failure 测试。
- Step 12C 增强 `mock_interviews` / `mock_turns`，支持 mock session 状态机、role、turn_type、phase、agent_action、content 和 error_message。
- Step 12C 新增 mock start/resume 语义、opening assistant turn、formal answer、hint/explanation、follow-up、topic switch、complete、cancel、failed 路径。
- Step 12C 新增 `POST /api/mock-interviews/:mock_id/cancel`。
- Step 12C 只有 mock formal answer 更新 `practice_states`；hint/explanation/cancel/parse failure 不更新。
- Step 12C 将 formal answer turns 与 `practice_states` 更新放入同一事务，practice update 失败时回滚本轮 turns。
- Step 12C 没有引入 MCP/native function calling/ReAct。
- Step 12D-1 新增 service-controlled 内部 practice state update BusinessTool/helper，mock 和 coaching 复用同一 upsert 逻辑。
- Step 12D-1 已让 coaching formal answer 自动更新 `practice_states`，source_type 为 `coaching_task_attempt`，source_id 为本次 attempt_id。
- Step 12D-1 没有引入 MCP/native function calling/ReAct，也没有让 Agent 自由调用工具。
- Step 12D-2 新增 completed coaching session / completed mock interview 手动生成 `memory_candidates`。
- Step 12D-2 新增 `memory_candidates.source_ref_type` / `source_ref_id`，用于追踪候选来自 coaching session 或 mock interview。
- Step 12D-2 生成候选幂等：同一 source_ref 已有 pending/accepted candidates 时直接返回，不重复调用 Agent。
- Step 12D-2 没有直接写 `memory_items`，仍保持 accept/reject 边界。
- Step 12E-1 新增 `agent_decision_traces` 表，记录关键 Agent 执行快照。
- Step 12E-1 新增 `GET /api/agent-decision-traces` 只读查询 API，支持 user/interview/source/agent/step/status 过滤。
- Step 12E-1 覆盖 coaching plan generation、coaching session turn、mock start、mock turn、completed coaching/mock memory candidate generation。
- Step 12E-1 selected context snapshot 覆盖 coaching plan generation、mock start、mock turn。
- Step 12E-1 Agent run/parse/persist 失败会尽量保存 failed trace，trace 保存失败不阻断主业务。
- Step 12E-2A 新增 failure injection tests，覆盖 coaching/mock/memory candidate generation 的关键失败路径。
- Step 12E-2A 新增 coaching/mock golden tests，用固定 Agent JSON 锁定主要状态转移。
- Step 12E-2A 测试断言 `agent_decision_traces`，把 trace 作为状态转移和失败恢复的可验证证据。
- Step 12E-2B 新增 `GET /api/agent-evaluations` 只读 API，基于已有 trace 实时计算规则评测报告。
- Step 12E-2B evaluation harness 检查 trace 完整性、JSON 可解析性、selected context 结构、service_actions 关键动作、failed trace 可定位性和 `memory_items` 写入边界。
- Step 12E-2B 不新增业务状态，不调用真实 LLM，不做 LLM-as-judge，不改变业务状态机和长期记忆写入边界。
- `go test -tags real_hardening ./server -run '^$'` 通过，仅编译 build-tag 隔离测试，未调用真实 LLM。
- Step 11 已知验证：`go test -tags real_step11 ./server -run TestStep11ConstructedLongTranscriptSegmentedFlow -v` 通过。
- 构造长 transcript 字符数：`13084`。
- segmented review 已触发。
- `transcript_segments`：Step 11 验证中为 `3`，全部 `extracted`；Step 12A 已将 segment 输入大小调低，后续同一文本可能生成更多 segment。
- review report：`generated`。
- questions：`10`。
- memory candidates：`9`。
- accepted memory item：已生成。
- coaching plan/tasks：已生成。
- mock interview/turn/practice_states：已生成并 completed。
- segment extraction prompt 已限制输出规模。
- segment JSON parse failure 已增加一次 compact strict retry。
- 已新增 transcript segments 只读调试 API。
- 已新增 MemorySelector selected context 动态只读调试 API。

结论：主链路已经能完成真实 LLM 下的长 transcript segmented review 到 coaching/mock/practice state 闭环。Step 12A 已完成第一轮 review pipeline 稳定性和可观测性硬化，但更小 chunk fallback 仍保留为后续优化。

## Bug 1：超长 segment 的 LLM 输出被截断导致 JSON 解析失败

### 状态

已完成第一轮修复。

Step 12A 已实现：

- 将 `segmentMaxChars` 从 `6000` 调低到 `4000`。
- 将 `segmentOverlapChars` 从 `300` 调低到 `200`。
- 在 segment extraction prompt 中限制 `question_candidates`、`key_evidence`、`uncertain_parts` 最多 5 条。
- 限制 `segment_summary`、`evidence_text` 等字段长度，并要求不要输出 Markdown/code fence。
- segment JSON parse failure 后自动进行一次 compact strict retry。
- retry 仍失败时保留最后一次 `raw_agent_output` 和 `error_message`，segment/report 进入 failed，不产出正式 questions。

### 触发场景

Step 11 第一次真实验证中，subagent 生成了约 `27.5k` 字的长 transcript。进入 segmented review 后，某个 segment 的 LLM 输出过长，导致：

- segment 2 的模型输出被截断。
- JSON fence 未闭合。
- segment review JSON 解析失败。
- 整个 segmented review 链路失败。

随后将 fixture 压缩到约 `13k` 字后，链路通过。

### 影响

这是长 transcript 真实场景下的重要稳定性问题：

- Step 12A 前 `segmentMaxChars = 6000`，对某些内容仍可能导致 per-segment 输出过长。
- Step 12A 前 segment extraction prompt 允许模型输出较多 `question_candidates`、`key_evidence`、`speaker_role_notes`，输出长度不可控。
- Step 12A 前一旦单个 segment 解析失败，整次 review 会失败。
- 对真实面试长录音来说，这是可能复现的生产风险。

### 临时规避

- 使用较短 transcript 或较短音频片段。
- 控制测试 fixture 在约 `13k` 字以内。
- 避免单个 segment 中包含过多重复问答或证据。

### 后续优化方向

当前已完成 prompt 限制、segment 大小调整和 strict retry。后续如真实更长文本仍暴露截断，可继续考虑：

1. 更小 chunk fallback。
   - parse 失败后将单个 segment 临时拆成更小 chunk。
   - 分别 extraction 后合并为原 segment result。

2. 增加 JSON 修复或容错策略。
   - 可以考虑对常见 code fence 截断、尾部缺失做有限修复。
   - 但不能把不完整语义强行当成功结果。
   - 修复失败仍应标记 failed。

3. 扩展真实 LLM 长文本稳定性测试。
   - 当前已新增 `real_hardening` build tag 测试。
   - 后续可覆盖 `25k+` transcript，但需评估费用、耗时和输出稳定性。

### 建议优先级

P1。

原因：主链路可跑通，但真实长音频/长 transcript 稳定性会受这个问题影响。Step 12A 已完成第一轮 review pipeline 硬化，后续继续观察真实长文本表现。

## Bug 2：真实音频 ASR 样本过短，未覆盖真实长 transcript 分段

### 状态

已识别，非代码 bug。

### 现象

真实 MP3 的 1/10 片段 ASR 成功，但 transcript 只有约 `1006` 字符，未超过 `longTranscriptThresholdChars = 12000`，因此没有触发 segmented review。

### 影响

- 真实 ASR 链路已验证。
- 真实音频到“长 transcript segmented review”的端到端仍未完全覆盖。
- 当前 segmented review 的真实 LLM 验证依赖构造长 transcript fixture，而不是直接来自真实长音频 ASR。

### 建议处理

- 保留当前结论：真实 ASR 链路通过，构造长 transcript 的真实 LLM 主链路通过。
- 后续如需更强验证，可使用更长音频或完整音频做 ASR，但要评估费用、耗时、文件大小和模型限制。
- 不建议为了验证强行把有声书内容适配成面试内容。

### 建议优先级

P2。

## Risk 1：MemorySelector 结果未落库，调试依赖 prompt

### 状态

已进一步缓解。

Step 12A 选择轻量方案 A：新增动态只读 debug API，不落库：

- `GET /api/interviews/:interview_id/selected-context?user_id=...&target_round=...&current_task=coaching_plan|mock_start|mock_turn`
- 返回 selected memory_items、selected practice_states、score、selection_reason。

Step 12E-1 已新增 `agent_decision_traces`：

- coaching plan generation、mock start、mock turn 会保存 selected context snapshot。
- 可通过 `GET /api/agent-decision-traces` 查询当时选中的 memory_items / practice_states、score 和 selection_reason。

限制：`GET /api/interviews/:interview_id/selected-context` 仍然是重新执行 selector 的动态结果；历史快照应以 `agent_decision_traces` 为准。

### 现象

Step 10A 中 `MemorySelector` 已将 `score` 和 `selection_reason` 注入 prompt，但当时没有保存 selector snapshot。

### 影响

- coaching plan/mock start/mock turn 已可通过 trace 复盘当时为什么选择这些 memory/practice_states。
- 未覆盖的路径仍可继续按需扩展 trace。

### 建议处理

后续如需更强追踪，可继续扩大 selected context snapshot 覆盖范围，或把 selector 作为受控 read tool 接入后续 evaluation harness。

### 建议优先级

P2。

## Risk 2：缺少 transcript_segments 调试 API

### 状态

已完成。

Step 12A 新增只读 API：

- `GET /api/interviews/:interview_id/transcript-segments`

返回 segment id、offset、char_count、summary、status、error_message、结构化 extraction 字段和 `content_preview`。不返回完整 content 和 raw agent output，避免响应过大。

### 现象

`transcript_segments` 已落库，但目前没有 API 查看分段摘要、状态和失败原因。

### 影响

- 真实验证需要直接查 SQLite。
- 用户或前端无法观察长 transcript 分段进度和失败点。
- 出现 segment failed 时，定位体验较差。

### 建议处理

已通过只读 API 处理；后续如需要前端展示，可在不增加写 API 的前提下做简单调试面板。

### 建议优先级

P2。

## Risk 3：coaching session practice_states 更新

### 状态

已完成。

### 现象

Step 12B 只落库 coaching session、turn 和 formal answer attempt，不把二面辅导中的正式训练回答自动写入 `practice_states`。

Step 12D-1 已接入：
- 仅 `formal_answer` 更新 `practice_states`。
- `hint_request` / `explanation_request` / `pause` / `skip_task` 不更新。
- Agent run failure / parse failure 不更新。
- practice update 与本轮 user turn、assistant turn、`coaching_task_attempt`、task/session 状态更新在同一事务内。
- source_type 使用 `coaching_task_attempt`，source_id 使用本次 attempt_id。

### 影响

- 当前二面辅导 session 可以推进 task、记录 attempt，并同步 practice state。
- practice state 画像现在能反映 coaching formal answer 和 mock formal answer 两类训练。

### 建议处理

已通过内部 BusinessTool/helper 接入；仍保持 `memory_items` 写入边界不变，不让 second_round_coach 直接写长期记忆。

## Risk 4：coaching session selected context 未保存快照

### 状态

已部分缓解，后续优化。

### 现象

Step 12B 的 session turn prompt 主要使用 plan/task/recent turns，未每轮重新运行 MemorySelector。

Step 12E-1 已为 coaching session turn 保存 decision trace，包括 input snapshot、raw agent output、parsed decision 和 service actions；但因为该路径当前没有 selector 调用，所以没有 selected context snapshot。

### 影响

- 当前状态机和错误恢复已可测试。
- 真实运行后可以复盘本轮 Agent 决策和服务端动作。
- 如后续要求 coaching session 每轮动态选择记忆，还需要在该路径接入 MemorySelector 并保存 selected context snapshot。

### 建议处理

后续可评估 coaching session turn 是否每轮运行 MemorySelector，并把结果写入 existing trace。

## 下一阶段代办：Step 12 收尾后的工程落地

Step 12E-2B 已完成。Step 12 到此收尾，不继续在 Step 12 下扩 MCP/function calling/ReAct。后续建议：

```text
API/用户流程梳理 -> 前端或调试页面 -> README/demo guide -> 真实或构造数据 demo -> 简历/面试材料 -> 少量 bug hardening
```

Step 12B 已完成内容：
- 用户入口是某次 interview/review 对应的“开始/继续二面辅导”。
- 一个 coaching session 绑定整个 `coaching_plan_id`。
- 一个 session 内推进多个 `coaching_tasks`，通过 `current_task_id`、task 状态和 attempt 记录表达进度。
- 不为每个 task 默认创建短会话。
- 具体产品视角见 `COACHING_PRODUCT_USER_FLOW.md`。

Step 12C 已完成内容：
- 用户入口是某次 interview/review 对应的“开始/继续模拟面试”。
- 一个 active mock interview 会话可恢复，避免重复创建。
- opening / user answer / evaluation / follow-up / topic switch / closing / error 都作为结构化 `mock_turns` 保存。
- 只有正式回答驱动 `practice_states` 更新，且与本轮 turn 同事务提交。
- `mock_interviewer` 不写 `memory_items`，长期记忆写入流程不变。

Step 12D-1 已完成内容：
- 新增内部 practice state update BusinessTool/helper，不暴露给 LLM。
- mock 和 coaching 复用统一 `practice_states` upsert、平滑更新和 dimension 推断逻辑。
- coaching formal answer 更新 practice state；非正式输入和失败路径不更新。
- practice update 失败时回滚本轮 coaching turn/attempt/task/session 状态。

Step 12D-2 已完成内容：
- completed coaching session 可生成 `memory_candidates`。
- completed mock interview 可生成 `memory_candidates`。
- 仅生成候选，不直接写 `memory_items`。
- 候选带 `source_ref_type/source_ref_id`。
- 重复触发具备幂等性，不重复调用 `memory_curator`。

Step 12E-1 已完成内容：
- 新增 Agent Decision Trace / Selected Context Snapshot 第一版。
- 关键 Agent 执行保存 selected context snapshot、输入摘要、raw output、parsed decision、service actions、status/error。
- 新增只读 trace 查询 API。
- parse failure 会保存 failed trace。
- trace 不作为业务事实来源，不改变状态机和记忆边界。

Step 12E-2A 已完成内容：
- failure injection tests：coaching Agent run/parse failure、coaching practice update failure。
- failure injection tests：mock Agent run/parse failure、mock practice update failure。
- failure injection tests：completed coaching/mock memory candidate generation parse failure。
- golden tests：coaching formal answer passed/failed、hint、explanation、skip。
- golden tests：mock start、follow-up、switch topic、complete、hint、cancel。
- 每类测试都断言 trace 的 status/source/step/service_actions 等关键字段。

Step 12E-2B 已完成内容：
- 新增 trace-based evaluation service。
- 新增只读 API `GET /api/agent-evaluations`，复用 trace 查询参数。
- evaluation 只读取 `agent_decision_traces` 并实时计算，不落库，不作为业务事实来源。
- 规则覆盖成功 trace、失败 trace、selected context、service_actions、memory_items 写入边界。
- 测试覆盖 evaluation 成功/失败/边界和 controller 过滤。

Step 12 收尾边界：
- Step 12 已完成到 E2B，不继续无限扩展。
- 不在 Step 12 下继续扩 MCP/function calling/ReAct。
- 后续应转入 API 体验整理、前端/调试页面、README/demo 文档、真实或构造数据 demo、简历亮点整理、少量真实 LLM 验证和 bug hardening。
