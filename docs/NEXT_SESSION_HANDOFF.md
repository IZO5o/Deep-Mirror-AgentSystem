# 新代码会话交接文档

本文档是后续单一代码会话的主入口。新会话的目标不是一次性做完所有事情，而是按合理计划拆小任务，在用户允许后逐步实现、验证和交付。

MVP landing boundary: accepted `memory_items`, selected `practice_states`, persistent per-session state, compact history, and completed coaching/mock `memory_events` are the landed memory/context path. `memory_events` are recorded after completed artifacts but are not selected or injected into prompts.

## 1. 会话角色

你是本项目的代码执行会话。所有反馈必须使用中文。

项目目录：

```text
/Users/zhengzhan/MyProject/agent-web-base
```

你的职责：

- 先理解项目规范、当前代码和真实完成状态。
- 自己制定阶段计划，但必须在开始大阶段实现前让用户确认。
- 每次只做一个小阶段，完成后验收，再进入下一阶段。
- 可以在合适时使用 subagent 做并行审计、测试、前端体验检查或文档审阅，但主会话必须自己读关键项目规范并对最终改动负责。
- 不要把所有 Landing 工作一次性做完。

## 2. 必读文档

开始任何实现前，按顺序阅读：

- `PROJECT_PROGRAMMING_STANDARD.md`
- `PROJECT_LANDING_TODO.md`
- `docs/DEMO_API_FLOW.md`
- `COACHING_PRODUCT_USER_FLOW.md`
- `BUG_FIX_BACKLOG.md`
- `ENGINEERING_HIGHLIGHTS.md`
- `README.md`

如需理解长期任务型 Agent 背景，再读：

- `LONG_CONVERSATION_PLANNING_NOTES.md`

注意：`LONG_CONVERSATION_PLANNING_NOTES.md` 是历史规划备忘，不是当前必须实施的路线。当前不允许把 MCP、function calling 或 ReAct 提前塞进主流程。

## 3. 必读代码

后端：

- `main/main.go`
- `server/controller.go`
- `server/db.go`
- `server/interview_service.go`
- `server/interview_review_service.go`
- `server/transcript_segment_service.go`
- `server/memory_candidate_service.go`
- `server/memory_selector_service.go`
- `server/coaching_plan_service.go`
- `server/coaching_session_service.go`
- `server/mock_interview_service.go`
- `server/practice_state_service.go`
- `server/agent_decision_trace_service.go`
- `server/agent_evaluation_service.go`
- `vo/vo.go`

前端：

- `frontend/src/App.vue`
- `frontend/src/api.js`
- `frontend/src/main.js`
- `frontend/src/styles.css`
- `frontend/package.json`
- `frontend/vite.config.js` 或 `frontend/vite.config.ts`

测试：

- 与本阶段相关的 `server/*_test.go`
- 特别关注 coaching、mock、memory candidate、trace/evaluation 相关测试。

## 4. 当前完成状态

核心后端已完成到 Step 12E-2B：

- 真实音视频上传和异步 ASR。
- 手动 transcript 写入。
- 长 transcript segmented review。
- review report/questions。
- memory candidates 与 accept/reject。
- 用户确认后写入 `memory_items`。
- MemorySelector。
- coaching plan/session。
- mock interview 状态机。
- coaching/mock formal answer 更新 `practice_states`。
- completed coaching/mock 生成长期记忆候选。
- completed coaching/mock 生成事实型 `memory_events`，只作为训练时间线记录，不注入 prompt。
- agent decision traces。
- failure injection tests、golden tests。
- trace-based evaluation harness。

Landing 已完成：

- Landing-1：`docs/DEMO_API_FLOW.md`，固化 API demo path 和真实 SQLite 落库说明。
- Landing-2A：新建 Vue/Vite Demo Console，前端 `:5173`，后端 `:8080`，Vite proxy `/api -> http://127.0.0.1:8080`。

当前问题：

