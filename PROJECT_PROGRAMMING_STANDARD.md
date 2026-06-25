# 项目编程标准与协作边界

本文档是本项目后续规划、提示词生成、vibecoding 执行和代码审查的共同基准。任何新会话在动手前都应先阅读本文档；如果项目阶段、表结构、API 或边界发生变化，应同步更新本文档。

## 1. 项目一句话定位

这是一个 Go 实现的面向求职面试复盘与训练执行的多 Agent Web 项目，主线是把真实面试输入转化为结构化复盘、用户确认后的长期记忆、二面准备计划、模拟面试训练和动态练习状态。项目后续方向是任务型 Agent 系统：Agent 不只是聊天或一次性生成文本，而是在受控业务状态中完成具体训练目标，并体现 agent loop、tool calling、状态机、错误恢复和评测能力。

核心主线必须始终保持为：

```text
真实面试输入 -> transcript -> review -> confirmed long-term memory -> coaching -> mock interview -> practice state -> 更精准辅导
```

后续 Agent 工程主线必须围绕“可执行任务”展开：

```text
训练目标 -> 读取业务上下文 -> 制定下一步动作 -> 调用受控业务工具 -> 持久化状态 -> 错误恢复 -> 评测任务完成度
```

## 2. 固定的 4 个 Agent 及职责

当前业务只允许固定 4 个 Agent。后续不得新增新的业务 Agent 类型。

### review

复盘分析 Agent。

职责：
- 一次性或分段分析 interview transcript。
- 生成结构化问答 `interview_questions`。
- 生成整体复盘报告 `interview_review_reports`。
- 只基于 transcript、面试元信息、必要摘要和关键片段工作，不凭空编造面试内容。

### memory_curator

记忆整理 Agent。

职责：
- 从真实面试复盘结果中生成 `memory_candidates`。
- 只生成候选长期记忆，不直接写入正式长期记忆。
- 候选记忆必须等待用户 accept/reject。

### second_round_coach

二面辅导 Agent。

职责：
- 根据复盘结果、已确认长期记忆、练习状态、公司画像、岗位画像和目标轮次生成准备计划。
- 生成 `coaching_plans` 和 `coaching_tasks`。
- 复习计划、二面准备计划、答法改进建议都归入该 Agent。

注意：不要再引入 `study_planner` 作为业务 Agent。当前代码中的 `study_planner` 只能视为历史兼容别名，不应在新业务设计中继续扩展。

### mock_interviewer

模拟面试 Agent。

职责：
- 基于公司画像、岗位要求、用户历史弱点、coaching plan 和 practice state 进行模拟面试。
- 生成 `mock_interviews` 和 `mock_turns`。
- 每轮 mock turn 后自动更新 `practice_states`。
- 不写入 `memory_items`。

## 3. 已完成主链路

当前已完成到 Step 12E-2B，主链路包括：

