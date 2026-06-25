# 项目工程亮点记录

本文档用于持续记录本项目在编程过程中的工程亮点，方便后续写简历、项目介绍和面试讲解。每完成一个阶段，都应补充本文件。

## 1. 项目定位亮点

项目不是通用聊天机器人，而是面向求职面试复盘与训练执行的多 Agent Web 系统。

主线闭环：

```text
真实面试输入 -> transcript -> review -> confirmed long-term memory -> coaching -> mock interview -> practice state -> 更精准辅导
```

后续演进方向：

```text
训练目标 -> 读取业务上下文 -> 制定下一步动作 -> 调用受控业务工具 -> 持久化状态 -> 错误恢复 -> 评测任务完成度
```

## 2. 多 Agent 业务边界

固定 4 个业务 Agent：

- `review`：复盘分析，产出结构化问答和报告。
- `memory_curator`：生成候选长期记忆，等待用户确认。
- `second_round_coach`：生成二面准备计划和任务。
- `mock_interviewer`：执行模拟面试和追问。

亮点：
- 没有无限扩展 Agent 类型，而是用清晰职责边界支撑业务闭环。
- `study_planner` 被约束为历史兼容别名，不作为新业务 Agent 扩展。
- review 和 memory 写入边界分离，避免 Agent 自动污染长期记忆。

## 3. 真实输入链路

已实现真实音频/视频上传与异步转写：

- `media_files` 保存上传文件元数据。
- `transcription_jobs` 管理异步转写状态。
- worker 处理 queued jobs。
- 视频通过 ffmpeg 提取音频。
- OpenAI-compatible ASR 写入 `interview_transcripts`。
- 成功后更新 `interview_sessions.status = ready_for_review`。
- 失败写入 `error_message`，不破坏已有 transcript/review 链路。

工程亮点：
- 上传、任务、处理、结果写入解耦。
- 状态清晰：uploaded/processing/transcribed/failed，queued/processing/succeeded/failed。
- 支持真实文件验证，同时默认测试用 fake ASR 隔离外部依赖。

## 4. 长 Transcript 分段 Review

Step 9 已实现长文本分段处理：

- 短 transcript 继续直接 review。
- 长 transcript 自动切成 `transcript_segments`。
- 每段由 review Agent 生成：
  - segment summary
  - speaker role notes
  - question candidates
  - key evidence
  - uncertain parts
- 最后由 review Agent 做 final merge，产出正式：
  - `interview_questions`
  - `interview_review_reports`

工程亮点：
- 没有新增 Agent，而是复用 review Agent 的不同 task mode。
- 原始 `interview_transcripts.content` 保留完整原文。
- 中间分段结果落库，便于后续可观测性和错误恢复。
- 避免长 transcript 直接塞入 prompt。

已完成的硬化点：
- 真实 27.5k 字 transcript 中，某段 LLM 输出过长导致 JSON 被截断。
- Step 12A 已围绕 segment 输出限制、一次 compact strict retry 和调试可观测性完成第一轮工程硬化。

## 5. 长期记忆写入边界

项目区分两类记忆：

### memory_items

正式长期画像记忆。

写入路径：

```text
review result -> memory_curator -> memory_candidates -> 用户 accept -> memory_items
```

亮点：
- Agent 不直接写正式长期记忆。
- 用户确认作为长期记忆写入边界。
- 支持 user/company/job/interviewer/question pattern/preparation tip 等多种记忆类型。
- 对面试官隐私属性有过滤规则。

### practice_states

训练动态掌握度记忆。

来源：
- mock turn 自动更新。
- coaching formal answer 自动更新。

亮点：
- 将“长期画像”和“训练状态”分离。
- mock/coaching 可以自动更新掌握度，但不能污染正式长期记忆。
- 支持 mastery_score、attempt_count、last_feedback、last_practiced_at 等字段。

## 6. MemorySelector 选择性上下文注入

Step 10A 已实现 server-side MemorySelector：

