# Deep-Mirror-AgentSystem

Deep-Mirror-AgentSystem 是一个基于 Go + Vue 的面试复盘与二面训练 Agent 系统。它不是通用聊天机器人，而是围绕真实面试输入构建的业务闭环：从 transcript 复盘、可信长期记忆、二面辅导计划、模拟面试训练，到练习状态追踪和 Agent 决策评测。

项目重点不在“接了多少模型接口”，而在把 LLM 输出放进可控的服务端状态机、可信记忆边界和可验证的工程流程里。

## 项目解决什么问题

真实面试之后，候选人的有效信息通常散落在录音、转写、个人笔记、临时反馈和模糊印象里，很难沉淀成下一轮面试可复用的训练资产。

本项目把这个过程产品化：

- 将真实面试内容转成结构化复盘报告和问题清单。
- 从复盘与训练过程中提取候选长期记忆。
- 只在用户确认后写入正式长期记忆。
- 基于长期记忆和练习状态生成二面辅导计划。
- 用服务端状态机推进 coaching session 和 mock interview。
- 将正式训练表现写入 `practice_states`，避免污染长期记忆。
- 记录 Agent 决策 trace，并用规则评测关键边界是否被破坏。

## 核心流程

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

## 核心能力

- **面试复盘链路**：支持手动 transcript、音视频上传、异步 ASR、视频提取音频、短文本直接复盘和长 transcript 分段复盘。
- **可信长期记忆**：Agent 只能生成 `memory_candidates`，正式 `memory_items` 必须经过用户 accept/reject。
- **二面辅导状态机**：生成 `coaching_plans` / `coaching_tasks`，并通过 plan 级 `coaching_sessions` 管理 submit、attempt、pause/cancel、complete 和失败恢复。
- **模拟面试状态机**：支持 opening question、formal answer、hint、explanation、follow-up、topic switch、complete、cancel 和 failed 状态。
- **练习状态追踪**：coaching/mock 的正式回答更新 `practice_states`，用于表达训练掌握度，不替代长期记忆。
- **事实型训练事件**：completed coaching/mock artifact 会生成 factual `memory_events`，用于训练时间线和审计；当前 MVP 不把这些事件注入 prompt。
- **Trace 与评测**：`agent_decision_traces` 保存关键 Agent 的输入、输出、解析结果和服务端动作；`agent-evaluations` 用规则检查状态推进、上下文选择和记忆写入边界。
- **Vue Demo Console**：提供手动 transcript 主链路、候选记忆选择、coaching/mock 状态查看和 debug 输出。

## 架构设计

当前系统固定 4 个业务 Agent：

| Agent | 职责 |
| --- | --- |
| `review` | 分析 transcript，生成复盘报告和结构化问答。 |
| `memory_curator` | 生成候选长期记忆，只写 `memory_candidates`。 |
| `second_round_coach` | 生成二面准备计划，并推进 coaching session。 |
| `mock_interviewer` | 执行模拟面试状态机。 |

长期记忆写入边界：

```text
memory_candidates -> accept/reject -> memory_items
```

Agent 不允许直接创建或更新正式 `memory_items`。`practice_states` 表达训练动态状态，不污染长期记忆。`memory_events` 只记录 completed artifact 的事实型时间线；event prompt injection、event rerank、动态 event budget 和 ProfileRebaser 仍是后续阶段能力。

## 技术栈

- 后端：Go, Gin, GORM, SQLite
- 前端：Vue 3, Vite
- AI 能力：OpenAI-compatible LLM / ASR 配置
- 媒体处理：FFmpeg
- 测试：Go tests, golden tests, failure-injection tests, trace-based evaluation checks

## 快速启动

准备配置：

```bash
cp config.example.json config.json
```

填写 `config.json` 中的 LLM/ASR 配置。手动 transcript demo 不依赖真实 ASR，但 review、memory、coaching、mock 都需要可用 LLM 配置。

启动后端：

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

启动前端 Demo Console：

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

## 推荐 Demo 路径

最稳定的演示路径是手动 transcript：

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

如果真实 LLM 配置不可用，页面会展示后端返回的错误。这代表 provider 配置或调用路径失败，不代表前端流程或后端状态机是 mock 的。

## 当前边界

- 不新增第 5 个业务 Agent。
- 不复活 `study_planner` 为独立业务 Agent。
- 不把项目改成通用聊天机器人。
- 不让 Agent 直接写正式 `memory_items`。
- 不把 `memory_events` 用于 prompt 注入、LLM rerank、动态预算或 ProfileRebaser。
- 不在当前主流程中引入 MCP、OpenAI function calling 或 ReAct。
- 不为了前端方便绕过 coaching/mock 后端状态机。

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

## 文档

- [Demo Guide](docs/DEMO_GUIDE.md)：前后端演示说明。
- [Demo API Flow](docs/DEMO_API_FLOW.md)：可复制的 API 级 demo path。
- [Next Session Handoff](docs/NEXT_SESSION_HANDOFF.md)：后续代码会话交接说明。
- [Project Programming Standard](PROJECT_PROGRAMMING_STANDARD.md)：项目边界和协作标准。
- [Engineering Highlights](ENGINEERING_HIGHLIGHTS.md)：工程亮点记录。