1. 多 Agent 路由底座。
2. `interview_sessions` / `interview_transcripts`。
3. 手动 transcript 写入。
4. 音频/视频上传，保存 `media_files`。
5. 异步转写任务，保存 `transcription_jobs`。
6. 视频提取音频。
7. 调用 ASR 写入 `interview_transcripts`。
8. 转写成功后 `interview_sessions.status = ready_for_review`。
9. 长 transcript 自动切分为 `transcript_segments`。
10. `review` Agent 对 segment 做分段结构化提取。
11. `review` Agent 对分段结果做 final merge。
12. 最终生成 `interview_questions` / `interview_review_reports`。
13. `memory_curator` 生成 `memory_candidates`。
14. 用户 accept/reject candidate。
15. accepted candidate 写入 `memory_items`。
16. `second_round_coach` 生成 `coaching_plans` / `coaching_tasks`。
17. `MemorySelector` 为 `second_round_coach` 选择相关 `memory_items` / `practice_states`。
18. `mock_interviewer` 生成 `mock_interviews` / `mock_turns`。
19. `MemorySelector` 为 mock start / mock turn 选择相关 `memory_items` / `practice_states`。
20. mock turn 自动更新 `practice_states`。
21. 真实 LLM 端到端闭环已有 build tag 隔离测试。
22. Step 12A 已完成 review pipeline 硬化：segment 输出规模限制、parse failure 一次 compact strict retry、失败 raw output/error 留存。
23. Step 12A 已新增 transcript_segments 只读调试 API。
24. Step 12A 已新增 MemorySelector selected context 动态只读调试 API。
25. Step 12B 已新增 plan 级别 `coaching_sessions`，一个 session 绑定整个 `coaching_plan_id` 并通过 `current_task_id` 推进多个 `coaching_tasks`。
26. Step 12B 已新增 `coaching_session_turns` 和 `coaching_task_attempts`，记录多轮交互、正式回答尝试、评分、反馈、raw output 和错误。
27. Step 12B 已实现 coaching session start/resume、submit turn、pause、cancel 和显式状态校验。
28. Step 12C 已将 `mock_interviewer` 升级为 mock session 状态机：start/resume、opening question、formal answer、hint/explanation、follow-up、topic switch、complete、cancel、failed。
29. Step 12C 已增强 `mock_interviews` / `mock_turns` 字段，保存 role、turn_type、phase、agent_action、content、current_topic、last_feedback 和 error_message。
30. Step 12C 只让 mock formal answer 更新 `practice_states`；hint/explanation/cancel/parse failure 不更新，且 practice update 与 turn 保存同事务。
31. Step 12D-1 已新增 service-controlled 内部 BusinessTool/helper，用于统一 `practice_states` upsert；mock formal answer 和 coaching formal answer 均复用该工具。
32. Step 12D-1 已让 coaching session formal answer 自动更新 `practice_states`，source_type 为 `coaching_task_attempt`，source_id 为本次 attempt_id，且与本轮 turn/attempt/task/session 更新同事务。
33. Step 12D-2 已支持 completed coaching session / completed mock interview 手动触发 `memory_curator` 生成 `memory_candidates`。
34. Step 12D-2 已为 `memory_candidates` 增加 `source_ref_type` / `source_ref_id`，用于追踪候选来自 review、coaching session 或 mock interview。
35. Step 12E-1 已新增 `agent_decision_traces`，记录关键 Agent 执行的 selected context snapshot、输入摘要、raw output、parsed decision、service actions 和 status/error。
36. Step 12E-1 已新增 `GET /api/agent-decision-traces` 只读查询 API，支持按 user/interview/source/agent/step/status 过滤。
37. Step 12E-2A 已新增 failure injection tests，覆盖 Agent run failure、JSON parse failure、practice update failure、memory candidate generation failure 等关键失败路径。
38. Step 12E-2A 已新增 coaching/mock 状态机 golden tests，用固定 Agent JSON 锁定主要状态转移，并断言 `agent_decision_traces`。
39. Step 12E-2B 已新增 trace-based evaluation harness，基于已有 `agent_decision_traces` 实时做规则评测，不新增业务状态。
40. Step 12E-2B 已新增 `GET /api/agent-evaluations` 只读 API，支持复用 trace 查询参数输出评测报告。

## 4. 两类记忆的边界

### memory_items：用户确认后的长期画像记忆

`memory_items` 是正式长期记忆，必须走如下路径：

```text
review result -> memory_curator -> memory_candidates -> 用户 accept -> memory_items
```

允许记录：
- 用户薄弱点。
- 用户优势。
- 公司画像。
- 岗位画像。
- 面试官专业关注点。
- 高频追问模式。
- 准备建议。

硬性规则：
- Agent 不允许直接写入 `memory_items`。
- `second_round_coach` 不允许写入 `memory_items`。
- `mock_interviewer` 不允许写入 `memory_items`。
- 未经用户确认的内容只能停留在 `memory_candidates`。
- 不记录面试官年龄、性别、外貌、家庭、婚姻、私人身份等隐私属性。

### practice_states：训练过程自动更新的动态掌握度记忆

`practice_states` 是训练状态，不需要用户每次确认。

允许由 mock formal answer 和 coaching formal answer 自动更新：
- topic。
- dimension。
- mastery_score。
- attempt_count。
- last_score。
- last_feedback。
- last_practiced_at。
- source_type / source_id。

硬性规则：
- `practice_states` 只表达练习掌握度和最近反馈。
- 不替代 `memory_items`。
- 不存储未经确认的长期画像判断。
- 不改变 candidate -> accept -> memory_items 的长期记忆写入流程。

## 5. 当前阶段和后续阶段