- `SelectMemoriesForCoaching`
- `SelectMemoriesForMock`

选择对象：
- `memory_items`
- `practice_states`

选择依据：
- user_id
- company_name
- job_title
- target_round
- current_task
- memory_type
- confidence
- mastery_score
- last_score
- attempt_count
- recency

输出：
- selected item
- score
- selection_reason

工程亮点：
- 不再无差别注入全部长期记忆。
- 不引入向量库，先用结构化 SQL/filter/ranking 保持可测和可解释。
- coaching 和 mock 使用不同 ranking profile。
- selection_reason 便于调试和面试讲解。

已完成的硬化点：
- selector 结果仍主要注入 prompt。
- Step 12A 已新增 selected context 动态只读 debug API；如需审计级追踪，后续可落库 snapshot。

## 7. 一次性业务生成闭环

当前已经能完成：

- review report/questions
- memory candidates
- memory item accept
- coaching plan/tasks
- mock interview start
- mock turn
- practice state update
- mock complete

工程亮点：
- service 层负责业务编排和持久化。
- Agent 输出使用严格 JSON schema。
- parse failure 会保存 failed report/plan 或 raw output。
- 普通测试使用 fake runner，不依赖真实 LLM。
- 真实 LLM 测试通过 build tag 隔离。

## 8. 真实验证与评测隔离

已形成的验证策略：

- `go test ./...`：默认不调用真实 LLM/ASR。
- fake runner/fake ASR：验证业务流程和状态转换。
- `real_llm` build tag：验证真实 LLM 业务闭环。
- `real_step11` build tag：验证真实长 transcript 主链路。

工程亮点：
- 外部依赖隔离，不污染普通 CI/本地快速测试。
- 真实验证和单元测试分层。
- Step 11 暴露真实 LLM 输出截断问题，并沉淀为工程硬化 backlog。

## 9. Step 12A 工程硬化亮点

Step 12A 已完成 review pipeline 与可观测性硬化：

已完成：
- 提升长 transcript review pipeline 稳定性。
- 提升分段和 selector 的可观测性。
- 为后续任务型 Agent 的状态机和错误恢复打基础。

工程亮点：
- 将 segment 输入大小从 `6000/300` 调整为更保守的 `4000/200`。
- 在 segment extraction prompt 中限制 `question_candidates`、`key_evidence`、`uncertain_parts` 最多 5 条。
- 对 `segment_summary`、`evidence_text`、`speaker_role_notes` 增加明确长度和简洁性约束，降低 LLM 输出截断概率。
- 对 segment JSON parse failure 增加一次 compact strict retry，retry prompt 进一步限制最多 3 个 question candidates。
- retry 失败时保留最后一次 raw output 和 error_message，segment/report 明确 failed，不写半成品 questions。
- 新增 `GET /api/interviews/:interview_id/transcript-segments`，只读查看 segment 状态、summary、错误和 content preview。
- 新增 `GET /api/interviews/:interview_id/selected-context`，动态查看 MemorySelector 选择的 memory_items / practice_states、score 和 selection_reason。
- 新增 `real_hardening` build tag 测试，隔离真实 LLM 长文本稳定性验证，并记录 segment count/status counts/retry count。

保留风险：
- 当前没有实现更小 chunk fallback；如果更长 transcript 仍出现截断，可在后续阶段补充。
- selected-context debug API 是动态重算，不是 coaching/mock 生成当时的快照；如需审计级追踪，可后续落库 snapshot。

可用于面试表达：

```text
在真实长文本验证中发现 LLM 输出截断导致 JSON 解析失败，
我没有简单扩大 prompt 或忽略错误，而是把它沉淀为 pipeline hardening：
限制输入和输出规模、增加有限重试、保存失败定位信息、
补充只读调试 API 和 build tag 隔离的真实 LLM 回归测试。
```

## 10. Step 12B coaching session 状态机亮点

Step 12B 已将 `second_round_coach` 从一次性 plan 生成，推进到 plan 级别任务会话的第一版。

