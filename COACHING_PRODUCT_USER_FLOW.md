# 二面辅导产品用户视角

本文档从用户和前端产品视角描述“真实面试复盘 -> 二面辅导”的体验目标，供后续页面设计、API 设计和任务型 Agent 实现参考。它不替代 `PROJECT_PROGRAMMING_STANDARD.md`，所有实现仍必须遵守项目编程标准和记忆边界。

## 1. 一句话产品描述

用户上传一次真实面试录音/录屏后，系统先生成面试复盘报告，再基于复盘结果、已确认长期记忆和练习状态生成该次面试对应的二面辅导计划；用户可以围绕这个计划开启一个可暂停、可恢复、可推进多个任务的二面辅导会话。

## 2. 用户主流程

```text
上传面试录音/录屏
-> ASR 转写
-> transcript
-> review report/questions
-> memory candidates
-> 用户确认长期记忆
-> 自动或手动生成 coaching plan/tasks
-> 用户进入 plan 级别二面辅导会话
-> Agent 按 task 推进训练
-> 用户回答/求提示/重答/完成任务
-> practice_states 更新
-> 会话暂停或完成
```

## 3. 面试详情页用户视角

一次真实面试完成复盘后，面试详情页应能看到：

- transcript 状态。
- review report。
- interview questions。
- memory candidates 及确认入口。
- coaching plan 状态。
- coaching tasks。
- “开始二面辅导”或“继续二面辅导”入口。
- mock interview 入口。
- practice state 变化摘要。

其中“开始二面辅导”不是开启泛聊天，而是进入该 interview 对应的 coaching plan 执行会话。

## 4. Coaching Plan 的定位

`coaching_plan` 是针对一次真实面试的二面准备方案。

生成时应结合：
- 当前 interview 的 review report。
- 当前 interview 的 interview questions。
- 用户已确认的 memory_items。
- 当前 practice_states。
- company_name / job_title / target_round。

生成时机可以先支持手动触发；后续可以在 review report 生成后由后台自动生成。即使自动生成，也必须保持幂等，避免重复创建多个无意义 plan。

一个真实 interview 可以允许存在若干个 coaching plan，但早期建议先控制为“默认 active plan + 历史 plan”，避免产品和状态复杂度过早上升。

## 5. Coaching Session 的定位

二面辅导长会话应绑定 `coaching_plan_id`，而不是只绑定单个 `coaching_task_id`。

原因：
- 一个 task 粒度太小，往往只有几轮交互，难以体现上下文管理、状态恢复和任务推进。
- 一个 plan 通常包含多个 task，更适合承载连续辅导过程。
- 用户的真实心智是“继续这次二面准备”，不是“打开某一个孤立任务”。

会话内 Agent 应能：
- 读取当前 plan 和所有 tasks。
- 判断当前应推进哪个 task。
- 对用户发起练习问题。
- 区分用户输入是正式回答、求提示、要求解释、跳过还是结束。
- 对正式回答评分和反馈。
- 要求用户重答或补充。
- 达标后标记当前 task 完成，并推进到下一个 task。
- 中途暂停，后续恢复到正确任务和状态。

当前 Step 12B 已实现最小可用版本：
- `coaching_sessions` 绑定整个 `coaching_plan_id`。
- session 保存 `current_task_id`、状态、进度摘要、最后 Agent 消息和错误信息。
- 同一个 plan 的 active session 支持 start/resume 幂等，不重复创建多个会话。
- session 内通过 `coaching_session_turns` 记录 user/assistant/system 轮次。
- 正式回答通过 `coaching_task_attempts` 记录分数、反馈、是否达标和 attempt index。
- 当前仍由 service 层驱动状态转移，不做 function calling/ReAct。

Step 12D-1 已补齐：
- coaching session 的正式训练回答会自动更新 `practice_states`。
- 只有 formal answer 更新；求提示、解释、跳过、暂停、Agent 失败或解析失败不更新。
- 更新由 service-controlled 内部 BusinessTool/helper 执行，不是 Agent 自由调用工具。

Step 12D-2 已补齐：
- completed coaching session 可以手动触发生成 `memory_candidates`。
- 生成的是候选长期记忆，不直接写入 `memory_items`。
- 候选带有 `source_ref_type = coaching_session` 和 `source_ref_id = session_id`。
- 重复触发会返回已有 pending/accepted candidates，不重复生成。

Step 12E-1 已补齐：
- 关键 Agent 执行会写入 `agent_decision_traces`，用于调试和后续评测。
- coaching plan generation 会保存 selected context snapshot，能复盘当时 MemorySelector 选中了哪些长期记忆和练习状态。
- coaching session turn 会保存输入摘要、Agent 原始输出、parsed decision 和 service actions。
- mock start / mock turn 会保存 selected context snapshot，能复盘模拟面试当时读取了哪些上下文。
- trace 只读，不改变用户侧业务状态，也不替代 turns、attempts、practice_states 或 memory_candidates。

Step 12E-2A 已补齐：
- failure injection tests 会故意模拟 Agent error、非法 JSON、practice update failure 和 memory candidate generation failure。
- golden tests 用固定 Agent JSON 锁定 coaching/mock 的标准状态转移。
- 测试会断言 trace 中的 service_actions、status/error 和 selected context snapshot，确保 trace 可用于后续评测。
- 本阶段不改变用户 API、业务状态机和记忆写入边界。