### 当前阶段

Step 12E-2B 已进入当前代码：trace-based evaluation harness 雏形，是 Step 12 的收尾阶段。

当前能力：
- 上传音频/视频。
- 保存 `media_files` / `transcription_jobs`。
- 视频文件通过 ffmpeg 提取音频。
- 通过 ASR 转写。
- 写入或更新 `interview_transcripts`。
- 更新 interview status 为 `ready_for_review`。
- 短 transcript 继续由 `review` Agent 直接复盘。
- 长 transcript 自动生成 `transcript_segments`。
- 分段使用 `review` Agent 提取 segment summary、speaker role notes、question candidates、key evidence 和 uncertain parts。
- final merge 使用 `review` Agent 合并分段结果，最终仍写入 `interview_questions` / `interview_review_reports`。
- `second_round_coach` 不再无差别注入全部 active `memory_items`。
- `mock_interviewer` 在 mock start / mock turn 中不再无差别注入全部 active `memory_items`。
- `MemorySelector` 基于 user、company、job、target_round、current_task、掌握度和置信度选择少量 `memory_items` / `practice_states`。
- prompt 中包含 selector 的 `score` 和 `selection_reason`，方便调试和面试讲解。
- 长 transcript segment extraction 已降低单段大小：`segmentMaxChars = 4000`，`segmentOverlapChars = 200`。
- segment extraction prompt 已限制输出规模：最多 5 个 question candidates / key evidence / uncertain parts，且限制 summary/evidence 长度。
- segment JSON parse failure 会触发一次 compact strict retry；retry 失败后 segment/report failed，不写半成品 questions。
- 可通过 `GET /api/interviews/:interview_id/transcript-segments` 查看 segment summary/status/error/content_preview。
- 可通过 `GET /api/interviews/:interview_id/selected-context` 动态查看 MemorySelector 选择结果、score 和 selection_reason。
- 可通过 `POST /api/coaching-plans/:plan_id/sessions` 开始或恢复某个 coaching plan 的 plan 级别 session。
- 一个 coaching session 绑定整个 `coaching_plan_id`，在会话内通过 `current_task_id` 推进多个 `coaching_tasks`。
- `SubmitCoachingSessionTurn` 调用固定 `second_round_coach` 输出严格 JSON，由 service 层判断 formal answer / hint / explanation / skip / pause 并落库。
- 正式回答写入 `coaching_task_attempts`；达标后 task 标记 done 并推进下一个 task，未达标进入 needs_revision。
- 本阶段没有接入 MCP/native function calling/ReAct，也没有让 `second_round_coach` 写 `memory_items`。
- coaching session formal answer 会通过 service-controlled 内部 helper 更新 `practice_states`，但 hint / explanation / pause / skip / parse failure 不更新。
- `mock_interviewer` 仍不写 `memory_items`，只在正式回答轮通过 service 层更新 `practice_states`。
- `POST /api/interviews/:interview_id/mock-interviews` 现在是 start/resume 语义，同一 active mock 不重复创建。
- `POST /api/mock-interviews/:mock_id/turns` 会根据 Agent JSON 决策写入 user/evaluation/followup/topic_switch/closing/error 等多条结构化 `mock_turns`。
- mock turn 的 Agent/JSON parse failure 会保存 raw output/error，mock 进入 failed，不更新 `practice_states`。
- completed coaching session 可通过 `POST /api/coaching-sessions/:session_id/memory-candidates` 手动生成长期记忆候选。
- completed mock interview 可通过 `POST /api/mock-interviews/:mock_id/memory-candidates` 手动生成长期记忆候选。
- session/mock 来源的候选只写入 `memory_candidates`，不会直接写 `memory_items`。
- 同一 `source_ref_type/source_ref_id` 已有 pending 或 accepted 候选时，重复触发直接返回已有候选，不重复调用 Agent。
- `agent_decision_traces` 记录关键 Agent 执行过程，不作为业务事实来源。
- trace 覆盖 coaching plan generation、coaching session turn、mock start、mock turn、completed coaching/mock memory candidate generation。
- selected context snapshot 至少覆盖 coaching plan generation、mock start、mock turn。
- Agent run/parse/persist 失败会尽量保存 failed trace，trace 保存失败不阻断主业务流程。
- 可通过 `GET /api/agent-decision-traces?user_id=&interview_id=&source_type=&source_id=&agent_type=&step_name=&status=` 查询最近 trace。
- failure injection tests 会故意注入 Agent error、非法 JSON、practice update 写入失败和 memory curator parse failure，验证不会留下半成品业务状态。
- golden tests 用固定 Agent JSON 固化 coaching/mock 主要状态转移，并把 trace 作为可断言的工程证据。
- 可通过 `GET /api/agent-evaluations?user_id=&interview_id=&source_type=&source_id=&agent_type=&step_name=&status=&limit=` 对最近 trace 做实时规则评测。
- evaluation harness 只读取 `agent_decision_traces`，检查 trace 完整性、JSON 可解析性、selected context 结构、service_actions 关键动作、failed trace 可定位性和 `memory_items` 写入边界。
- Step 12 到 E2B 后停止继续扩展；后续不继续在 Step 12 下扩 MCP、function calling 或 ReAct，优先进入 API/用户流程、前端或调试页、README/demo、真实或构造数据 demo、简历材料和少量 hardening。