已完成：
- 新增 `coaching_sessions`，一个 session 绑定整个 `coaching_plan_id`，通过 `current_task_id` 在多个 `coaching_tasks` 间推进。
- 新增 `coaching_session_turns`，记录 user / assistant / system 轮次、turn_type、agent_action、score、feedback、raw output 和 error。
- 新增 `coaching_task_attempts`，只记录正式回答尝试、分数、反馈、是否达标和 attempt index。
- Start/Resume 幂等：同一个 plan 的 active session 不重复创建。
- Submit turn 使用固定 `second_round_coach` 输出严格 JSON，service 层执行状态转移，不开放工具调用。
- formal answer 达标后 task 标记 done 并推进下一个 task；不达标进入 needs_revision。
- hint/explanation 不写 attempt，不把 task 标记 done。
- paused / completed / failed / cancelled 等状态有明确非法提交保护。
- parse failure 保存 raw output/error，session 进入 failed，不写半成品 attempt/task done。

工程亮点：
- 把“二面辅导”建模为可恢复的业务状态机，而不是一次 prompt 或泛聊天。
- 用户入口是“继续这次二面准备”，不是为每个 task 创建孤立短会话。
- Agent 只负责结构化判断，业务状态转移、落库和边界控制由 service 层完成。
- 普通测试使用 fake runner 覆盖成功、失败、提示、暂停、取消、非法状态和解析失败。

保留风险：
- Step 12E-1 后已保存关键 Agent decision trace；coaching session turn 目前仍不每轮重新运行 MemorySelector。
- 暂未引入 MCP/native function calling/ReAct；这些留到后续状态机稳定后再设计。

可用于面试表达：

```text
我没有把二面辅导做成普通聊天，而是抽象成 plan 级别 session 状态机：
一个会话推进多个训练任务，正式回答会形成 attempt，Agent 输出只作为结构化判断，
最终由服务端控制状态转移、任务完成、错误落库和恢复边界。
```

## 11. Step 12C mock interview 状态机亮点

Step 12C 已将 `mock_interviewer` 从一次性 next-question 生成，推进为可恢复的模拟面试状态机。

已完成：
- `POST /api/interviews/:interview_id/mock-interviews` 具备 start/resume 语义，同一 active mock 不重复创建。
- start 成功后写入 opening assistant turn，mock 进入 `waiting_answer`。
- `mock_turns` 新增 role、turn_type、phase、agent_action、content、error_message，用结构化 turn 表达 user answer、evaluation、follow-up、topic switch、closing、error。
- submit turn 由固定 `mock_interviewer` 输出严格 JSON，service 层执行 formal_answer / hint_request / explanation_request / cancel 分支。
- formal answer 后可 ask_followup、switch_topic 或 complete。
- 只有 formal answer 更新 `practice_states`；hint/explanation/cancel/parse failure 不更新。
- formal answer turns 与 `practice_states` 更新同事务，practice update 失败时回滚本轮 turns。
- 新增 `POST /api/mock-interviews/:mock_id/cancel`。
- parse/agent failure 保存 raw output/error，mock 进入 failed。

工程亮点：
- 把“模拟面试”建模为可恢复、可审计的业务状态机，而不是单条 prompt 生成下一问。
- Agent 只输出结构化决策，服务端负责状态转移、事务边界和长期记忆边界。
- `mock_interviewer` 不写 `memory_items`，只通过受控 service 更新练习掌握度。
- 普通测试使用 fake runner 覆盖 start/resume、正式回答、提示、切题、完成、取消、失败和事务回滚。

可用于面试表达：

```text
mock interviewer 不再只生成下一道题，而是输出结构化决策：
服务端把用户回答、评分反馈、追问、切题和总结拆成可审计 turn，
并且只在正式回答轮用事务更新 practice state，避免失败时出现半成品训练状态。
```

## 12. Step 12D-1 内部 BusinessTool/helper 亮点

