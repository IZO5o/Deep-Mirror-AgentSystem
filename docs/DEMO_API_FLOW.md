# Demo API Flow

本文档用于 Landing-1：固化一条可复制、可演示、可真实落库的 API 链路。

如果要通过 Vue Demo Console 做前端演示，先读 [DEMO_GUIDE.md](DEMO_GUIDE.md)；本文档保留 curl/API 级复现细节。

## 1. 项目定位与边界

本项目是一个 Go 实现的面向求职面试复盘与训练执行的多 Agent Web 系统，主线是：

```text
真实面试输入 -> transcript -> review -> memory_candidates -> 用户确认 memory_items -> coaching -> mock interview -> practice_states -> trace/evaluation
```

当前固定 4 个业务 Agent：

- `review`：生成面试复盘报告和结构化问答。
- `memory_curator`：只生成 `memory_candidates`，不直接写正式长期记忆。
- `second_round_coach`：生成二面准备计划和推进 coaching session。
- `mock_interviewer`：执行模拟面试状态机。

长期记忆边界必须保持不变：

```text
memory_candidates -> accept/reject -> memory_items
```

Agent 不允许直接写 `memory_items`。`practice_states` 是训练过程动态状态，可由 coaching/mock 的正式回答自动更新，但不替代正式长期记忆。

本阶段不引入第 5 个业务 Agent，不复活 `study_planner` 为新业务 Agent，不把 MCP、OpenAI function calling 或 ReAct 写成当前主业务流程能力。

## 2. 真实服务数据库与配置

### 2.1 持久化数据库

真实服务入口在 `main/main.go`，当前使用：

```go
server.InitDB("agent-web-base.db")
```

因此用 `go run ./main` 启动的真实服务会读写项目根目录的 SQLite 文件：

```text
/Users/zhengzhan/MyProject/agent-web-base/agent-web-base.db
```

`server/db.go` 使用 GORM `AutoMigrate` 初始化业务表，包括：

- `interview_sessions`
- `interview_transcripts`
- `transcript_segments`
- `media_files`
- `transcription_jobs`
- `interview_questions`
- `interview_review_reports`
- `memory_candidates`
- `memory_items`
- `coaching_plans`
- `coaching_tasks`
- `coaching_sessions`
- `coaching_session_turns`
- `coaching_task_attempts`
- `mock_interviews`
- `mock_turns`
- `practice_states`
- `agent_decision_traces`

测试和真实服务不要混用数据库。普通测试通常通过 `t.TempDir()` 创建临时 `test.db`，不会写项目根目录的 `agent-web-base.db`。少量 real build tag 测试会显式说明是否使用真实 DB。

验证真实服务已落库的方式：

1. 启动服务并按 demo path 创建一条 interview。
2. 停止服务后重新启动。
3. 再次查询同一个 `interview_id`：

```bash
curl -sS "$API/interviews/$INTERVIEW_ID"
```

如果仍能查到同一条记录，说明数据来自持久化 SQLite 文件。

也可以直接用 sqlite 工具查看：

```bash
sqlite3 agent-web-base.db ".tables"
sqlite3 agent-web-base.db "select interview_id,user_id,status,company_name from interview_sessions order by created_at desc limit 5;"
sqlite3 agent-web-base.db "select count(*) from agent_decision_traces;"
```

### 2.2 启动前配置检查

真实服务可以通过两种方式读取配置：

- 推荐方式：准备 `config.json`，显式写清 LLM/ASR/media 配置。
- 兼容方式：缺少 `config.json` 时，`shared.LoadAppConfig("config.json")` 会回退读取环境变量，例如 `OPENAI_BASE_URL`、`OPENAI_API_KEY`、`OPENAI_MODEL`、`ASR_BASE_URL`、`ASR_API_KEY`、`ASR_MODEL`。

为了 demo 可复制，建议仍然准备 `config.json`：

准备配置：

```bash
cp config.example.json config.json
```