- Vue Demo Console 仍不够好用。
- 用户认为“顺序虽然清晰一些，但某些步骤展示仍模糊”。
- 二面辅导和模拟面试在实际页面验证中未能顺畅使用。
- memory candidates 生成入口复用较多，用户容易不知道候选来自 review、coaching session 还是 mock interview。
- 目前应优先修复 Landing-2A，而不是进入 Landing-3/4/5。

## 5. 不变项目边界

严格遵守：

- 固定 4 个业务 Agent：`review`、`memory_curator`、`second_round_coach`、`mock_interviewer`。
- 不新增业务 Agent。
- 不复活 `study_planner` 为独立业务 Agent。
- 不让 Agent 直接写 `memory_items`。
- `memory_items` 只能通过 `memory_candidates -> accept/reject -> memory_items`。
- `practice_states` 是训练动态状态，不替代长期记忆。
- 不在当前 landing 阶段引入 MCP、OpenAI function calling、ReAct。
- 不把 `memory_events` 用于 prompt 注入、LLM rerank、动态预算或 ProfileRebaser。
- 不把项目改成通用聊天机器人。
- 不为了前端方便破坏后端状态机。
- 不一次性做完所有 Landing 阶段。

## 6. 推荐工作方式

每个阶段使用以下流程：

```text
阅读上下文 -> 现状审计 -> 提出小阶段计划 -> 等用户确认 -> 实现 -> 自测 -> 总结 -> 等用户确认进入下一阶段
```

必须避免：

- 不读文档直接改代码。
- 一次性大改前端和后端。
- 为了页面体验新增大而泛的聚合 API。
- 未定位原因就重写 coaching/mock 状态机。
- 把真实 LLM 配置问题误判为前端问题。
- 把前端状态刷新问题误判为后端业务 bug。

## 7. Subagent 使用建议

可以在用户允许后开启 subagent，但必须控制范围。

适合 subagent 的任务：

- 前端体验审计：只读 `frontend/src/*`，列出 UI/状态编排问题。
- API 链路审计：按 `docs/DEMO_API_FLOW.md` 对照 `server/controller.go` 和 `vo/vo.go`，确认请求体、状态和响应字段。
- 测试审计：检查 coaching/mock/memory/trace 测试覆盖点，不改代码。
- 文档审阅：检查 README/demo guide 是否与当前代码一致。

不适合 subagent 的任务：

- 解释或改写项目根规范。
- 直接重构核心业务状态机。
- 自行决定引入新框架、Agent、MCP 或 function calling。
- 在没有主会话复核的情况下提交大范围改动。

## 8. 下一阶段：Landing-2A-Fix

优先做这个阶段，不要直接进入 README、demo script 或简历材料。

### 8.1 目标

让 Vue Demo Console 真正能作为手动 transcript demo 的主入口。

最小验收链路：

```text
create/select interview
-> upsert manual transcript
-> trigger/get review
-> generate/list review memory candidates
-> accept selected candidates
-> list memory_items
-> generate/get coaching plan
-> start/get coaching session
-> submit coaching turn
-> start/get mock interview
-> submit mock turn
-> list practice_states
-> list traces/evaluations
```

如果真实 LLM 配置不可用，页面必须明确展示后端错误，而不是表现成“按钮没反应”或“流程断了”。

### 8.2 先做诊断，不要直接改

第一小阶段只做审计和复现：

- 启动后端和前端。
- 用页面从头跑一遍 manual transcript 路径。
- 记录 coaching session 无法使用的具体表现：
  - 按钮禁用原因错误？
  - 请求体错误？
  - API 返回错误？
  - ID 没有保存？
  - 请求成功但页面未刷新？
  - 真实 LLM 输出/配置失败？
- 记录 mock interview 无法使用的具体表现，按同样维度分析。
- 检查浏览器 console、Network、后端响应和页面状态。

诊断结束后，先给用户一个小计划，等待确认再动代码。

### 8.3 可能的前端修复方向

优先考虑：

- 每个成功动作后自动刷新相关对象，例如 review 后刷新 interview/review/questions，generate plan 后刷新 plan/tasks，start session 后刷新 session。
- 当前步骤只显示 1 个主动作，其他动作放入 secondary/debug。
- 对 coaching 和 mock 单独做清楚的子流程：
  - plan 是否存在
  - task 是否存在
  - session/mock 是否已开始
  - 当前状态是否允许 submit
  - submit 后产生了哪些 turns/attempts/practice_states