Step 12D-1 没有做 OpenAI function calling，而是先沉淀 service-controlled 内部业务工具层。

已完成：
- 新增内部 practice state update BusinessTool/helper，只由 service 显式调用，不暴露给 LLM。
- helper 输入 user_id、topics、score、feedback、source_type、source_id，在事务内 upsert `practice_states`。
- 继续复用 mastery_score 平滑更新和 dimension 推断逻辑。
- mock formal answer 迁移到通用 helper，保持原有 practice state 行为。
- coaching formal answer 创建 `coaching_task_attempt` 后，同事务更新 `practice_states`。
- source_type 新增 `coaching_task_attempt`，source_id 使用本次 attempt_id。
- coaching topic 由当前 `coaching_task` 稳定派生，优先 title，再退到 task_type / description。
- practice update 失败时，本轮 coaching turn、attempt、task/session 状态全部回滚。

工程亮点：
- 先把“工具”定义为服务端受控业务动作，而不是让 Agent 自由调工具。
- 读写边界清晰：Agent 只给结构化判断，service 决定是否允许更新训练状态。
- `memory_items` 边界不变，coaching/mock 都不能直接写长期记忆。
- 普通测试覆盖 formal answer 通过/未通过、提示/解释/跳过/暂停不更新、parse failure 不更新、事务回滚和 mock 行为不回退。

可用于面试表达：

```text
我没有一上来开放 function calling，而是先把高风险写操作沉淀成内部 BusinessTool：
由 service 在状态机校验后调用，并放进同一个事务。
这样 Agent 不能自由写业务状态，但训练掌握度更新逻辑可以在 coaching 和 mock 之间复用。
```

## 13. Step 12D-2 session 记忆候选亮点

Step 12D-2 已把 completed coaching/mock 的长期观察沉淀为候选记忆，但仍坚持用户确认边界。

已完成：
- 新增 `POST /api/coaching-sessions/:session_id/memory-candidates`。
- 新增 `POST /api/mock-interviews/:mock_id/memory-candidates`。
- completed coaching session / mock interview 才允许生成候选。
- 继续使用固定 `memory_curator` Agent，不新增业务 Agent。
- 新增 `memory_candidates.source_ref_type` / `source_ref_id`，追踪候选来自 coaching session 或 mock interview。
- 同一 source_ref 已有 pending/accepted candidates 时直接返回已有结果，不重复调用 Agent。
- prompt 明确只生成长期稳定观察，不把单次分数、一次性失误、临时情绪或 practice state 流水账写成长期记忆。

工程亮点：
- 把训练过程中的长期观察沉淀到 `memory_candidates`，但不绕过用户确认。
- 通过 source_ref 实现候选来源可追溯，方便前端调试和后续审计。
- 通过幂等策略避免手动重复触发导致候选膨胀。
- `second_round_coach` / `mock_interviewer` 仍不直接写 `memory_items`。

可用于面试表达：

```text
我把 session 结束后的长期观察做成候选记忆生成，而不是直接写长期记忆：
memory_curator 只产出 memory_candidates，并通过 source_ref 追踪来源；
用户 accept 后才进入 memory_items，既能沉淀训练洞察，也不会污染长期画像。
```

## 14. Step 12E-1 Agent Decision Trace 与评测基础

Step 12E-1 新增轻量 Agent Decision Trace / Selected Context Snapshot，把关键 Agent 执行从“只看最终回复”推进到可复盘 observe / decide / act / persist 全过程。

为什么需要 trace：
- 任务型 Agent 的质量不能只看最终自然语言回复，还要能解释它读取了什么上下文、做了什么结构化决策、服务层实际持久化了什么动作。
- coaching/mock 已经是受控状态机，后续要做 golden tests、failure injection 和 evaluation harness，必须有可查询的历史执行快照。
- MemorySelector 之前只有动态 debug API，无法复盘“当时”选中的上下文；trace 保存 selected context snapshot 后，可以做真实运行后的回放。
- parse failure、run failure、persist failure 需要沉淀为 failed trace，便于验证没有半成品业务状态。

