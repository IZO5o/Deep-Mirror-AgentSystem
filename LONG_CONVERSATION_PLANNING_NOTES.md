# 二面辅导与模拟面试任务型 Agent 规划备忘

本文档用于记录 Step 10A 之后，围绕 `second_round_coach` 和 `mock_interviewer` 的任务型 Agent 能力、长对话能力、工具调用、状态机、错误恢复和评测体系的待讨论事项。当前不要把这些内容直接塞进 Step 11 或已有主流程；后续应拆成独立阶段逐步设计和实现。

## 1. 当前已完成基线

当前主链路已经完成到 Step 12E-2B：

```text
文件上传/手动 transcript
-> interview_transcripts
-> 短文本直接 review / 长文本 segmented review
-> interview_questions + interview_review_reports
-> memory_curator 生成 memory_candidates
-> 用户 accept/reject
-> memory_items
-> MemorySelector 选择性注入上下文
-> second_round_coach 一次性生成 coaching_plans/coaching_tasks
-> coaching plan session 状态机推进多个 coaching_tasks
-> coaching_session_turns / coaching_task_attempts 落库
-> mock_interviewer 以 mock session 状态机推进 mock_interviews/mock_turns
-> mock formal answer 自动更新 practice_states
-> coaching formal answer 通过内部 BusinessTool/helper 更新 practice_states
-> completed coaching/mock 可生成 memory_candidates，用户确认后才写入 memory_items
-> agent_decision_traces 记录关键 Agent 执行快照
-> failure injection / golden tests 固化状态机和失败恢复
-> trace-based evaluation harness 用规则评测 trace 完整性、上下文、动作和边界
```

当前 `MemorySelector` 仍由 service 层显式调用，不开放为 Agent 工具，主要服务：
- coaching plan generation
- mock interview start
- mock interview turn / mock session submit

Step 12B 已先解决 `second_round_coach` 的 plan 级别 session、显式状态机、会话恢复、turn/attempt 落库和 parse failure 保存。Step 12C 已解决 `mock_interviewer` 的 session 状态机、start/resume、opening/follow-up/topic-switch/closing turn、cancel、failed 和 practice state 事务更新。Step 12D-1 已新增 service-controlled 内部 BusinessTool/helper，并让 coaching formal answer 也更新 practice_states。Step 12D-2 已支持 completed coaching/mock 手动生成长期记忆候选。Step 12E-1 已新增轻量 Agent Decision Trace / selected context snapshot。Step 12E-2A 已补齐 failure injection tests 和状态机 golden tests。Step 12E-2B 已新增 trace-based evaluation harness 雏形，用规则评测 trace 中的上下文、JSON、服务端动作、失败定位和记忆边界。

## 2. 任务型 Agent 目标初步定义

后续要讨论的任务型 Agent 对象只包括：
- `second_round_coach`
- `mock_interviewer`

不扩展到：
- `review`
- `memory_curator`
- 新 Agent 类型
- `study_planner`

核心调整：
- 不把后续能力定义为普通长对话。
- 要把它定义为面向求职面试准备的任务型 Agent 系统。
- Agent 必须围绕具体训练目标推进状态，而不是只基于记忆闲聊。
- 设计重点应体现 agent loop、tool calling、业务状态机、错误恢复、可观测性和评测。

任务型能力的目标：
- 用户可以围绕某一次真实面试对应的 `coaching_plan` 开启或恢复一个二面辅导长会话。
- 一个 coaching session 绑定整个 `coaching_plan_id`，在会话内推进多个 `coaching_tasks`，而不是每个 task 都开一个很短的孤立会话。
- `second_round_coach` 能读取相关上下文、判断当前应推进哪个 task、生成练习题、等待用户回答、评分、指出缺口、要求重答、对比改进、更新训练状态并判断 task/plan 是否完成。
- `mock_interviewer` 能管理一场模拟面试状态机，决定提问、追问、切换 topic、评分、记录 turn、更新 practice state、判断是否结束。
- 对话应能读取必要业务上下文，但工具调用必须受控、可测试、可恢复。
- 关键状态变化必须可追踪、可恢复、可评测。