### Step 9：长 transcript 分段处理

已完成：
- 真实 transcript 过长时，不再直接全量塞给 `review` Agent。
- 已设计 `transcript_segments`、分段摘要、问答候选抽取和 final merge。
- 最终 `interview_questions` / `interview_review_reports` 仍由 review 业务链路产出。

边界：
- 不新增 Agent 类型。
- 可以复用 `review` Agent 或新增 review task mode。
- 不做 RAG。
- 不改 `memory_items` / `practice_states` 设计。
- 不做 MCP/function calling。
- 不做复杂 UI。

### Step 10：MemorySelector 选择性上下文注入

Step 10A 已完成：
- 不再把所有 `memory_items` / `practice_states` 都塞进 prompt。
- 根据 `user_id`、`company_name`、`job_title`、`target_round`、`current_task` 选择相关记忆。
- 服务对象先限定为 `second_round_coach` 和 `mock_interviewer` 的一次性生成场景。
- 使用 SQL/filter/ranking，输出 selected memories、selected practice states、selection_reason 和 score。

边界：
- 先用 SQL/filter/ranking。
- 不做向量 RAG，除非后续明确需要。
- 不新增 Agent 类型。
- 不改变 memory 写入流程。
- 不做 MCP。
- 输出 selected memories 和 selection_reason，方便调试和面试讲解。

### Step 11：端到端真实文件测试

目标：
- 用真实音频/视频文件跑完整流程：

```text
upload media -> transcription -> transcript -> segmented review -> memory candidate -> accept -> coaching plan -> mock interview -> practice_states
```

边界：
- 默认 `go test ./...` 不跑真实 LLM/真实 ASR。
- 可以使用 `real_llm` build tag 或手动验证命令。
- 使用临时 DB 或隔离测试数据。
- 重点验证主链路，不新增功能。

### Step 12A：review pipeline 与可观测性硬化

已完成：
- 降低 segment 输入大小和 overlap，减少 per-segment 输出过长风险。
- segment extraction prompt 明确限制输出条数和字段长度。
- segment parse failure 后进行一次 compact strict retry。
- retry 失败时保存最后一次 raw output/error，segment/report failed，不写正式 questions。
- 新增 transcript_segments 只读调试 API。
- 新增 MemorySelector selected context 动态只读调试 API。
- 新增 `real_hardening` build tag 测试，隔离真实 LLM 长文本稳定性验证。

边界：
- 不新增 Agent 类型。
- 不改长期记忆写入流程。
- 不引入 MCP/native function calling。
- 不做复杂 UI。
- 不把真实 LLM/ASR 放进普通 `go test ./...`。
- 更小 chunk fallback 暂未实现，保留为后续优化。

### Step 12B：coaching plan session 状态机和多 task 运行模型

已完成：
- 新增 `coaching_sessions`，保存 plan 级别辅导会话、当前 task、状态、进度摘要、最后 Agent 消息和错误。
- 新增 `coaching_session_turns`，保存 user/assistant/system 轮次、turn type、agent action、score、feedback、raw output 和错误。
- 新增 `coaching_task_attempts`，保存正式回答尝试、分数、反馈、是否达标和 attempt index。
- 新增 start/resume、get、submit turn、pause、cancel API。
- session start/resume 幂等：同一 plan 的 active session 不重复创建。
- submit turn 通过固定 `second_round_coach` 输出严格 JSON，由 service 层执行状态转移。
- JSON parse/agent failure 会保存 raw output/error，session 进入 failed，不写半成品 attempt/task done。

