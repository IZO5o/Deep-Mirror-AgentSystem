# Agent Session Pages MVP Design

## Goal

Complete the next MVP layer of the task-oriented Agent workbench by turning the current stub pages into usable demonstrations of the project's Agent engineering depth:

- `CoachingPage`
- `MockInterviewPage`
- `EngineeringTracePage`

The goal is not to add new Agent architecture. The goal is to expose the existing coaching/mock state machines, selected context, decision traces, evaluations, and failure evidence through the frontend so the project can be demonstrated as a task-oriented Agent engineering system.

## Current State

The previous MVP phase completed:

- Backend minimal APIs:
  - `GET /api/dashboard-summary`
  - `GET /api/interviews/:interview_id/detail`
  - `GET /api/memory-candidates`
- Frontend multi-page workbench shell:
  - Dashboard
  - Interviews
  - Interview Detail
  - Memory Inbox
  - Coaching
  - Mock Interview
  - Engineering Trace
- First usable product loop:
  - Dashboard summary
  - Interview create/list/detail
  - Transcript save
  - Review trigger
  - Memory candidate query
  - Accept/reject
  - Formal `memory_items` display

Remaining gap:

- `CoachingPage`, `MockInterviewPage`, and `EngineeringTracePage` are still stubs.
- These pages are the most important evidence for the resume positioning: task state machines, controlled persistence, trace, evaluation, and failure recovery.

## Design Principles

- Reuse existing backend APIs first.
- Do not add new business Agents.
- Do not introduce ReAct, MCP, or function calling into the main flow.
- Do not change coaching/mock state machine semantics.
- Do not let Agents directly write `memory_items`.
- Do not build a generic chat UI.
- Make the technical evidence visible: selected context, parsed decision, service actions, evaluation checks, and failures.
- Keep the first version product-complete enough to demo, not feature-complete for production.

## Coaching Page

### Purpose

Show `second_round_coach` as a task-oriented long-session Agent.

The page should let a user generate or resume a coaching plan/session for a selected interview, submit answers, inspect task progress, and jump to related trace/evaluation evidence.

### Data Inputs

The page should support:

- Manual `interview_id` entry or selection from recent reviewed interviews.
- Existing selected IDs from `workbenchState`, especially:
  - `selectedInterviewId`
  - `selectedPlanId`
  - `selectedSessionId`

The page can rely on Dashboard or Interview Detail to seed these IDs. It does not need a new backend list endpoint in the first pass.

### Required UI

Show:

- Interview context:
  - company
  - job title
  - interview status
  - review readiness
- Coaching plan controls:
  - generate plan
  - get plan
  - target round
  - remaining days
- Coaching tasks:
  - title
  - task type
  - status
  - description
  - success criteria
  - current task highlight
- Session controls:
  - start/resume
  - refresh session
  - pause
  - cancel
- Session state:
  - status
  - current task
  - progress summary
  - last agent message
  - error message
- Turn composer:
  - user input textarea
  - submit button
  - clear button
- Latest result:
  - assistant response
  - score
  - feedback
  - action
  - whether an attempt was recorded
- Practice evidence:
  - link or summary showing related `practice_states` changed after formal answers
- Trace/evaluation links:
  - button to open Engineering Trace with filters for `source_type=coaching_session` and current `session_id`

### Backend APIs To Reuse

- `POST /api/interviews/:interview_id/coaching-plan`
- `GET /api/interviews/:interview_id/coaching-plan`
- `GET /api/coaching-plans/:plan_id/tasks`
- `POST /api/coaching-plans/:plan_id/sessions?user_id=...`
- `GET /api/coaching-sessions/:session_id`
- `POST /api/coaching-sessions/:session_id/turns`
- `POST /api/coaching-sessions/:session_id/pause`
- `POST /api/coaching-sessions/:session_id/cancel`
- `GET /api/practice-states?user_id=...`
- `GET /api/agent-decision-traces?...`
- `GET /api/agent-evaluations?...`

### Out Of Scope

- New coaching session list API.
- New backend state transitions.
- Rewriting the coaching prompt.
- Adding tool calling.
- Generating memory candidates from this page unless the session is already completed and the existing endpoint is trivial to expose.

## Mock Interview Page

### Purpose

Show `mock_interviewer` as an independent task Agent with a long-session state machine.

The page should let a user start or resume a mock interview for a selected interview, submit answers, inspect the turn timeline, and jump to related trace/evaluation evidence.

### Data Inputs

The page should support:

- Manual `interview_id` entry or selection from recent reviewed interviews.
- Optional `plan_id`.
- Existing selected IDs from `workbenchState`, especially:
  - `selectedInterviewId`
  - `selectedPlanId`
  - `selectedMockId`

The first pass can rely on Dashboard/Interview Detail to seed these IDs.

### Required UI

Show:

- Interview context:
  - company
  - job title
  - interview status
- Mock controls:
  - start/resume mock
  - refresh mock
  - complete
  - cancel
- Mock state:
  - status
  - current topic
  - current question
  - overall goal
  - error message
- Turn timeline:
  - opening question
  - user answers
  - evaluation turns
  - hints/explanations
  - follow-ups
  - topic switches
  - closing/completion
- Answer composer:
  - answer textarea
  - submit button
  - clear button
- Latest result:
  - feedback
  - score
  - next action
  - practice update summary if available
- Trace/evaluation links:
  - button to open Engineering Trace with filters for `source_type=mock_interview` and current `mock_id`

### Backend APIs To Reuse

- `POST /api/interviews/:interview_id/mock-interviews`
- `GET /api/mock-interviews/:mock_id`
- `GET /api/mock-interviews/:mock_id/turns`
- `POST /api/mock-interviews/:mock_id/turns`
- `POST /api/mock-interviews/:mock_id/complete`
- `POST /api/mock-interviews/:mock_id/cancel`
- `GET /api/practice-states?user_id=...`
- `GET /api/agent-decision-traces?...`
- `GET /api/agent-evaluations?...`