然后填写：

- `llm_providers.front_model.api_key`
- `llm_providers.back_model.api_key`
- `asr.api_key`
- 如使用非默认服务，填写对应 `base_url` 和 `model`
- 如需要真实视频转音频，确认 `media.ffmpeg_path` 指向可执行的 `ffmpeg`
- 如希望媒体文件落到固定目录，设置 `media.storage_dir`；为空时默认使用工作区下 `.agent-web-base/media`

程序会执行 `godotenv.Load()`，因此 `.env` 中的环境变量可以作为缺省配置来源；如果 `config.json` 中某些 ASR/media 字段为空，代码也会从环境变量补齐。为减少演示时的隐式依赖，建议把最终 demo 使用的 LLM/ASR 配置写入 `config.json`。

`mcp-server.json` 是可选项；缺失时只会打印 warning，不影响本 demo 主业务 API。当前 demo 不依赖 MCP。

启动服务：

```bash
go run ./main
```

API 基础地址：

```bash
export API=http://127.0.0.1:8080/api
export USER_ID=demo_user_001
```

Landing-2A 已新增 Vue Demo Console 辅助手动 transcript demo path。前端地址为 `http://127.0.0.1:5173`，Vite dev server 会通过 proxy 将 `/api` 转发到 `http://127.0.0.1:8080`。Landing-2A-Fix 后，console 已支持主流程引导、候选记忆来源分组、勾选并 accept 候选、coaching/mock 会话状态展示，以及 practice/trace/evaluation 调试查看。

## 3. 手动 Transcript Demo Path

这条路径不依赖真实音视频文件和 ASR，最适合作为 README/demo 的稳定主链路。仍会调用真实 LLM Agent，因此需要 `config.json` 中 LLM 配置可用。

所有响应统一包在：

```json
{"code":0,"msg":"ok","data":{}}
```

下面示例使用 `jq` 提取 ID；也可以手动从响应 `data` 中复制。

### 3.1 Create Interview

Endpoint:

```text
POST /api/interviews
```

示例：

```bash
CREATE_RESP=$(curl -sS -X POST "$API/interviews" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"demo_user_001",
    "company_name":"Acme AI",
    "job_title":"Backend Engineer",
    "interview_round":"first_round",
    "interview_type":"technical",
    "occurred_at":0
  }')

export INTERVIEW_ID=$(echo "$CREATE_RESP" | jq -r '.data.interview_id')
```

关键响应字段：

- `interview_id`
- `user_id`
- `company_name`
- `status`

下一步条件：`status = created`。

### 3.2 Upsert Transcript

Endpoint:

```text
PUT /api/interviews/:interview_id/transcript
```

示例：

```bash
curl -sS -X PUT "$API/interviews/$INTERVIEW_ID/transcript" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"demo_user_001",
    "source_type":"manual_text",
    "language":"zh",
    "content":"面试官：请介绍你在 Go 项目中做过的 Agent 系统。候选人：我做了一个面向面试复盘和二面训练的多 Agent Web 项目，包含 transcript、review、memory candidate、coaching plan 和 mock interview。面试官：你如何保证长期记忆不会被模型污染？候选人：正式长期记忆必须先进入 memory_candidates，用户 accept 后才写入 memory_items。面试官：如果用户回答不好，系统如何处理？候选人：coaching 和 mock 都有状态机，正式回答会记录 attempt 或 turn，并更新 practice_states。"
  }'
```

关键响应字段：

- `transcript_id`
- `source_type`
- `language`
- `content`

下一步条件：重新查询 interview，`status = ready_for_review`。

```bash
curl -sS "$API/interviews/$INTERVIEW_ID"
```

### 3.3 Trigger Review

Endpoint:

```text
POST /api/interviews/:interview_id/review
```

示例：

```bash
curl -sS -X POST "$API/interviews/$INTERVIEW_ID/review"
```

关键响应字段：