Step 12E-2B 已补齐：
- 新增 `GET /api/agent-evaluations` 只读评测 API，基于已有 `agent_decision_traces` 实时计算规则评测结果。
- evaluation harness 不评价自然语言是否优美，而是检查任务型 Agent 的工程行为：trace 完整性、JSON 可解析性、selected context 结构、service_actions 是否符合状态机动作、failed trace 是否可定位错误、是否违反 `memory_items` 写入边界。
- 本阶段不新增业务 Agent，不调用真实 LLM，不做 MCP/function calling/ReAct，不改变 coaching/mock 状态机。
- Step 12 到 E2B 后收尾，后续优先做 API/用户流程梳理、前端或调试页面、README/demo、真实或构造数据 demo、简历材料和少量 hardening。

## 6. Coaching Task 的用户体验

`coaching_task` 是 plan 内的训练单元，不建议直接作为独立长会话入口。

用户在会话中会感知到 task，例如：
- “我们先补齐 Redis 缓存一致性的回答。”
- “这一项已经达标，接下来练习项目复盘中的 trade-off 表达。”
- “这个任务还没达标，我会给你一个更具体的追问。”

task 应有明确完成条件，例如：
- 用户回答覆盖必要要点。
- 至少完成 N 次正式回答。
- 最近一次评分达到阈值。
- Agent 判断不再需要立即重练。

## 7. 会话状态建议

当前已新增 plan 级别会话对象 `coaching_sessions`。

候选状态：
- `created`：会话已创建，尚未开始正式辅导。
- `in_progress`：正在执行 plan。
- `waiting_user_answer`：Agent 已提出练习问题，等待用户回答。
- `evaluating`：正在评价用户回答。
- `needs_revision`：当前 task 未达标，需要用户重答或补充。
- `task_completed`：当前 task 刚完成，准备推进下一个 task。
- `paused`：用户中途暂停，可恢复。
- `completed`：plan 内任务已完成或达到本轮辅导目标。
- `failed`：系统错误导致会话无法继续。
- `cancelled`：用户主动取消。

状态转移必须显式、可测试、可恢复。

## 8. 需要落库的内容

Step 12D-1 当前落库方案：

- `coaching_sessions`：plan 级别长会话，记录当前状态、current_task_id、进度、失败原因、最后活动时间。
- `coaching_session_turns`：用户与 Agent 的多轮交互，记录 role、message、turn_type、agent_action、score、feedback、raw output/error。
- `coaching_task_attempts`：针对某个 task 的正式回答尝试，记录 answer、rubric score、feedback、是否达标。
- `practice_states`：仅由正式训练回答更新，source_type 为 `coaching_task_attempt`，source_id 为本次 attempt_id。
- `memory_candidates`：completed coaching session 可手动生成候选，source_ref 指向 session。
- `agent_decision_traces`：关键 Agent 执行快照，记录 selected context、输入摘要、raw output、parsed decision、service actions 和错误。
- `agent_evaluations`：不落库；通过只读 API 基于 trace 实时计算评测报告。

后续还可以讨论：
- 更完整的 `coaching_session_events` 或 evaluation trace：记录状态转移、工具调用、retry、error category，便于调试和评测。

## 9. 记忆和练习状态边界

二面辅导长会话可以自动更新 `practice_states`，但不能直接写入 `memory_items`。

当前 Step 12D-1 已自动更新 `practice_states`：只让正式训练回答更新 practice state，提示/解释/跳过/暂停类输入不更新。
当前 Step 12D-2 已支持 completed coaching/mock session 生成长期记忆候选，但仍必须由用户 accept 后才进入 `memory_items`。
当前 Step 12E-1 已支持 Agent decision trace，但 trace 不是记忆来源，也不会写入 `memory_items`。
当前 Step 12E-2A 已用 failure/golden tests 固化这些边界，确保失败路径不会产生半成品长期记忆或练习状态。
当前 Step 12E-2B 已用 trace-based evaluation harness 把这些边界变成可自动检查的规则，尤其检查 service_actions 中不能出现直接 created/updated `memory_items`。

建议规则：
- 正式训练回答可以更新 `practice_states`。
- 用户求提示、闲聊、解释性问题默认不更新 `practice_states`。
- 如果会话中发现新的长期画像信息，只能生成 `memory_candidates`，仍需用户确认。
- 所有长期记忆写入继续遵守：

```text
memory_candidates -> 用户 accept -> memory_items
```

## 10. 工具能力产品视角

后续二面辅导 Agent 需要的本地 business tools 可以从产品动作倒推：

- 读取当前 interview 的 review report。
- 读取 interview questions。
- 读取当前 coaching plan 和 tasks。
- 运行 MemorySelector 获取相关长期记忆和练习状态。
- 读取最近会话历史和当前 task 进度。
- 保存用户回答和 Agent 反馈。
- 更新当前 task 状态。
- 更新 practice_states。
- 保存错误、重试和状态转移 trace。

这些优先作为本地 service/business tools，不急于做 MCP。MCP 后续更适合外部题库、文件知识库、日历提醒或第三方资料接入。

## 11. 前端页面启发

后续前端可以围绕三个层级设计：

- Interview Detail：展示 transcript/review/questions/memory candidates/coaching plan/mock 入口。
- Coaching Plan：展示 plan 总览、task 列表、每个 task 的状态和最近反馈。
- Coaching Session：一个持续对话界面，左侧或顶部展示当前 task、进度、状态、是否达标。

用户主要操作：
- 开始/继续辅导。
- 正式回答。
- 求提示。
- 要求解释。
- 重答。
- 跳过当前 task。
- 暂停会话。
- 查看 task 完成情况和练习状态变化。

## 12. 当前明确不做

- 不把二面辅导做成无目标泛聊天。
- 不为每个 task 默认创建一个独立短会话。
- 不让 Agent 直接写 `memory_items`。
- 不在没有状态机和错误恢复前开放写工具。
- 不把 MCP 提前塞进本地业务上下文读取。
- 不新增第 5 个业务 Agent。
