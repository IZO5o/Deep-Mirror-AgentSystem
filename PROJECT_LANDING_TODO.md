# 项目工程落地待办

本文档记录 Step 12 收尾后的工程落地路线。当前项目已经完成核心后端链路，后续重点是把项目做成可运行、可演示、可写进 README 和简历的作品。

## 当前状态

已完成：

- Step 12E-2B：trace-based evaluation harness，Step 12 收尾。
- Landing-1：`docs/DEMO_API_FLOW.md`，固化 API demo path 和真实 SQLite 落库说明。
- Landing-2A：新增 Vue/Vite Demo Console，前端 `:5173` 通过 Vite proxy 调后端 `:8080`。

当前不足：

- Vue Demo Console 仍偏调试工具，演示体验不够顺滑。
- 虽然页面已有 workflow 结构，但部分步骤展示模糊。
- 二面辅导和模拟面试在用户实际验证中未能顺畅使用，需要优先诊断是前端状态编排问题、真实 LLM 配置问题，还是后端 API/状态机缺口。
- memory candidates 的来源需要在 UI 中更明确地区分 review、coaching session 和 mock interview。

## 不变边界

- 不新增业务 Agent。
- 不复活 `study_planner` 为新业务 Agent。
- 不让 Agent 直接写 `memory_items`。
- 不绕过 `memory_candidates -> accept/reject -> memory_items`。
- 不把 MCP / function calling / ReAct 提前塞进当前主流程。
- 不把项目改成通用聊天机器人。
- 不为了前端方便破坏后端状态机。
- 不一次性做完所有 landing 事项，后续必须按小阶段计划、执行、验收。

## 后续路线

### Landing-2A-Fix：修复 Vue Demo Console 演示链路

优先级：P0。

目标：

- 让用户能按页面引导完成 manual transcript 稳定 demo path。
- 诊断并修复 coaching session 和 mock interview 在页面中无法顺畅使用的问题。
- 保留 Vue Demo Console 的调试价值，但让主流程更清楚。

验收：

- 页面能清楚展示当前步骤、下一步动作和阻塞原因。
- review -> memory accept -> coaching plan/session -> mock -> practice states -> traces/evaluations 至少能在手动 transcript 路径中跑通或明确显示真实 LLM 配置错误。
- memory candidates 按来源分组，用户知道 accept 的候选来自 review、coaching session 还是 mock interview。
- `npm run build` 和 `go test ./...` 通过，或明确说明失败原因。

### Landing-3：README + Demo Guide

优先级：P1。

目标：

- README 能让他人快速理解项目定位、启动方式、演示路径和工程亮点。
- 不把 README 写成通用 Agent 框架介绍。

验收：

- README 包含项目定位、固定 4 个 Agent、主链路图、启动步骤、配置说明、前后端地址、demo path、测试命令、已知边界。
- 面试官不读源码也能看懂项目不是普通聊天机器人。

### Landing-4：稳定 Demo 数据与脚本

优先级：P1。

目标：

- 准备稳定、可重复的 demo case，降低真实 LLM/ASR 临场不确定性。

候选产出：

- 构造长 transcript fixture 或 demo transcript。
- API demo 脚本，覆盖主链路关键步骤。
- 真实 ASR 路径说明和可选样本。

验收：

- 外部依赖不可用时，仍能通过构造数据展示主要状态机和页面流程。
- 真实 LLM/ASR 路径有清楚的手动验证说明。

### Landing-5：简历与面试材料

优先级：P1。

目标：

- 把工程亮点收敛为简历和面试讲述材料。

建议突出：

- 真实音视频到 transcript/review 的业务闭环。
- 长 transcript segmented review 和 hardening。
- 用户确认式长期记忆与 practice state 分离。
- coaching/mock 任务型 Agent 状态机。
- decision trace、failure injection、golden tests、trace-based evaluation harness。

### Landing-6：少量 Hardening

优先级：P2。

只修影响 demo 或主链路稳定性的问题，例如：

- 更长 transcript 的更小 chunk fallback。
- failed coaching/mock 的恢复或 retry 策略。
- 重复提交/idempotency key。
- trace/evaluation 响应过大时分页和裁剪。
- 是否需要只读 demo-summary 聚合 API。

## 新会话入口

后续负责代码的会话应从以下交接文档开始：

- `docs/NEXT_SESSION_HANDOFF.md`

该文档包含当前代码状态、文档状态、边界、建议拆分计划、subagent 使用建议和验收要求。