推荐的受控执行循环：

```text
observe -> decide -> act -> persist -> evaluate -> next
```

其中：
- observe：读取任务、面试、复盘、记忆、练习状态、最近对话。
- decide：决定下一步动作，如提问、追问、提示、评分、结束任务。
- act：调用白名单本地业务工具或生成用户可见回复。
- persist：写入 mock_turn、training_attempt、practice_state、task status 等业务状态。
- evaluate：评估本轮是否达成目标、是否需要重试、是否要降级处理。
- next：进入下一轮或完成任务。

Step 12B 当前完成范围：
- 已实现 `coaching_sessions`、`coaching_session_turns`、`coaching_task_attempts`。
- 已实现 start/resume、get、submit turn、pause、cancel。
- 已实现 formal_answer / hint_request / explanation_request / skip_task / pause 的 service-driven 分支。
- 已实现 parse failure 保存 raw output/error 并让 session 进入 failed。
- 未做 MCP/native function calling/ReAct。
- Step 12D-1 后，formal_answer 已自动更新 `practice_states`。

Step 12C 当前完成范围：
- 已增强 `mock_interviews` / `mock_turns`，用 status、role、turn_type、phase、agent_action、content 和 error_message 表达长会话状态。
- 已实现 mock start/resume 幂等，start 成功写 opening assistant turn。
- 已实现 formal_answer / hint_request / explanation_request / cancel 的 service-driven 分支。
- 已实现 ask_followup / switch_topic / complete 的 assistant action turn。
- 只有 mock formal answer 更新 `practice_states`；hint/explanation/cancel/parse failure 不更新。
- formal answer turns 与 `practice_states` 更新同事务，practice update 失败时回滚本轮 turn。
- 已实现 parse/agent failure 保存 raw output/error，mock 进入 failed。
- 未做 MCP/native function calling/ReAct。

Step 12D-1 当前完成范围：
- 已新增内部 practice state update BusinessTool/helper，由 service 显式调用，不暴露给 LLM。
- helper 在事务内按 user_id、topics、score、feedback、source_type、source_id upsert `practice_states`。
- mock formal answer 复用该 helper，行为保持不变。
- coaching formal answer 创建 `coaching_task_attempt` 后，同事务更新 `practice_states`。
- coaching topic 优先来自当前 `coaching_task.title`，再退到 task_type / description，不做二次 LLM 判断。
- hint_request / explanation_request / skip_task / pause / parse failure / run failure 不更新 `practice_states`。
- 未做 MCP/native function calling/ReAct，也未做 selected context snapshot / decision trace。

Step 12D-2 当前完成范围：
- completed coaching session 可手动触发 `memory_curator` 生成 `memory_candidates`。
- completed mock interview 可手动触发 `memory_curator` 生成 `memory_candidates`。
- 新增 `source_ref_type` / `source_ref_id` 追踪候选来源。
- 同一 source_ref 已有 pending 或 accepted candidates 时直接返回已有候选，不重复调用 Agent。
- 不直接写 `memory_items`，仍必须由用户 accept。
- 未完成、failed、cancelled、paused、in_progress 状态不允许生成候选。
- 未做 MCP/native function calling/ReAct，也未做 selected context snapshot / decision trace。

Step 12E-1 当前完成范围：
- 新增 `agent_decision_traces` 表和只读查询 API。
- trace 覆盖 coaching plan generation、coaching session turn、mock start、mock turn、completed coaching/mock memory candidate generation。
- selected context snapshot 覆盖 coaching plan generation、mock start、mock turn。
- trace 保存 input snapshot、raw agent output、parsed decision、service actions、status/error。
- Agent run failure、JSON parse failure、业务持久化失败会尽量保存 failed trace。
- trace 保存失败不阻断主业务流程。
- 未做 MCP/native function calling/ReAct，也不把 trace 作为业务事实来源。