边界：
- 不新增 Agent 类型。
- 不引入 MCP/native function calling/ReAct。
- 不开放工具调用。
- 不改 `memory_candidates -> accept/reject -> memory_items` 流程。
- 不让 `second_round_coach` 直接写 `memory_items`。
- Step 12D-1 后，coaching formal answer 已通过内部 helper 更新 `practice_states`。
- 不破坏现有一次性 `GenerateCoachingPlan` 流程。

### Step 12C：mock interview execution 状态机和多轮运行模型

已完成：
- `mock_interviews.status` 支持 `created`、`in_progress`、`waiting_answer`、`evaluating_answer`、`asking_followup`、`switching_topic`、`completed`、`failed`、`cancelled`。
- `mock_turns` 支持 `role`、`turn_type`、`phase`、`agent_action`、`content`、`error_message`，兼容原有 question/answer/feedback/score/next_question 字段。
- start/resume 幂等：同一 user/interview/plan/round 的 active mock 直接返回已有 session。
- start 成功后写入 opening assistant turn，状态进入 `waiting_answer`。
- submit turn 使用固定 `mock_interviewer` 输出严格 JSON，由 service 层执行 formal answer / hint / explanation / cancel / follow-up / topic switch / complete 分支。
- 只有 formal answer 且 Agent 决策允许时更新 `practice_states`；非正式输入、取消和失败路径不更新。
- formal answer 的 user/evaluation/action turns 与 `practice_states` 更新在同一事务里，practice update 失败时不落半成品 turn。
- 新增 `POST /api/mock-interviews/:mock_id/cancel`。

边界：
- 不新增 Agent 类型。
- 不引入 MCP/native function calling/ReAct。
- 不开放业务写工具给 Agent。
- 不改 `memory_candidates -> accept/reject -> memory_items` 流程。
- 不让 `mock_interviewer` 直接写 `memory_items`。
- 保留原有 start/turn/complete API，并做兼容增强。

### Step 12D-1：内部 BusinessTool/helper 与 coaching practice state 更新

已完成：
- 新增 service-controlled practice state update helper，作为内部 BusinessTool，不暴露给 LLM 自由调用。
- helper 输入为 user_id、topics、score、feedback、source_type、source_id，在事务内 upsert `practice_states`。
- 继续复用 mastery_score 平滑更新与 dimension 推断逻辑。
- `mock_interviewer` 的 formal answer practice update 迁移到通用 helper，现有行为保持不变。
- `second_round_coach` 的 coaching session formal answer 会创建 `coaching_task_attempt` 后，同事务更新 `practice_states`。
- coaching practice state topic 优先取当前 `coaching_task.title`，为空再退到 task_type / description，避免额外 LLM 判断。
- source_type 新增 `coaching_task_attempt`，source_id 使用本次 attempt_id。

边界：
- 不做 OpenAI function calling。
- 不做 ReAct。
- 不做 MCP。
- 不让 Agent 自由选择或调用工具。
- 不生成 memory_candidates。
- 不改变 `memory_items` 写入边界。
- 不做 selected context snapshot / decision trace。

### Step 12D-2：completed session/mock 长期记忆候选生成

已完成：
- 新增 `POST /api/coaching-sessions/:session_id/memory-candidates`。
- 新增 `POST /api/mock-interviews/:mock_id/memory-candidates`。
- 仍使用固定 `memory_curator` Agent 生成候选，不新增业务 Agent。
- 仅 `completed` coaching session / mock interview 允许生成候选。
- prompt 明确只生成长期稳定观察，不把单次分数、一次性失误、临时情绪或 practice state 流水账写成长期记忆。
- 新增 `memory_candidates.source_ref_type` / `source_ref_id`，coaching 使用 `coaching_session/session_id`，mock 使用 `mock_interview/mock_id`。
- 幂等策略：同一 source_ref 下已有 pending 或 accepted candidate 时，直接返回已有候选，不重复调用 Agent。