- `report_id`
- `status`
- `overall_summary`
- `strengths`
- `weaknesses`
- `follow_up_risks`
- `suggested_preparation`

下一步条件：

- 成功：`status = generated`，interview 后续会变为 `reviewed`。
- 失败：`status = failed`，查看 `raw_agent_output` 或服务日志。

### 3.4 Get Review / Questions / Transcript Segments

Endpoints:

```text
GET /api/interviews/:interview_id/review
GET /api/interviews/:interview_id/questions
GET /api/interviews/:interview_id/transcript-segments
```

示例：

```bash
curl -sS "$API/interviews/$INTERVIEW_ID/review"
curl -sS "$API/interviews/$INTERVIEW_ID/questions"
curl -sS "$API/interviews/$INTERVIEW_ID/transcript-segments"
```

关键响应字段：

- review: `status`, `overall_summary`, `weaknesses`
- questions: `question_id`, `sequence`, `question`, `answer`, `topic_tags`, `weakness_summary`
- transcript segments: `sequence`, `status`, `summary`, `content_preview`, `error_message`

下一步条件：review report 已 `generated`，interview 已 `reviewed`。

短 transcript 通常不会生成 segment；长 transcript 触发 segmented review 后，segment 状态应为 `extracted` 或 `failed`。

### 3.5 Generate Memory Candidates

Endpoint:

```text
POST /api/interviews/:interview_id/memory-candidates
```

示例：

```bash
curl -sS -X POST "$API/interviews/$INTERVIEW_ID/memory-candidates"
```

关键响应字段：

- `candidate_id`
- `memory_type`
- `content`
- `evidence`
- `confidence`
- `status`
- `source`

下一步条件：至少有需要保留的候选，`status = pending`。

如果 review 未完成，服务会拒绝生成。

### 3.6 Accept / Reject Candidates

Endpoints:

```text
POST /api/memory-candidates/:candidate_id/accept
POST /api/memory-candidates/:candidate_id/reject
```

示例：

```bash
export CANDIDATE_ID=$(curl -sS "$API/interviews/$INTERVIEW_ID/memory-candidates" | jq -r '.data[0].candidate_id')

curl -sS -X POST "$API/memory-candidates/$CANDIDATE_ID/accept"
```

关键响应字段：

- accept 返回 `memory_item` 视图：`memory_id`, `source_candidate_id`, `status = active`
- reject 返回 candidate：`status = rejected`

下一步条件：至少一个 candidate 被 accept，正式长期记忆才进入 `memory_items`。

### 3.7 List Memory Items

Endpoint:

```text
GET /api/memory-items?user_id=:user_id
```

示例：

```bash
curl -sS "$API/memory-items?user_id=$USER_ID"
```

关键响应字段：

- `memory_id`
- `memory_type`
- `content`
- `confidence`
- `source_candidate_id`
- `status = active`

下一步条件：可以为空，但有 accepted memory 时，后续 MemorySelector 更容易展示效果。

### 3.8 Generate Coaching Plan

Endpoint:

```text
POST /api/interviews/:interview_id/coaching-plan
```

示例：

```bash
PLAN_RESP=$(curl -sS -X POST "$API/interviews/$INTERVIEW_ID/coaching-plan" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"demo_user_001",
    "target_round":"second_round",
    "remaining_days":3
  }')

export PLAN_ID=$(echo "$PLAN_RESP" | jq -r '.data.plan_id')
```

关键响应字段：

- `plan_id`
- `target_round`
- `overall_strategy`
- `focus_summary`
- `status`

下一步条件：`status = generated`。

该步骤会写 `agent_decision_traces`，其中包括 selected context snapshot。

### 3.9 List Coaching Tasks

Endpoint:

```text
GET /api/coaching-plans/:plan_id/tasks
```

示例：

```bash
curl -sS "$API/coaching-plans/$PLAN_ID/tasks"
```

关键响应字段：

- `task_id`
- `sequence`
- `task_type`
- `title`
- `description`
- `priority`
- `status`