Step 12E-2A 当前完成范围：
- 新增 failure injection tests，模拟 Agent run failure、JSON parse failure、practice update failure、memory candidate generation failure。
- 新增 coaching golden tests，固定 formal answer passed/failed、hint、explanation、skip task 的状态转移。
- 新增 mock golden tests，固定 mock start、follow-up、switch topic、complete、hint、cancel 的状态转移。
- 测试断言 `agent_decision_traces` 的 status、agent_type、source、step、raw output、parsed decision、service_actions 和 selected context snapshot。
- 未做 MCP/native function calling/ReAct，也未做完整 evaluation harness。
- Step 12 完成 E2A 和后续 E2B 后应进入工程落地收尾，不继续无限扩展。

Step 12E-2B 当前完成范围：
- 新增只读 `GET /api/agent-evaluations`，复用 trace 查询参数。
- 新增 evaluation service，基于已有 `agent_decision_traces` 实时计算 report，不落库评测结果。
- 规则评测覆盖 trace 完整性、非空 JSON 字段可解析性、selected context 必要性和结构、service_actions 关键动作、failed trace 错误定位和 `memory_items` 写入边界。
- 不评自然语言质量，不做 LLM-as-judge，不调用真实 LLM。
- 未做 MCP/native function calling/ReAct，也未新增业务 Agent。
- Step 12 到 E2B 后停止扩展，后续进入工程落地收尾。

## 2.1 必须成为亮点的工程 harness

后续任务型 Agent 设计中，以下内容不是可选增强，而是核心亮点：

- 明确状态机：每个任务/会话有哪些状态、允许哪些状态转移。
- 工具白名单：Agent 只能调用明确允许的本地 business tools。
- 权限边界：读工具和写工具分开；写工具需要幂等、事务或确认策略。
- 错误恢复：LLM JSON 解析失败、tool 调用失败、状态写入失败、用户中断都要有恢复策略。
- 幂等机制：重复提交、重试同一轮、worker/请求重复不能产生重复业务结果。
- 可观测性：保存 raw output、tool calls、selected context、decision trace 或至少保存可调试快照。
- 评测 harness：评估工具调用顺序、状态更新正确性、任务完成度、错误恢复路径和最终反馈质量。

后续讲项目时，应强调：

```text
我不是只做了 prompt 拼接的聊天机器人，而是把面试准备建模成可执行任务，
让 Agent 在受控业务状态机中通过工具读取上下文、推进任务、持久化结果、
处理失败并接受自动化评测。
```

## 3. 待讨论主题

### 3.0 训练任务如何建模

需要先定义“具体任务”是什么，否则后续长对话会退化成普通聊天。

候选任务模型：
- coaching plan execution：围绕一个 `coaching_plan` 开启长会话，在会话中推进多个 `coaching_tasks`。
- coaching task execution：作为 plan session 内部的训练单元，而不是默认独立长会话入口。
- mock session execution：执行一场 `mock_interviews`。
- focused drill：针对某个 practice_state/topic 做专项训练。
- answer rewrite task：围绕某个 interview question 反复改写回答。

需要明确：
- 任务输入是什么。
- 任务完成条件是什么。
- 任务有哪些状态。
- 一个 plan 内如何选择当前 task。
- 当前 task 达标后如何推进下一个 task。
- 每轮用户输入属于正式回答、求提示、追问解释还是普通聊天。
- 哪些轮次会更新 practice_states。
- 哪些结果需要落库。

### 3.1 conversation 与业务对象如何关联