trace 记录了什么：
- `selected_context_snapshot`：MemorySelector 选中的 memory_items / practice_states、score 和 selection_reason。当前覆盖 coaching plan generation、mock start、mock turn。
- `input_snapshot`：本次 Agent 输入摘要，例如 interview/session/mock/task id、状态、turn count、question count、prompt length，不保存完整长 transcript 或大 prompt。
- `raw_agent_output`：Agent 原始输出，便于复盘模型到底返回了什么。
- `parsed_decision`：parse 成功后的结构化 JSON，例如 coaching session 的 input_type/score/next_action，mock turn 的 next_action/practice_updates。
- `service_actions`：service 根据 Agent 输出实际执行的动作，例如 created coaching_plan、created coaching_tasks、recorded coaching_session_turn、recorded coaching_task_attempt、updated practice_states、created mock_turns、generated memory_candidates。
- `status/error`：成功或失败，以及 run/parse/persist error 的简要信息。

当前覆盖路径：
- `coaching_plan_generate`：记录 `second_round_coach` 生成 plan/tasks 的 selected context、raw output、parsed plan 和创建动作。
- `coaching_session_turn`：记录每次 session turn 的输入摘要、Agent decision 和 service-driven 状态更新。
- `mock_start`：记录 `mock_interviewer` 开场时读取的 selected context、parsed opening question 和创建 mock/turn 动作。
- `mock_turn`：记录 mock 每轮回答后的 selected context、parsed feedback/next action 和 practice state 更新动作。
- `coaching_session_memory_candidates`：记录 completed coaching session 触发 `memory_curator` 后生成候选的过程。
- `mock_interview_memory_candidates`：记录 completed mock interview 触发 `memory_curator` 后生成候选的过程。

如何支撑后续 evaluation harness：
- 可评估 Agent 是否读取了正确上下文：通过 selected context snapshot 检查是否选中了相关 memory_items / practice_states。
- 可评估 parsed decision 是否符合 schema 和业务状态：例如 hint 不应更新 practice_states，formal answer 才能触发 scoring 和 practice update。
- 可评估 service actions 是否符合状态机：例如 completed session 才能生成 memory_candidates，mock turn 是否创建了正确类型的 turns。
- 可做 failure injection：注入 malformed JSON、模型错误、持久化错误后，验证 failed trace 存在且没有半成品业务状态。
- 可做 golden tests：对比同一输入下的状态转移、service actions 和失败恢复是否稳定。

为什么先做 service-controlled trace，而不是直接做 ReAct/MCP：
- 当前项目核心是受控业务状态机，关键风险在“Agent 输出后服务端如何安全落库”，不是让 Agent 自由调用工具。
- trace 先覆盖本地业务 loop，可以在不改变状态机和记忆边界的前提下增加可观测性。
- 后续如果接入 function calling/MCP，也可以复用这套 trace 字段记录 tool calls、selected context 和 service actions。

可用于面试表达：

```text
我没有把 Agent 当成黑盒聊天接口，而是为每一次关键决策保存 selected context、
输入摘要、原始输出、结构化决策和服务端动作。这样可以回放 Agent 为什么这么做，
也能把状态机正确性和失败恢复纳入自动化评测。
```

## 15. Step 12E-2A Failure Injection 与 Golden Tests

Step 12E-2A 在 trace 基础上补齐 failure injection tests 和状态机 golden tests，用自动化测试证明任务型 Agent 链路在成功路径和失败路径下都不会污染业务状态。

failure injection 是什么：
- 故意模拟 Agent run failure，例如 fake runner 返回 `model unavailable`。
- 故意模拟 JSON parse failure，例如 Agent 返回 `not json`。
- 故意模拟 practice update failure，例如在写 `practice_states` 时注入数据库错误。
- 故意模拟 memory candidate generation failure，例如 completed coaching/mock 调用 `memory_curator` 后解析失败。
- 验证失败后不会留下半成品 `turn`、`attempt`、`practice_state`、`memory_candidate` 或 `memory_item`。