下一步条件：至少有一个 `todo` 或 `in_progress` task。

### 3.10 Start / Resume Coaching Session

Endpoint:

```text
POST /api/coaching-plans/:plan_id/sessions?user_id=:user_id
```

示例：

```bash
SESSION_RESP=$(curl -sS -X POST "$API/coaching-plans/$PLAN_ID/sessions?user_id=$USER_ID")
export SESSION_ID=$(echo "$SESSION_RESP" | jq -r '.data.session.session_id')
```

关键响应字段：

- `session.session_id`
- `session.status`
- `session.current_task_id`
- `session.last_agent_message`
- `current_task`
- `tasks`
- `turns`
- `attempts`

下一步条件：通常为 `session.status = waiting_user_answer`。

重复调用是 start/resume 语义，同一个 active session 不会重复创建。

### 3.11 Submit Coaching Formal Answer / Hint / Explanation

Endpoint:

```text
POST /api/coaching-sessions/:session_id/turns
```

正式回答示例：

```bash
curl -sS -X POST "$API/coaching-sessions/$SESSION_ID/turns" \
  -H "Content-Type: application/json" \
  -d '{
    "user_input":"正式回答：我会先说明系统目标，再解释 transcript 到 review 的链路，最后强调 memory_candidates 到 memory_items 的用户确认边界，以及 coaching/mock 如何通过状态机更新 practice_states。"
  }'
```

提示请求示例：

```bash
curl -sS -X POST "$API/coaching-sessions/$SESSION_ID/turns" \
  -H "Content-Type: application/json" \
  -d '{"user_input":"给我一个提示"}'
```

解释请求示例：

```bash
curl -sS -X POST "$API/coaching-sessions/$SESSION_ID/turns" \
  -H "Content-Type: application/json" \
  -d '{"user_input":"解释一下这题应该怎么答"}'
```

关键响应字段：

- `session.status`
- `session.progress_summary`
- `session.last_agent_message`
- `turns[].turn_type`
- `turns[].agent_action`
- `attempts[].score`
- `attempts[].passed`

下一步条件：

- 继续训练：`waiting_user_answer` 或 `needs_revision`
- 本 plan 完成：`completed`
- 异常：`failed`

只有 formal answer 会写 `coaching_task_attempts` 并更新 `practice_states`。hint/explanation/pause/skip/parse failure 不更新 `practice_states`。

### 3.12 Get Coaching Session Detail

Endpoint:

```text
GET /api/coaching-sessions/:session_id
```

示例：

```bash
curl -sS "$API/coaching-sessions/$SESSION_ID"
```

关键响应字段同 start/resume。

可选暂停或取消：

```bash
curl -sS -X POST "$API/coaching-sessions/$SESSION_ID/pause"
curl -sS -X POST "$API/coaching-sessions/$SESSION_ID/cancel"
```

如果需要从 completed session 生成长期记忆候选，必须先让 session 进入 `completed`。

### 3.13 List Practice States

Endpoint:

```text
GET /api/practice-states?user_id=:user_id
```

示例：

```bash
curl -sS "$API/practice-states?user_id=$USER_ID"
```

可选过滤：

```text
topic=:topic
dimension=:dimension
```

关键响应字段：

- `state_id`
- `topic`
- `dimension`
- `mastery_score`
- `attempt_count`
- `last_score`
- `last_feedback`
- `source_type`
- `source_id`

下一步条件：formal answer 执行后通常会出现或更新 practice state。

### 3.14 Start / Resume Mock Interview

Endpoint:

```text
POST /api/interviews/:interview_id/mock-interviews
```

示例：

```bash
MOCK_RESP=$(curl -sS -X POST "$API/interviews/$INTERVIEW_ID/mock-interviews" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\":\"$USER_ID\",
    \"plan_id\":\"$PLAN_ID\",
    \"target_round\":\"second_round\"
  }")

export MOCK_ID=$(echo "$MOCK_RESP" | jq -r '.data.mock_id')
```