需要明确：
- 一个 conversation 是否绑定一个 interview_id。
- 一个 conversation 是否绑定一个 coaching_plan_id。
- 一个 coaching session 是否绑定一个 coaching_plan_id，并通过 current_task_id 表示当前推进的 task。
- 一个 mock interview 是否应该直接使用 conversation 作为对话载体。
- `chat_messages` 是否需要增加业务关联字段，或新增独立映射表。
- coaching chat 和 mock interview chat 是否共用一套会话模型。
- 一个任务执行器是否需要独立 task_run/session 表。
- conversation 是否只是用户交互层，还是也承担任务状态。

待评估方案：
- 方案 A：沿用现有 conversation/message，通过 metadata 关联 interview/mock/plan。
- 方案 B：为 mock interview 单独设计长对话消息表。
- 方案 C：保留业务结构化表，conversation 只做交互层。
- 方案 D：为 coaching 新增 plan 级别 `coaching_sessions`，通过 `current_task_id` 和 session turns 管理多 task 进度。

当前倾向：
- `second_round_coach` 使用 plan 级别 session，不按单个 task 默认创建独立会话。
- `coaching_task` 作为 session 内部推进和评测的训练单元。
- 用户入口是“开始/继续这次二面辅导”，而不是“打开某个孤立 task”。

### 3.2 上下文管理机制

需要明确：
- 每轮对话是否都重新运行 MemorySelector。
- selector 输入如何包含当前对话意图。
- 是否需要 conversation-level summary。
- 是否需要按轮次压缩历史消息。
- 哪些上下文稳定注入，哪些上下文动态检索。

候选上下文来源：
- interview session metadata
- interview transcript summary
- interview_review_reports
- interview_questions
- memory_items
- practice_states
- coaching_plans
- coaching_tasks
- mock_interviews
- mock_turns
- 当前 conversation history
- conversation summary

待评估方案：
- 固定上下文 + 最近 N 轮消息。
- 固定上下文 + MemorySelector 每轮选择。
- conversation summary + MemorySelector + 最近 N 轮。
- tool/function calling 按需读取业务上下文。

后续优化顺序：
1. 先保证任务执行闭环、状态机和错误恢复。
2. 再调 MemorySelector 准确性、上下文注入粒度。
3. 再讨论 token 节省、summary、压缩和缓存。

### 3.3 长对话中的长期记忆更新

需要重新明确长期记忆边界。

现有规则：
- `memory_items` 必须由 `memory_candidates` 经用户确认后写入。
- `practice_states` 可由 mock turn 自动更新。

待讨论问题：
- 用户在长对话中主动说“记住这个”时是否可以生成 memory_candidate。
- 长对话中发现新的用户弱点，是否直接生成 candidate，还是只更新 practice_state。
- 哪些内容必须用户确认。
- 是否允许某些低风险偏好类信息自动写入。
- 如何避免 mock/chat 内容污染正式长期记忆。

初步原则：
- 默认不自动写 `memory_items`。
- 可以考虑在长对话结束时生成 `memory_candidates`，仍由用户确认。
- `practice_states` 可以继续自动更新，但要限定来源、topic 和 evidence。

### 3.4 practice_states 动态更新策略

需要明确：
- coaching chat 是否也能更新 practice_states。
- mock interviewer 每一轮都更新，还是只有正式回答轮更新。
- 用户请求提示/解释时是否更新。
- answer score 缺失或解析失败时如何处理。
- topic_tags 如何规范化和合并。

当前策略：
- mock formal answer 更新 practice_states。
- coaching formal answer 更新 practice_states。
- 普通咨询、提示、解释、跳过、暂停不更新 practice_states。
- Agent run/parse failure 不更新 practice_states。

### 3.5 ReAct、agent loop 与 function calling 方案

需要讨论是否使用：
- 简单 prompt + RunStreamingWithHistory。
- ReAct 风格 agent loop。
- OpenAI native function calling。
- 本地 business tools。
- MCP 外部工具。