边界：
- 不直接写 `memory_items`。
- 不改变 accept/reject 语义。
- 不让 `second_round_coach` 或 `mock_interviewer` 直接写长期记忆。
- 不在未完成、failed、cancelled、paused、in_progress 状态生成候选。
- 不做 MCP/native function calling/ReAct。

### Step 12E-1：Agent Decision Trace 与 selected context snapshot

已完成：
- 新增 `agent_decision_traces` 表，保存关键 Agent 执行的轻量决策轨迹。
- 新增 `GET /api/agent-decision-traces` 只读查询 API。
- trace 字段包括 selected context snapshot、input snapshot、raw agent output、parsed decision、service actions、status 和 error_message。
- 覆盖 `coaching_plan_generate`、`coaching_session_turn`、`mock_start`、`mock_turn`、`coaching_session_memory_candidates`、`mock_interview_memory_candidates`。
- `coaching_plan_generate`、`mock_start`、`mock_turn` 会保存 MemorySelector 的 selected context snapshot。
- Agent run failure、JSON parse failure、业务持久化失败都会尽量记录 failed trace。
- trace 保存失败只记录 warn，不阻断主业务流程。

边界：
- trace 不是业务事实来源，不替代 `memory_candidates`、`practice_states` 或状态机表。
- 不保存完整长 transcript、媒体内容或完整大 prompt，只保存结构化摘要和 prompt length。
- 不做 OpenAI function calling。
- 不做 ReAct。
- 不做 MCP。
- 不新增业务 Agent。
- 不改变 `memory_items` 写入边界。

### Step 12E-2A：Failure Injection 与 Golden Tests

已完成：
- 新增 failure injection tests，覆盖 coaching session Agent run failure、parse failure、practice state update failure。
- 新增 failure injection tests，覆盖 mock turn Agent run failure、parse failure、practice state update failure。
- 新增 failure injection tests，覆盖 completed coaching/mock memory candidate generation parse failure。
- 新增 coaching golden tests，覆盖 formal answer passed、formal answer failed、hint、explanation、skip task。
- 新增 mock golden tests，覆盖 mock start、formal answer follow-up、switch topic、complete、hint、cancel。
- golden/failure tests 会断言 `agent_decision_traces` 的 status、agent_type、source、step、raw output、parsed decision、service_actions 和 selected context snapshot。

边界：
- 只做测试与少量测试 helper，不新增业务功能。
- 不做 OpenAI function calling。
- 不做 ReAct。
- 不做 MCP。
- 不改变现有业务 API。
- 不改变 `memory_items` 写入边界。

### Step 12E-2B：Trace-Based Evaluation Harness 与 Step 12 收尾

已完成：
- 新增 `AgentEvaluationReportVO` / `AgentEvaluationResultVO` / `AgentEvaluationCheckVO`。
- 新增 `GET /api/agent-evaluations` 只读 API，复用 `agent_decision_traces` 查询参数。
- 新增 trace-based evaluation service，实时读取 trace 后执行规则评测，不落库 evaluation result。
- 评测覆盖 trace 完整性、JSON/schema 可解析性、selected context 是否存在且包含 `selected_memory_items` / `selected_practice_states` / `debug_summary`、service_actions 是否符合关键状态机动作、failed trace 是否可定位错误、是否违反 `memory_items` 写入边界。
- 测试覆盖成功 trace、失败 trace、非法 JSON、缺关键 service action、直接写 memory_items 边界违规和 controller 过滤。

边界：
- 不做复杂评测平台。
- 不做 LLM-as-judge。
- 不调用真实 LLM。
- 不新增 Agent。
- 不做 MCP。
- 不做 ReAct。
- 不做 OpenAI function calling。
- 不改变业务状态机。
- 不改变 `memory_items` 写入边界。
- Step 12 到 E2B 后停止继续扩展。

后续工程落地方向：
- API/用户流程梳理，确认完整 demo 路径。
- 前端或简单调试页面，演示核心闭环。
- README + demo guide。
- 真实或构造数据 demo。
- 简历/面试讲述材料。
- 少量 bug hardening。

后续可选方向：
- MCP 外部工具接入。
- 外部题库。
- 文件知识库。
- 日历提醒。