golden tests 是什么：
- 用固定初始状态和固定 Agent JSON，锁定 coaching/mock 状态机的标准转移。
- coaching golden 覆盖 formal answer passed、formal answer failed、hint、explanation、skip task。
- mock golden 覆盖 mock start、formal answer follow-up、switch topic、complete、hint、cancel。
- 这些测试把“状态机应该怎么走”固定成可回归的标准剧本，防止后续改 prompt 或 service 逻辑时悄悄破坏边界。

它们和 trace 的关系：
- trace 不只是日志，而是测试可断言的工程证据。
- tests 会检查 `agent_decision_traces.status`、`agent_type`、`source_type/source_id`、`step_name`。
- 成功路径会检查 `raw_agent_output`、`parsed_decision`、`service_actions`。
- 失败路径会检查 `error_message`、`raw_agent_output` 和 failed `service_actions`。
- mock start / mock turn 等路径会检查 selected context snapshot，证明当时的上下文选择可回放。

为什么这体现任务型 Agent 工程能力：
- Agent 输出不是最终业务事实，而是服务端状态机的 decision input。
- 服务端负责校验状态、执行事务、更新 task/session/mock/practice state，并在失败时回滚半成品状态。
- 测试覆盖 observe / decide / persist / failure recovery，不只是验证 happy path 的自然语言回复。
- 这让项目从“能跑通”升级到“关键状态转移和失败恢复可证明”。

和后续 evaluation harness 的关系：
- E2A 先固化状态机和失败恢复。
- E2B 再基于 trace 做 evaluation harness 雏形，例如批量检查 service_actions 是否符合状态机、parsed decision 是否符合 schema、失败注入后是否无半成品状态。
- 完成 E2A/E2B 后，Step 12 不再继续无限扩展，后续应转入 API 体验、前端/调试页面、README/demo、简历亮点、少量真实 LLM 验证和 bug hardening。

边界：
- 仍不做 ReAct。
- 仍不做 MCP。
- 仍不做 OpenAI function calling。
- 不新增业务 Agent。
- 当前优先工程落地，不继续扩大 Step 12 架构范围。

可用于面试表达：

```text
我没有只验证 happy path，而是做了 failure injection 和 golden tests：
用 fake Agent 固定输出模拟成功、失败和非法 JSON，验证 coaching/mock 状态机不会产生半成品数据；
同时用 decision trace 断言服务端实际执行的状态转移动作，为后续 Agent evaluation harness 提供可回放证据。
```

## 16. Step 12E-2B Trace-Based Evaluation Harness

Step 12E-2B 基于 `agent_decision_traces` 新增轻量 evaluation harness，用规则评测任务型 Agent 的工程行为，而不是先评价自然语言是否优美。

为什么不直接评自然语言：
- 当前系统的关键风险不是“回复像不像人”，而是 Agent 是否在业务状态机里可靠执行任务。
- coaching/mock 的核心质量首先体现在是否读取了正确上下文、是否输出可解析 JSON、服务端是否执行了正确动作、失败时是否能定位且不留下半成品状态。
- 自然语言质量更适合后续真实 LLM build tag 或 LLM-as-judge 单独评测；默认测试要稳定、低成本、可重复。

当前评测分层：
- trace 完整性：succeeded trace 应有 raw output、parsed decision、service actions；failed trace 应有 error message 和调试 payload。
- JSON/schema 可解析性：`parsed_decision`、`selected_context_snapshot`、`input_snapshot`、`service_actions` 非空时必须是合法 JSON。
- selected context：`coaching_plan_generate`、`mock_start`、`mock_turn` 必须保存 selected context，且包含 `selected_memory_items`、`selected_practice_states`、`debug_summary`。
- service actions：按 `step_name` 检查关键动作，例如 coaching plan 创建 plan/tasks、mock start 创建 mock/opening turn、mock turn 创建 turns 并按需更新 practice_states、memory candidate generation 只生成 candidates。
- failed trace：失败 trace 必须能通过 `error_message` 和 `service_actions` 定位失败阶段。
- memory boundary：`service_actions` 不允许出现直接 created/updated `memory_items`，长期记忆仍必须走 candidate accept。