关键响应字段：

- `mock_id`
- `status`
- `overall_goal`
- `first_question`
- `current_topic`

下一步条件：`status = waiting_answer`。

重复调用是 start/resume 语义，同一 active mock 不会重复创建。

### 3.15 List Mock Turns

Endpoint:

```text
GET /api/mock-interviews/:mock_id/turns
```

示例：

```bash
curl -sS "$API/mock-interviews/$MOCK_ID/turns"
```

关键响应字段：

- `turn_id`
- `turn_index`
- `role`
- `turn_type`
- `phase`
- `agent_action`
- `content`
- `interviewer_question`
- `feedback`
- `score`
- `next_question`

### 3.16 Submit Mock Answer / Hint / Explanation / Cancel / Complete

Endpoint:

```text
POST /api/mock-interviews/:mock_id/turns
```

正式回答示例：

```bash
curl -sS -X POST "$API/mock-interviews/$MOCK_ID/turns" \
  -H "Content-Type: application/json" \
  -d '{
    "answer":"正式回答：我会从业务状态机、长期记忆确认边界、失败落库和 trace-based evaluation 四个方面介绍这个项目。"
  }'
```

提示请求示例：

```bash
curl -sS -X POST "$API/mock-interviews/$MOCK_ID/turns" \
  -H "Content-Type: application/json" \
  -d '{"answer":"给我一个提示"}'
```

解释请求示例：

```bash
curl -sS -X POST "$API/mock-interviews/$MOCK_ID/turns" \
  -H "Content-Type: application/json" \
  -d '{"answer":"解释一下这题的考察点"}'
```

手动完成：

```bash
curl -sS -X POST "$API/mock-interviews/$MOCK_ID/complete"
```

取消：

```bash
curl -sS -X POST "$API/mock-interviews/$MOCK_ID/cancel"
```

关键响应字段：

- turn submit 返回 `MockTurnVO`
- complete/cancel 返回 `MockInterviewVO`
- `status` 可通过 `GET /api/mock-interviews/:mock_id` 查询

下一步条件：

- mock 继续：`waiting_answer`
- 追问中：`asking_followup`
- 切 topic：`switching_topic`
- 完成：`completed`
- 异常：`failed`
- 取消：`cancelled`

只有 mock formal answer 更新 `practice_states`。hint/explanation/cancel/parse failure 不更新。

### 3.17 List Practice States Again

Endpoint:

```text
GET /api/practice-states?user_id=:user_id
```

示例：

```bash
curl -sS "$API/practice-states?user_id=$USER_ID"
```

对比 coaching/mock formal answer 前后的 `attempt_count`、`last_score`、`last_feedback`、`source_type`。

### 3.18 Generate Memory Candidates From Completed Coaching / Mock

Completed coaching session:

```text
POST /api/coaching-sessions/:session_id/memory-candidates
```

示例：

```bash
curl -sS -X POST "$API/coaching-sessions/$SESSION_ID/memory-candidates"
```

Completed mock interview:

```text
POST /api/mock-interviews/:mock_id/memory-candidates
```

示例：

```bash
curl -sS -X POST "$API/mock-interviews/$MOCK_ID/memory-candidates"
```

关键响应字段：

- `candidate_id`
- `status = pending`
- `source_ref_type = coaching_session` 或 `mock_interview`
- `source_ref_id = session_id` 或 `mock_id`

下一步条件：source 对象必须先是 `completed`。未完成、failed、cancelled、paused、in_progress 状态不允许生成。

重复触发同一 source 的候选生成会复用已有 pending/accepted candidates，不重复调用 Agent。

### 3.19 Accept Selected Candidates Again

Endpoint:

```text
POST /api/memory-candidates/:candidate_id/accept
```

示例：

```bash
curl -sS -X POST "$API/memory-candidates/$CANDIDATE_ID/accept"
curl -sS "$API/memory-items?user_id=$USER_ID"
```