边界：
- 必须在 Step 9-12E-1 主流程稳定后再讨论。
- 不允许提前塞进 Step 9-12E-1。
- 本地业务 DB 查询优先使用 server-side service/business tools，MCP 优先用于外部系统接入。
- 不做没有明确训练目标的泛聊天增强。

## 6. 明确不做什么

以下事项默认不做，除非用户明确改变项目方向：

- 不新增第 5 个业务 Agent。
- 不把 `study_planner` 重新发展成独立业务 Agent。
- 不绕过 `memory_candidates` 直接写 `memory_items`。
- 不让 mock turn 写正式长期记忆。
- 不在 Step 9-12E-1 引入 MCP。
- 不在 Step 9-12E-1 引入 native function calling 工程增强。
- 不在 Step 9 做向量 RAG。
- 不在 Step 9 做复杂前端 UI。
- 不把所有历史记忆无差别塞进 prompt。
- 不为了炫技扩大功能面，优先保证主链路稳定、可解释、可测试。
- 不把本项目改成通用聊天机器人、通用知识库、通用 Agent 平台。
- 不把长对话做成无目标闲聊；后续长对话必须围绕可执行训练任务、状态推进和可评测结果设计。
- 不把二面辅导默认拆成每个 `coaching_task` 一个孤立短会话；优先采用 `coaching_plan` 级别长会话，在一个会话内推进多个 task。

## 7. API / 数据表总览

### 核心业务 API

Conversation 底座：
- `POST /api/conversation`
- `GET /api/conversation`
- `PATCH /api/conversation/:conversation_id`
- `DELETE /api/conversation/:conversation_id`
- `POST /api/conversation/:conversation_id/message`
- `GET /api/conversation/:conversation_id/message`

Interview 与 transcript：
- `POST /api/interviews`
- `GET /api/interviews`
- `GET /api/interviews/:interview_id`
- `PUT /api/interviews/:interview_id/transcript`
- `GET /api/interviews/:interview_id/transcript`

媒体上传与转写：
- `POST /api/interviews/:interview_id/media`
- `GET /api/interviews/:interview_id/media`
- `GET /api/transcription-jobs/:job_id`

Review：
- `POST /api/interviews/:interview_id/review`
- `GET /api/interviews/:interview_id/review`
- `GET /api/interviews/:interview_id/questions`
- `GET /api/interviews/:interview_id/transcript-segments`
- `GET /api/interviews/:interview_id/selected-context?user_id=...&target_round=...&current_task=coaching_plan|mock_start|mock_turn`

Memory：
- `POST /api/interviews/:interview_id/memory-candidates`
- `GET /api/interviews/:interview_id/memory-candidates`
- `POST /api/memory-candidates/:candidate_id/accept`
- `POST /api/memory-candidates/:candidate_id/reject`
- `GET /api/memory-items`

Coaching：
- `POST /api/interviews/:interview_id/coaching-plan`
- `GET /api/interviews/:interview_id/coaching-plan`
- `GET /api/coaching-plans/:plan_id/tasks`
- `POST /api/coaching-plans/:plan_id/sessions`
- `GET /api/coaching-sessions/:session_id`
- `POST /api/coaching-sessions/:session_id/turns`
- `POST /api/coaching-sessions/:session_id/pause`
- `POST /api/coaching-sessions/:session_id/cancel`
- `PATCH /api/coaching-tasks/:task_id`

Mock interview：
- `POST /api/interviews/:interview_id/mock-interviews`
- `GET /api/mock-interviews/:mock_id`
- `POST /api/mock-interviews/:mock_id/turns`
- `GET /api/mock-interviews/:mock_id/turns`
- `POST /api/mock-interviews/:mock_id/complete`
- `POST /api/mock-interviews/:mock_id/cancel`

Practice state：
- `GET /api/practice-states`
- `GET /api/practice-states/:state_id`

Trace / evaluation：
- `GET /api/agent-decision-traces`
- `GET /api/agent-evaluations`

### 数据表

Conversation 底座：
- `conversations`
- `chat_messages`

Interview 输入：
- `interview_sessions`
- `interview_transcripts`
- `media_files`
- `transcription_jobs`

Review 输出：
- `transcript_segments`
- `interview_questions`
- `interview_review_reports`

长期记忆：
- `memory_candidates`
- `memory_items`