- 为所有 disabled 按钮展示具体原因。
- 对 API 错误展示完整 `msg` 和必要 raw response。
- memory candidates 必须按来源分组：
  - Review Memory Candidates：`source_ref_type` 为空或 review/interview 来源。
  - Coaching Session Memory Candidates：`source_ref_type = coaching_session`。
  - Mock Interview Memory Candidates：`source_ref_type = mock_interview`。
- 从 completed coaching/mock 生成 candidates 的按钮只能在对应 source completed 后出现或可用。

暂不优先：

- 新增后端 aggregate API。
- media upload UI。
- 复杂路由。
- 登录权限。
- 大型 UI 组件库。

### 8.4 如果确实发现后端缺口

先在总结中说明，不要直接大改。

允许的小后端修复：

- 明显的 controller 请求绑定错误。
- 文档/API 不一致。
- 前端无法获取已存在业务对象的只读缺口，但必须先证明现有 API 不能完成。

不允许：

- 改 memory 写入边界。
- 改 Agent 类型。
- 重写 coaching/mock 状态机。
- 为了页面方便直接创建或更新 `memory_items`。

## 9. Landing-2A-Fix 验收标准

必须满足：

- 用户能看懂当前处于哪个 demo step。
- 页面能明确提示下一步做什么。
- coaching session 至少可以 start/get/submit 一次 turn，或明确显示真实 LLM/配置错误。
- mock interview 至少可以 start/get/submit 一次 turn，或明确显示真实 LLM/配置错误。
- memory candidates 来源展示不混乱。
- Debug 信息可见但不压过主流程。
- `npm run build` 通过。
- `go test ./...` 通过，除非只改前端且用户明确允许跳过后端测试；默认仍建议运行。

最终反馈必须包含：

- 改了哪些文件。
- 复现到的问题是什么。
- 哪些是前端问题，哪些是配置/后端问题。
- 修复了哪些体验点。
- coaching/mock 是否能用；如果不能，具体阻塞是什么。
- 测试和 build 结果。

## 10. 后续阶段概览

只有 Landing-2A-Fix 验收后，才进入后续阶段。

### Landing-3：README + Demo Guide

- 重写 README，使其适合项目展示。
- 包含项目定位、固定 4 个 Agent、主链路、启动、配置、前端、测试、demo path 和边界。

### Landing-4：稳定 Demo 数据与脚本

- 准备 demo transcript/fixture。
- 准备 API demo 脚本。
- 为真实 LLM/ASR 和 fallback demo 做分层说明。

### Landing-5：简历与面试材料

- 生成简历项目描述。
- 生成 3-5 条亮点 bullet。
- 准备 2 分钟和 8-10 分钟讲解稿。
- 准备常见追问回答。

### Landing-6：少量 Hardening

- 只修影响 demo 和主链路的关键问题。
- 候选项见 `PROJECT_LANDING_TODO.md`。

## 11. 文档状态

当前保留的核心文档：

- `PROJECT_PROGRAMMING_STANDARD.md`：最高优先级项目规范。
- `PROJECT_LANDING_TODO.md`：当前 landing 路线。
- `docs/DEMO_API_FLOW.md`：API demo path。
- `COACHING_PRODUCT_USER_FLOW.md`：coaching 产品语义。
- `BUG_FIX_BACKLOG.md`：真实验证风险与 hardening backlog。
- `ENGINEERING_HIGHLIGHTS.md`：简历/面试亮点素材。
- `LONG_CONVERSATION_PLANNING_NOTES.md`：历史规划备忘，谨慎参考。

已删除：

- `docs/DEMO_CONSOLE_DEBUG_NOTES.md`：内容已并入本文档和 `PROJECT_LANDING_TODO.md`，且旧文档曾写“Landing-2A-UX 已处理项”，与当前用户验收不一致，容易误导新会话。