它如何承接 E1/E2A：
- E1 保存 trace，把 Agent 的 observe/decide/act/persist 快照落成可查询数据。
- E2A 用 failure injection 和 golden tests 固定状态机成功/失败路径。
- E2B 用 trace 做规则评测，把这些快照转成可批量检查的 report。

当前实现方式：
- 新增 `GET /api/agent-evaluations` 只读 API，复用 trace 查询参数。
- 新增 evaluation service，实时读取 `agent_decision_traces` 并计算 report，不落库。
- 新增 VO：`AgentEvaluationReportVO`、`AgentEvaluationResultVO`、`AgentEvaluationCheckVO`。
- 每条 trace 按 checks 计算简单百分制，全部通过为 100，不引入复杂权重。

边界：
- 不做复杂评测平台。
- 不做 LLM-as-judge。
- 不调用真实 LLM。
- 不新增业务 Agent。
- 不做 MCP。
- 不做 ReAct。
- 不做 OpenAI function calling。
- 不改变业务状态机。
- 不改变 `memory_items` 写入边界。

Step 12 收尾：
- Step 12 到 E2B 后停止继续扩展。
- 后续转入工程落地收尾：API/用户流程梳理、前端或调试页面、README/demo guide、真实或构造数据 demo、简历/面试材料和少量 bug hardening。

可用于面试表达：

```text
我把 Agent 评测拆成可自动化的工程层：不是先用 LLM 评价回复好不好，
而是基于 decision trace 检查上下文选择、JSON 决策、服务端动作、失败恢复和记忆边界。
这样评测稳定、可重复，也能证明 Agent 在业务状态机里可靠执行任务。
```

## 17. 后续任务型 Agent 亮点规划

后续计划将 `second_round_coach` 和 `mock_interviewer` 升级为任务型 Agent，而不是普通聊天 Agent。

核心执行循环：

```text
observe -> decide -> act -> persist -> evaluate -> next
```

计划亮点：
- 本地 business tools/function calling。
- 工具白名单和权限边界。
- 事务边界和幂等。
- raw output / tool trace / decision trace。
- failure injection tests。
- Agent 评测 harness。

目标表达：

```text
我把面试准备建模为可执行任务，让 Agent 在受控业务状态机中读取上下文、
调用工具、推进任务、持久化结果、处理失败，并通过评测 harness 验证行为。
```

## 18. 当前可讲的项目亮点摘要

- 真实音视频输入到 ASR 转写的异步管线。
- 长 transcript 分段 review 和 final merge。
- 候选长期记忆 + 用户确认写入机制。
- `memory_items` 与 `practice_states` 的边界设计。
- MemorySelector 的结构化 ranking 和可解释 selection_reason。
- Agent Decision Trace：保存 selected context、raw output、parsed decision、service actions 和失败信息。
- Failure injection tests 和 golden tests：证明状态机成功/失败路径不会留下半成品业务状态。
- Trace-based evaluation harness：基于 decision trace 自动检查上下文、JSON、服务端动作、失败恢复和记忆边界。
- coaching plan session 状态机和多 task 运行模型。
- mock interview execution 状态机和 practice state 事务更新。
- service-controlled 内部 BusinessTool/helper，统一 coaching/mock practice state 更新。
- completed coaching/mock 生成可追溯 memory_candidates，用户确认后才进入 memory_items。
- coaching/mock/practice state 的端到端业务闭环。
- fake runner/fake ASR 与 real build tag 分层测试。
- 真实验证暴露问题后沉淀工程硬化 backlog。
- 后续任务型 Agent 方向明确：状态机、工具调用、错误恢复、评测。