下一步条件：新的 accepted candidate 会写入 `memory_items`，状态为 `active`。

### 3.20 List Agent Decision Traces

Endpoint:

```text
GET /api/agent-decision-traces
```

示例：

```bash
curl -sS "$API/agent-decision-traces?user_id=$USER_ID&interview_id=$INTERVIEW_ID&limit=20"
```

可选过滤：

```text
user_id
interview_id
source_type
source_id
agent_type
step_name
status
limit
```

关键响应字段：

- `trace_id`
- `agent_type`
- `source_type`
- `source_id`
- `step_name`
- `selected_context_snapshot`
- `input_snapshot`
- `raw_agent_output`
- `parsed_decision`
- `service_actions`
- `status`
- `error_message`

下一步条件：执行过 coaching plan、coaching session turn、mock start、mock turn 或 completed session/mock candidate generation 后，应能看到 trace。

### 3.21 List Agent Evaluations

Endpoint:

```text
GET /api/agent-evaluations
```

示例：

```bash
curl -sS "$API/agent-evaluations?user_id=$USER_ID&interview_id=$INTERVIEW_ID&limit=20"
```

过滤参数与 traces API 相同。

关键响应字段：

- `total_traces`
- `passed_traces`
- `failed_traces`
- `results[].score`
- `results[].checks`

评测结果不落库；它实时读取 `agent_decision_traces` 并做规则检查。

## 4. 真实 Media / ASR Demo Path

这条路径用于展示真实音视频上传、异步转写和落库。它依赖 `config.json` 中 ASR 配置可用，视频文件还依赖 ffmpeg 可用。真实 ASR/LLM 有成本、耗时和外部服务失败风险。

### 4.1 Create Interview

同手动路径：

```bash
CREATE_RESP=$(curl -sS -X POST "$API/interviews" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"demo_user_001",
    "company_name":"Acme AI",
    "job_title":"Backend Engineer",
    "interview_round":"first_round",
    "interview_type":"technical"
  }')

export INTERVIEW_ID=$(echo "$CREATE_RESP" | jq -r '.data.interview_id')
```

下一步条件：`status = created`。

### 4.2 Upload Media

Endpoint:

```text
POST /api/interviews/:interview_id/media
```

请求类型：`multipart/form-data`

字段：

- `file`：音频或视频文件
- `user_id`：必须匹配 interview user
- `language`：可选，默认 `zh`

示例：

```bash
UPLOAD_RESP=$(curl -sS -X POST "$API/interviews/$INTERVIEW_ID/media" \
  -F "user_id=$USER_ID" \
  -F "language=zh" \
  -F "file=@ahq_part2_chinese_dialogue_1of10.mp3")

export JOB_ID=$(echo "$UPLOAD_RESP" | jq -r '.data.transcription_job.job_id')
```

关键响应字段：

- `media_file.media_id`
- `media_file.status = uploaded`
- `transcription_job.job_id`
- `transcription_job.status = queued`

下一步条件：拿到 `job_id`。

### 4.3 Poll Transcription Job

Endpoint:

```text
GET /api/transcription-jobs/:job_id
```

示例：

```bash
watch -n 2 "curl -sS $API/transcription-jobs/$JOB_ID | jq '.data.status,.data.transcript_id,.data.error_message'"
```

关键状态：

- `queued`
- `processing`
- `succeeded`
- `failed`

下一步条件：

- 成功：`status = succeeded`，响应里有 `transcript_id`
- 失败：`status = failed`，查看 `error_message`

ASR 成功后服务会写 `interview_transcripts`，并将 interview 更新为 `ready_for_review`。

### 4.4 Get Transcript

Endpoint:

```text
GET /api/interviews/:interview_id/transcript
```

示例：

```bash
curl -sS "$API/interviews/$INTERVIEW_ID/transcript"
curl -sS "$API/interviews/$INTERVIEW_ID"
```

