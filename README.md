# Agent Web Base

这是一个用 Go + Vue 实现的面向求职面试复盘与训练执行的多 Agent Web 项目。项目主线不是通用聊天机器人，而是把真实面试输入转化为结构化复盘、用户确认后的长期记忆、二面辅导计划、模拟面试训练和可评测的练习状态。

核心链路：

```text
真实面试输入
-> transcript
-> review report / questions
-> memory_candidates
-> 用户确认 memory_items
-> coaching plan / coaching session
-> mock interview
-> practice_states
-> completed coaching/mock memory_events
-> agent_decision_traces / evaluations
```

## 当前能力

- 手动 transcript 写入，适合作为稳定 demo 主路径。
- 音频/视频上传、异步 ASR、视频提取音频。
- 短 transcript 直接 review，长 transcript 自动分段 review。
- 生成 `interview_review_reports` 和 `interview_questions`。
- 从 review、completed coaching session、completed mock interview 生成 `memory_candidates`。
- 用户 accept/reject 后才写入正式 `memory_items`。
- 生成二面准备 `coaching_plans` / `coaching_tasks`。
- plan 级别 `coaching_sessions`，支持多轮 submit、attempt、pause/cancel 和状态恢复。
- `mock_interviews` 状态机，支持 opening question、formal answer、hint/explanation、follow-up、topic switch、complete/cancel/failed。
- coaching/mock 的正式回答更新 `practice_states`。
- completed coaching/mock artifact 会生成事实型 `memory_events`，作为训练时间线记录；当前 MVP 不把它们注入 prompt。
- `agent_decision_traces` 记录关键 Agent 执行快照。
- `agent-evaluations` 基于 trace 做规则评测。
- Vue/Vite Demo Console 支持手动 transcript 主链路、候选记忆选择、coaching/mock 会话状态查看和 debug 输出。

## 固定 4 个业务 Agent

当前只允许 4 个业务 Agent：

- `review`：分析 transcript，生成复盘报告和结构化问答。
- `memory_curator`：生成候选长期记忆，只写 `memory_candidates`。
- `second_round_coach`：生成二面准备计划，并推进 coaching session。
- `mock_interviewer`：执行模拟面试状态机。

长期记忆边界：

```text
memory_candidates -> accept/reject -> memory_items
```

Agent 不允许直接写 `memory_items`。`practice_states` 是训练动态状态，不替代正式长期记忆。

## 当前不做

- 不新增第 5 个业务 Agent。
- 不复活 `study_planner` 为独立业务 Agent。
- 不把项目改成通用聊天机器人。
- 不让 Agent 直接写 `memory_items`。
- 不把 `memory_events` 用于 prompt 注入、LLM rerank、动态预算或 ProfileRebaser。
- 不在当前主流程中引入 MCP、OpenAI function calling 或 ReAct。
- 不为了前端方便破坏 coaching/mock 后端状态机。

## 启动后端

准备配置：

```bash
cp config.example.json config.json
```

填写 `config.json` 中的 LLM/ASR 配置。手动 transcript demo 不依赖真实 ASR，但 review、memory、coaching、mock 都需要可用 LLM 配置。

启动：

```bash
go run ./main
```

后端地址：

```text
http://127.0.0.1:8080
```

真实服务会读写项目根目录下的 SQLite：

```text
agent-web-base.db
```

`mcp-server.json` 是可选配置；缺失时会打印 warning，但当前 demo 主链路不依赖 MCP。

## 启动前端 Demo Console

```bash
cd frontend
npm install
npm run dev
```

前端地址：

```text
http://127.0.0.1:5173
```

Vite dev server 会将 `/api` proxy 到：

```text
http://127.0.0.1:8080
```

Demo Console 的推荐使用路径：

```text
create/select interview
-> upsert manual transcript
-> trigger/get review
-> generate/list review memory candidates
-> 勾选并 accept selected candidates
-> list memory_items
-> generate/get coaching plan
-> start/get coaching session
-> submit coaching turn
-> start/get mock interview
-> submit mock turn
-> list practice_states
-> list traces/evaluations
```

如果真实 LLM 配置不可用，页面会展示后端返回的错误；不要把这种情况理解成前端或后端主链路已经跑通。

## 文档入口

- [Demo Guide](docs/DEMO_GUIDE.md)：面向演示者的前后端使用说明。
- [Demo API Flow](docs/DEMO_API_FLOW.md)：可复制的 API 级 demo path。
- [Next Session Handoff](docs/NEXT_SESSION_HANDOFF.md)：后续代码会话交接说明。
- [Project Programming Standard](PROJECT_PROGRAMMING_STANDARD.md)：项目边界和协作标准。
- [Engineering Highlights](ENGINEERING_HIGHLIGHTS.md)：工程亮点记录。

## 测试

普通测试不调用真实 LLM/ASR：

```bash
go test ./...
```

前端构建验证：

```bash
cd frontend
npm run build
```

真实 LLM/ASR 相关测试通过 build tag 隔离，运行前需要确认配置、成本和耗时。

## 工程亮点

- 长 transcript 分段 review，避免把超长原文一次性塞进 prompt。
- `memory_candidates` 与 `memory_items` 分离，用用户确认保护长期记忆。
- `practice_states` 单独表达训练掌握度，不污染长期记忆。
- coaching/mock 都是业务状态机，不是泛聊天。
- Agent 输出使用严格 JSON，失败时保存 raw output/error。
- failure injection tests、golden tests 和 trace-based evaluation harness 覆盖关键状态转移和边界。