初步倾向：
- 先设计受控业务状态机和本地业务工具，再讨论是否通过 function calling 暴露。
- 优先使用受控 agent loop，而不是开放式 ReAct。
- 不要一开始就上 MCP。
- MCP 更适合外部系统，不适合读取本地 SQLite 业务上下文的第一选择。

待讨论方案：
- 方案 A：service 编排固定步骤，LLM 只负责结构化判断。
- 方案 B：受控 agent loop，LLM 每轮选择有限动作。
- 方案 C：native function calling，LLM 调用白名单 business tools。
- 方案 D：ReAct 风格推理文本 + 工具调用，但需要严格 trace 和错误恢复。

初步建议：
- 先做 B 或 C，不做完全开放式 ReAct。
- 工具调用结果必须进入可测试 trace。
- 每个写工具必须有幂等和失败处理。

### 3.6 可能需要的本地业务 tools

候选本地工具：
- `get_interview_context`
- `get_review_report`
- `list_interview_questions`
- `select_memory_context`
- `list_practice_states`
- `get_coaching_plan`
- `list_coaching_tasks`
- `get_coaching_session`
- `list_coaching_session_turns`
- `record_coaching_session_turn`
- `record_coaching_task_attempt`
- `update_coaching_task_status`
- `update_coaching_session_state`
- `get_mock_interview`
- `list_mock_turns`
- `record_mock_turn`
- `update_practice_state`
- `propose_memory_candidate`
- `record_training_attempt`
- `mark_training_task_done`
- `save_agent_decision_trace`

需要逐个讨论：
- 是否真的需要给 Agent 调用。
- 是否应该由 service 编排，而不是 Agent 自主调用。
- 是否需要用户确认。
- 失败时如何回滚。
- 如何测试。
- 是否需要读写权限分级。
- 是否需要 idempotency key。

### 3.7 可能需要的 MCP

MCP 暂时放在后续增强，不进入 Step 11。

候选 MCP 方向：
- 外部题库。
- 文件知识库。
- 日历/提醒。
- Notion/飞书/Google Drive。
- 公司公开资料检索。
- JD 页面抓取。

原则：
- 本地业务 DB 查询优先用本地 service/tool。
- 外部系统集成再考虑 MCP。

### 3.8 错误处理与恢复

需要讨论：
- LLM 输出 JSON 解析失败怎么办。
- tool/function call 失败怎么办。
- mock turn 生成失败是否保存 partial state。
- 用户中断对话怎么办。
- 对话恢复后如何继续。
- 重复提交同一回答如何幂等。
- practice_states 更新失败是否影响 mock_turn 保存。

待评估机制：
- raw output 持久化。
- failed status。
- retry endpoint。
- idempotency key。
- transaction boundary。
- partial failure policy。
- decision/tool trace。
- compensating action。

必须设计的失败路径：
- LLM 输出无法解析：保存 raw output，尝试修复或重试，超过次数后进入 failed。
- 工具读失败：降级回复或中止任务，不写业务结果。
- 工具写失败：事务回滚，保留 failed trace。
- 用户中断：保存 paused/cancelled 状态，可恢复。
- 重复提交：通过 idempotency key 或 turn index 防止重复写入。
- practice_state 更新失败：明确是否影响 mock_turn 保存，不能默默吞掉。

### 3.8.1 状态机候选

coaching plan session 可能状态：
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

coaching task 在 plan session 内部可能状态：
- `todo`
- `in_progress`
- `needs_revision`
- `done`
- `skipped`

mock interview execution 当前状态：
- `created`
- `in_progress`
- `waiting_answer`
- `evaluating_answer`
- `asking_followup`
- `switching_topic`
- `completed`
- `failed`
- `cancelled`

状态机要求：
- 状态转移必须显式。
- 非法状态转移要返回错误。
- 重试不能重复创建业务记录。
- 测试必须覆盖成功、失败、重试、取消路径。

### 3.9 长对话的可观测性