关键响应字段：

- transcript: `source_type = asr_audio` 或 `asr_video`
- interview: `status = ready_for_review`

### 4.5 Reuse Review -> Memory -> Coaching -> Mock

后续复用手动 transcript 路径中的步骤：

```text
trigger review
-> get review/questions/transcript-segments
-> generate memory candidates
-> accept selected candidates
-> generate coaching plan
-> start/resume coaching session
-> submit coaching turns
-> list practice_states
-> start/resume mock interview
-> submit mock turns
-> generate completed session/mock memory_candidates
-> accept selected candidates
-> list traces/evaluations
```

## 5. 关键状态速查

### Interview

- `created`
- `ready_for_review`
- `reviewed`

### Review Report

- `generated`
- `failed`

### Transcript Segment

- `pending`
- `extracted`
- `failed`

### Media File

- `uploaded`
- `processing`
- `transcribed`
- `failed`

### Transcription Job

- `queued`
- `processing`
- `succeeded`
- `failed`

### Memory Candidate

- `pending`
- `accepted`
- `rejected`

### Memory Item

- `active`
- `archived`

### Coaching Plan

- `generated`
- `failed`

### Coaching Task

- `todo`
- `in_progress`
- `needs_revision`
- `done`
- `skipped`

### Coaching Session

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

### Mock Interview

- `created`
- `in_progress`
- `waiting_answer`
- `evaluating_answer`
- `asking_followup`
- `switching_topic`
- `completed`
- `failed`
- `cancelled`

### Agent Decision Trace

- `succeeded`
- `failed`

## 6. 常见失败排查

### 缺少或未显式准备 `config.json`

现象：服务仍可能通过 `.env` 启动，但 demo 配置来源不直观；如果 `.env` 也缺少 key，后续 LLM/ASR 调用会失败。

处理：

```bash
cp config.example.json config.json
```

然后填写 LLM/ASR 配置。

### LLM key 或 ASR key 未配置

现象：

- review/coaching/mock/memory candidate 生成失败。
- media job 进入 `failed`。

处理：检查 `config.json` 中 `llm_providers.*.api_key` 和 `asr.api_key`。

### ffmpeg 不可用

现象：视频上传后 transcription job 失败，`error_message` 指向音频提取失败。

处理：

```bash
ffmpeg -version
```

如果命令不可用，安装 ffmpeg 或在 `config.json` 的 `media.ffmpeg_path` 中填写实际路径。

### Review 未完成就生成后续对象

现象：

- generate memory candidates 报 review report 必须 `generated`
- generate coaching plan 报 interview status 必须 `reviewed`
- start mock 报 interview status 必须 `reviewed`

处理：先执行 `POST /api/interviews/:interview_id/review` 并确认 report `status = generated`。

### Session / Mock 未 completed 就生成长期记忆候选

现象：

- coaching session memory candidates 报 session status 必须 `completed`
- mock memory candidates 报 mock status 必须 `completed`

处理：继续推进状态机，或调用 mock complete；coaching session 需要通过任务推进到 completed。

### Trace / Evaluation 为空

常见原因：

- 还没执行会产生 trace 的 Agent 步骤。
- 查询参数过窄，例如 `source_type`、`source_id`、`status` 不匹配。
- review 本身目前不作为主要 trace 覆盖路径，重点看 coaching/mock/candidate generation。

处理：

```bash
curl -sS "$API/agent-decision-traces?limit=20"
curl -sS "$API/agent-evaluations?limit=20"
```

### 手动 transcript 路径与真实 ASR 路径混淆

手动路径用：

```text
PUT /api/interviews/:interview_id/transcript
```

真实 ASR 路径用：

```text
POST /api/interviews/:interview_id/media
GET /api/transcription-jobs/:job_id
```

两条路径最终都会写 `interview_transcripts`，并把 interview 推进到 `ready_for_review`。不要把测试临时 DB 的结果当作真实服务 DB 的结果。