### Out Of Scope

- New mock interview list API.
- New mock state machine semantics.
- New scoring logic.
- New Agent type.
- Tool calling.

## Engineering Trace Page

### Purpose

Make the Agent execution loop inspectable:

```text
selected context -> raw output -> parsed decision -> service actions -> evaluation checks
```

This page should be used during interviews to prove the system is not a black-box chatbot.

### Required Filters

Support filters:

- `user_id`
- `interview_id`
- `source_type`
- `source_id`
- `agent_type`
- `step_name`
- `status`
- `limit`

The page should read optional query params so Coaching/Mock pages can deep-link into filtered trace views.

Example:

```text
/trace?source_type=coaching_session&source_id=<session_id>
/trace?source_type=mock_interview&source_id=<mock_id>
```

### Required UI

Show:

- Trace filter form.
- Trace list:
  - timestamp
  - agent type
  - source type
  - source id
  - step name
  - status
  - error summary
- Trace detail panel:
  - selected context snapshot
  - input snapshot
  - raw agent output
  - parsed decision
  - service actions
  - error message
- Evaluation report:
  - overall trace score
  - check list
  - pass/fail status
  - failed check reason

### Backend APIs To Reuse

- `GET /api/agent-decision-traces`
- `GET /api/agent-evaluations`

### Presentation Rules

- Render JSON in collapsible or scrollable blocks.
- Keep raw output visible but not dominant.
- Highlight failed traces and failed evaluation checks.
- Make memory boundary checks visible when present.

## Shared Frontend Work

Add API helper functions for:

- coaching plan generation/read
- coaching tasks
- coaching session start/read/submit/pause/cancel
- mock start/read/turns/submit/complete/cancel
- practice state list
- trace list
- evaluation list

Update `workbenchState` to remember:

- `selectedInterviewId`
- `selectedPlanId`
- `selectedSessionId`
- `selectedMockId`

Add or reuse small components:

- status badge
- empty state
- error notice
- JSON block
- trace/evaluation summary card
- turn timeline item

## Backend Change Policy

The intended implementation should not need backend changes.

Only consider adding backend APIs if implementation proves resume/restore is not usable through existing Dashboard and Interview Detail data. If needed, only add read-only list endpoints:

```text
GET /api/coaching-sessions?user_id=...&status=active
GET /api/mock-interviews?user_id=...&status=active
```

These optional endpoints must not mutate state and must have tests.

## Error Handling

- If no interview is selected, show a clear empty state with a link to Interviews.
- If selected interview is not reviewed, explain that coaching/mock may need review context first.
- If LLM call fails, show the backend error and link to Trace when available.
- If a session is `failed`, disable submit and show error details.
- If a session is `completed` or `cancelled`, disable submit and offer refresh/start where valid.
- If trace/evaluation loading fails, do not block coaching/mock interaction.

## Testing And Verification

### Backend

No backend change is expected. Existing tests should continue to pass:

```bash
go test ./...
```

If optional list endpoints are added, add focused service/controller tests.

### Frontend

Run:

```bash
cd frontend
npm run build
```

Manual MVP checks:

- Coaching page can load/generate plan for an interview.
- Coaching page can start/resume a session.
- Coaching page can submit a user turn and display returned session details.
- Mock page can start/resume a mock interview.
- Mock page can submit a turn and display turn timeline.
- Trace page can list traces.
- Trace page can filter by coaching session or mock interview source.
- Trace page can display evaluation checks.
- Memory boundary remains unchanged.

## Acceptance Criteria

- `CoachingPage` is no longer a stub and demonstrates the coaching state machine.
- `MockInterviewPage` is no longer a stub and demonstrates the mock interview state machine.
- `EngineeringTracePage` is no longer a stub and demonstrates Agent observability and evaluation.
- Existing Dashboard, Interviews, Interview Detail, and Memory Inbox continue to work.
- No new business Agent is added.
- No core flow ReAct/MCP/function calling is added.
- `memory_candidates -> accept/reject -> memory_items` remains the only formal memory write path.
- `go test ./...` passes.
- `cd frontend && npm run build` passes.

## New Implementation Session Handoff

Recommended prompt for the next planning session:

```text
[$superpowers:writing-plans]

请阅读 docs/superpowers/specs/2026-06-25-agent-session-pages-design.md，
基于当前代码库写一个详细实施计划，保存到 docs/superpowers/plans/。

目标是把当前 stub 的 Coaching / Mock Interview / Engineering Trace 页面做成可演示 MVP，
优先复用现有后端 API，不新增业务 Agent，不引入 ReAct/MCP/function calling 到主流程，
不破坏 memory_candidates -> accept/reject -> memory_items 边界。

先规划，不要直接实现。每个任务要可独立测试和验收。
```

Recommended execution prompt after plan approval:

```text
[$superpowers:subagent-driven-development]

请阅读刚生成的计划文件，按计划使用 subagent-driven-development 执行。

要求：
1. 每个任务独立执行，任务之间必须有检查点和 review。
2. 优先完成 Coaching 页、Mock 页、Trace 页的前端 MVP。
3. 默认不改后端；如果必须新增只读接口，先说明原因并加测试。
4. 每阶段运行 npm run build。
5. 涉及后端时运行 go test ./...。
6. 不新增业务 Agent，不引入 ReAct/MCP/function calling 到主流程。
7. 不破坏 memory_candidates -> accept/reject -> memory_items 边界。
8. 每完成一个阶段，汇报改了哪些文件、验证结果、剩余风险。
```