需要讨论是否保存：
- 每轮 selected memory snapshot。
- 每轮 tool calls。
- 每轮 prompt input summary。
- Agent decision trace。
- practice_state update reason。
- generated memory_candidate reason。
- state transition log。
- idempotency key。
- retry count。
- error category。

目标：
- 面试讲解时可解释。
- 出错时可定位。
- 不泄露不必要隐私。

### 3.10 前端/交互边界

后续可能需要：
- coaching plan 页面里的“继续辅导对话”。
- mock interview 页面里的正式回答模式/求提示模式区分。
- memory candidate 确认 UI。
- selected context debug 面板。
- practice state 变化展示。

当前阶段不做复杂 UI，只保留为后续讨论项。

### 3.11 Agent 评测 harness

Step 12E-2B 已先实现轻量 trace-based evaluation harness，而不是只靠单元测试。

评测目标：
- Agent 是否选择正确工具。
- 工具调用顺序是否合理。
- 状态转移是否正确。
- 错误发生时是否进入正确恢复路径。
- mock/coaching 反馈是否覆盖 rubric。
- practice_state 是否按规则更新。
- 是否避免未经确认写入 `memory_items`。

当前 E2B 评测范围：
- trace 完整性：succeeded trace 应有 raw output、parsed decision、service actions；failed trace 应有 error message 和调试 payload。
- JSON/schema 可解析性：parsed_decision、selected_context_snapshot、input_snapshot、service_actions 非空时必须是合法 JSON。
- selected context：coaching_plan_generate、mock_start、mock_turn 必须有 selected context，且包含 selected_memory_items、selected_practice_states、debug_summary。
- service actions：按 step_name 检查关键动作，例如 coaching plan 创建 plan/tasks、mock start 创建 mock/opening turn、mock turn 创建 turns 和按需更新 practice_states、memory candidate generation 只生成 candidates。
- memory boundary：service_actions 不能出现直接 created/updated memory_items。
- failed trace：必须能从 error_message/service_actions 定位失败阶段。

候选评测方式：
- fake runner + scripted LLM outputs。
- tool trace golden tests。
- 状态机 transition table tests。
- failure injection tests。
- real_llm build tag 小样本评测。
- rubric-based evaluation report。

评测应成为后续简历亮点之一。真实 LLM 质量评测后续可用 build tag 单独做，默认测试优先保持稳定、低成本、可重复。

## 4. 拆阶段建议

后续可以考虑按以下顺序推进：

1. Step 11：真实文件端到端验证。
2. Step 12A：selector/segment/debug 可观测性增强。
3. Step 12B：coaching plan session 状态机和多 task 运行模型设计。已完成第一版。
4. Step 12C：mock interview execution 状态机和任务运行模型设计。已完成第一版。
5. Step 12D-1：内部 BusinessTool/helper 与 coaching practice state 更新。已完成第一版。
6. Step 12D-2：session 结束后的 memory_candidates 生成策略。已完成第一版。
7. Step 12E：错误恢复、幂等、trace 和评测 harness。已完成到 E2B，Step 12 收尾。
8. 后续工程落地：API/用户流程、前端或调试页面、README/demo、真实或构造数据 demo、简历材料、少量 hardening。
9. 更远期再讨论外部 MCP/题库/文件知识库。

实际顺序以后再确认，不要直接按本文档执行。

## 5. 当前需要继续坚持的边界

- 不新增业务 Agent。
- 不把 `study_planner` 复活成独立业务 Agent。
- 不让长对话直接写 `memory_items`，除非后续重新设计并明确确认机制。
- 不让 MCP 抢先进入主流程。
- 不绕过 `memory_candidates`。
- 不破坏当前一次性生成闭环。
- 不把本项目扩散成通用聊天机器人。
- 不做没有明确任务目标、状态推进和评测标准的泛聊天功能。
- 不在没有状态机和错误恢复设计前开放写工具。
