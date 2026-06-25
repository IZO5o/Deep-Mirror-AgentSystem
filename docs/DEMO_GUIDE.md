# Demo Guide

本文档面向项目演示和本地复现。更细的 curl/API 级路径见 [DEMO_API_FLOW.md](DEMO_API_FLOW.md)。

## 1. 演示目标

推荐演示手动 transcript 主路径：

```text
create/select interview
-> upsert manual transcript
-> trigger review
-> generate memory candidates
-> 勾选并 accept selected candidates
-> generate coaching plan
-> start coaching session
-> submit coaching turn
-> start mock interview
-> submit mock answer
-> view practice_states / traces / evaluations
```

这条路径不依赖真实音视频和 ASR，但仍依赖可用 LLM 配置。

## 2. 启动准备

后端：

```bash
cp config.example.json config.json
# 填写 LLM/ASR 配置；手动 transcript demo 至少需要 LLM 可用。
go run ./main
```

前端：

```bash
cd frontend
npm install
npm run dev
```

地址：

```text
Backend:  http://127.0.0.1:8080
Frontend: http://127.0.0.1:5173
```

前端通过 Vite proxy 调用 `/api`，不需要在浏览器里配置后端地址。

## 3. 前端 Demo Console 操作顺序

### Step 1 Interview

填写或使用默认：

- `user_id`
- `company_name`
- `job_title`
- `interview_round`
- `interview_type`

点击 `Create Interview`，或点击 `Refresh Interviews` 后选择已有 interview。

选择已有 interview 后，页面会尝试自动恢复：

- transcript
- review/questions
- memory candidates/items
- coaching plan/tasks
- practice states
- traces/evaluations

### Step 2 Transcript

使用默认 manual transcript，或粘贴自己的面试文本。

点击 `Upsert Transcript`。

成功后 interview 状态应进入：

```text
ready_for_review
```

### Step 3 Review

点击 `Trigger Review`。

成功后应看到：

- review status: `generated`
- questions 数量大于 0，或至少 review report 有 summary。
- interview 状态进入 `reviewed`。

如果失败，先看页面顶部错误和 Step 7 的 failure summary。真实 LLM 配置不可用时，不要继续假设主链路已跑通。

### Step 4 Memory Confirmation

点击 `Generate Review Memory Candidates`。

候选分为三组：

- `Review Memory Candidates`
- `Coaching Session Memory Candidates`
- `Mock Interview Memory Candidates`

初次演示通常先只会有 review 来源候选。

勾选若干 pending candidates，点击：

```text
Accept Selected Candidates
```

成功后正式长期记忆写入 `memory_items`。这一步仍然逐条调用 accept API，不绕过用户确认边界。

### Step 5 Coaching

点击 `Generate Coaching Plan`。

成功后应看到：

- plan id
- tasks 数量大于 0

点击 `Start / Resume Coaching Session`。

主面板 `Coaching Current Work` 会显示：

- session status
- current task
- task description
- last agent message

在 `coaching user_input` 输入正式回答或提示请求，点击 `Submit Coaching Turn`。

成功后应看到：

- session status 变化，例如 `waiting_user_answer` 或 `needs_revision`
- attempts 数量变化
- last attempt score/feedback
- `practice_states` 更新

### Step 6 Mock Interview

点击 `Start / Resume Mock Interview`。

主面板 `Mock Current Work` 会显示：

- mock status
- current turn
- current topic
- current question

在 `mock answer` 输入正式回答或提示请求，点击 `Submit Mock Answer`。

成功后应看到：

- mock turn timeline 增加
- feedback 或 next question
- `practice_states` 更新

### Step 7 Practice / Trace / Evaluation

点击：

- `Refresh Practice States`
- `Refresh Traces`
- `Refresh Evaluations`

成功时通常能看到：

- practice states 数量大于 0
- traces 中包含 coaching plan、coaching session、mock start、mock turn
- evaluations passed 数量大于 0

`Failure / Blocking Summary` 只在有 failed review、failed coaching/mock 或 failed trace 时显示。

## 4. 什么算演示成功

最小成功标准：

- review report 生成成功。
- 至少一个 memory candidate 被 accept 成 memory item。
- coaching session 能 start，并 submit 一次 turn。
- mock interview 能 start，并 submit 一次 answer。
- practice states 可见。
- traces/evaluations 可见。

如果 coaching/mock submit 后进入 `needs_revision`，这是正常状态，不是失败。它表示 Agent 认为当前回答还需要重答或补充。

## 5. 常见阻塞

### LLM 配置不可用

表现：

- review/coaching/mock 请求返回错误。
- 页面顶部显示 API 错误。
- 对象 status 可能变为 `failed`。
- Step 7 可能出现 failed trace。

处理：

- 检查 `config.json` 的 base_url、api_key、model。
- 检查后端日志。
- 不要把该状态描述成“前后端已跑通”。

### 端口被占用

后端默认使用：

```text
:8080
```

前端默认使用：

```text
:5173
```

如果端口被占用，先确认是否已有服务正在运行。不要随意杀进程，避免中断其他演示数据。

### 已有 interview 选择后缺少 session/mock

当前前端能根据 interview 恢复常见对象，但后端没有提供按 interview 查询已有 coaching session/mock 的聚合 API。因此选择旧 interview 后，如果没有保存 session/mock id，可能需要重新 `Start / Resume`。

这不改变后端状态机：start/resume 是幂等语义，同一 active session/mock 不会重复创建。

## 6. 演示边界说明

演示时建议明确说明：

- 本项目不是通用聊天机器人。
- 当前只有 4 个业务 Agent。
- `memory_items` 必须由用户确认产生。
- `practice_states` 是训练动态状态，不是正式长期记忆。
- 当前主流程不依赖 MCP、function calling 或 ReAct。
- 真实 LLM/ASR 会带来耗时和稳定性波动。

## 7. API 级复现

需要命令行复现时，使用：

- [DEMO_API_FLOW.md](DEMO_API_FLOW.md)

该文档包含完整 endpoint、请求体和关键响应字段。