二面辅导：
- `coaching_plans`
- `coaching_tasks`
- `coaching_sessions`
- `coaching_session_turns`
- `coaching_task_attempts`

模拟面试：
- `mock_interviews`
- `mock_turns`

动态练习状态：
- `practice_states`

### 当前重要状态值

Interview：
- `created`
- `ready_for_review`
- `reviewed`

Review report：
- `generated`
- `failed`

Memory candidate：
- `pending`
- `accepted`
- `rejected`

Memory item：
- `active`
- `archived`

Media file：
- `uploaded`
- `processing`
- `transcribed`
- `failed`

Transcription job：
- `queued`
- `processing`
- `succeeded`
- `failed`

Transcript segment：
- `pending`
- `extracted`
- `failed`

Coaching plan：
- `generated`
- `failed`

Coaching task：
- `todo`
- `in_progress`
- `needs_revision`
- `done`
- `skipped`

Coaching session：
- `created`
- `in_progress`
- `waiting_user_answer`
- `evaluating`
- `needs_revision`
- `task_completed`
- `paused`
- `completed`
- `failed`
- `cancelled`

Mock interview：
- `in_progress`
- `completed`
- `failed`

## 8. 给 vibecoding AI 的协作规则

### 每次开始前

执行会话必须先阅读：
- `PROJECT_PROGRAMMING_STANDARD.md`
- 本次任务相关 service/controller/VO/test 文件

不要只看用户当前一句话就直接改代码。

### 每个阶段的执行指令必须包含

- 当前已完成内容。
- 本阶段目标。
- 明确不做什么。
- 建议新增/修改哪些表、API、service、测试。
- 验收目标。

### 实现原则

- 优先沿用现有 Go/Gin/Gorm/service/controller/test 结构。
- 表结构新增要集中考虑 `server/db.go` 和 VO。
- API 新增要同步 controller、service、测试。
- 业务逻辑优先放在 server service 层，不要塞进 Agent runtime。
- 后续引入 agent loop/tool calling 时，必须先定义业务状态机、工具白名单、权限边界、错误恢复策略和评测标准。
- Prompt 构造保持结构化 JSON 输入输出，解析失败要保存 raw output 或给出可定位错误。
- 二面辅导长会话后续按 `coaching_plan` 级别设计，不按单个 task 默认拆成短会话；`coaching_task` 是会话内部推进和评测的训练单元。
- 默认测试使用 fake runner、fake ASR、临时 DB。
- 真实 LLM/真实 ASR 测试必须通过 build tag 或手动验证隔离。
- 每次实现后至少跑 `go test ./...`。

### 规划会话的规则

负责生成提示词和规划的会话也必须遵守本文档。

规划会话应：
- 先判断当前代码是否已经变化。
- 先总结当前阶段，再给下一步指令。
- 如果发现代码与本文档不一致，先指出差异。
- 如果阶段完成、表结构/API 变化或边界变化，要提醒更新本文档。
- 用户问实现路线时，先讨论设计和边界，再给执行提示词。

规划会话不应：
- 直接把未确认的新功能塞进执行指令。
- 为了显得高级而引入 Agent、MCP、RAG、function calling。
- 忽略 `memory_items` 和 `practice_states` 的边界。
- 忽略真实主线闭环。
- 把任务型 Agent 退化成普通聊天，缺少可验证目标、状态推进和评测。

### 跑偏预警

如果某个需求会导致以下情况，应先收敛再执行：

- 新增业务 Agent。
- 改写长期记忆写入规则。
- 把 mock 结果自动沉淀为正式长期记忆。
- 在长 transcript 分段阶段引入向量库。
- 在 MemorySelector 之前做复杂知识库。
- 在端到端真实文件测试之前做 MCP/function calling。
- 大幅改造 Agent runtime，而 service 层能解决问题。
- 在没有状态机、错误恢复和评测设计前，直接开放工具调用。

## 9. 后续维护规则

当以下内容发生变化时，必须更新本文档：

- Agent 列表和职责。
- 主链路阶段。
- 数据表。
- API。
- 记忆边界。
- Step 9-12 的阶段划分。
- 明确不做事项。
- vibecoding 协作规则。

后续每一份给执行会话的提示词，都应显式要求执行会话先阅读并遵守 `PROJECT_PROGRAMMING_STANDARD.md`。
