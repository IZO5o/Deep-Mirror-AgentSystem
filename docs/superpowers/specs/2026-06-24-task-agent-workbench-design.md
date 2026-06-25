# Task Agent Workbench Design

## Goal

Turn the current project into a resume-ready task-oriented Agent engineering project for Agent developer / LLM application engineering interviews.

The project should not be presented as a generic chatbot or as a rigid one-time stepper. It should be presented as an interview training workbench where users can independently:

- Upload or paste real interview materials.
- Let the system asynchronously transcribe and review interview records.
- Review generated reports and structured interview questions.
- Confirm or reject long-term memory candidates.
- Start or resume coaching and mock interview long sessions.
- Inspect selected context, decision traces, evaluations, and failure recovery evidence.

The technical focus is:

```text
observe -> decide -> service-controlled act -> persist -> trace -> evaluate
```

LLM output is treated as structured decision input. Server-side state machines control task progression, persistence, memory boundaries, traceability, evaluation, and failure recovery.

## Current Project Assessment

The backend already has meaningful Agent engineering depth:

- Interview sessions, transcripts, review reports, and interview questions.
- Audio/video upload, asynchronous ASR jobs, and video audio extraction.
- Long transcript segmented review and strict JSON retry hardening.
- `memory_candidates -> accept/reject -> memory_items` boundary.
- `practice_states` separated from formal long-term memory.
- `second_round_coach` coaching plan/session state machine.
- `mock_interviewer` mock interview state machine.
- Service-controlled practice state update helper.
- Session/mock-generated memory candidates with source references.
- Agent decision traces.
- Failure injection tests, golden tests, and trace-based evaluation harness.

The main gap is not raw backend depth. The main gap is product demonstration and narrative clarity:

- The frontend is currently closer to a debug console.
- The current stepper implies users must run one full serial workflow every time.
- The resume story needs to be centered around task Agent engineering, not a generic multi-Agent web demo.

## Product Model

The product should be modeled as three related but independent work areas, sharing a common context layer.

### Interview Materials And Review

Users can create interview records and upload or paste:

- Manual text transcripts.
- Audio interview recordings.
- Video interview recordings.

The system processes these materials asynchronously when needed:

```text
upload / paste
-> transcription job when applicable
-> transcript
-> review report
-> interview questions
-> memory candidates
```

This path produces structured material for later use. It should not force the user to immediately enter coaching or mock interview.

### Memory Inbox

Memory candidates should be handled in an independent confirmation center.

Candidates may come from:

- Review reports.
- Completed coaching sessions.
- Completed mock interviews.

The user can filter and confirm candidates by:

- User.
- Status.
- Source type.
- Company.
- Job title.
- Memory type.
- Confidence.

Only accepted candidates become `memory_items`. Agents must not directly write formal long-term memory.

### Training Long Sessions

Coaching and mock interview are independent long-session experiences.

They can be started or resumed even when a new upload/review job is running elsewhere. They read from confirmed `memory_items`, `practice_states`, and optional selected interview context, but they do not require the user to have just completed an upload flow.

These pages are the main technical showcase:

- Long-running task state machines.
- Current task/question.
- User answer attempts.
- Agent structured decisions.
- Server-side state transitions.
- Practice state updates.
- Selected context snapshots.
- Trace and evaluation evidence.
- Failure handling without dirty writes.

## Frontend Information Architecture

Replace the single-page debug console with a multi-page workbench.

Use `vue-router` and split the current `App.vue` into route-level pages.

### App Shell

`App.vue` should become a layout shell:

- Top or side navigation.
- Current `user_id` selector.
- Global API/error status.
- Route outlet.

It should not contain all product actions.

### Pages

#### Dashboard

Purpose: System overview.

Show:

- Recent interviews and review statuses.
- Pending memory candidate count.
- Active or resumable coaching sessions.
- Active or resumable mock interviews.
- Practice state summary.
- Recent failed traces or evaluation failures.

This page helps interviewers quickly see that the system is a persistent workbench, not a one-off script.

#### Interviews

Purpose: Manage interview materials and review results.

Show:

- Interview list.
- Create interview.
- Upload/paste transcript entry.
- Media upload status.
- Transcription job status.
- Review status.
- Link to interview detail.

#### Interview Detail

Purpose: Inspect one interview record.

Show:

- Interview metadata.
- Transcript status and content preview.
- Media/transcription job summary.
- Review report.
- Interview questions.
- Review-generated memory candidates.
- Related coaching plan summary when available.
- Related mock summary when available.

This page should not require the user to continue into coaching/mock.

#### Memory Inbox

Purpose: Confirm long-term memory.

Show:

- Pending memory candidates across all sources.
- Grouping by source type: review, coaching session, mock interview.
- Company/job/source metadata.
- Accept/reject actions.
- Accepted memory items view.

This page should make the memory boundary visible:

```text
memory_candidates -> user accept/reject -> memory_items
```

#### Coaching

Purpose: Run and inspect `second_round_coach` long sessions.

Show:

- Related company/job/interview context selection.
- Coaching plan and tasks.
- Current session status.
- Current task.
- Conversation turns.
- Latest attempt score and feedback.
- Pause/cancel/resume actions.
- Practice state update evidence.
- Selected context and trace/evaluation panel.

This page is a main Agent engineering showcase.

#### Mock Interview

Purpose: Run and inspect `mock_interviewer` long sessions.

Show:

- Related company/job/interview context selection.
- Current mock status.
- Turn timeline.
- Current question.
- User answer composer.
- Feedback, follow-up, topic switch, completion.
- Practice state update evidence.
- Selected context and trace/evaluation panel.

This page is also a main Agent engineering showcase.

#### Engineering Trace

Purpose: Optional dedicated engineering evidence page.

Show:

- Agent decision traces.
- Selected context snapshots.
- Raw output and parsed decisions.
- Service actions.
- Evaluation checks.
- Failure traces.

This page can be used during interviews to explain why the system is not a black-box chatbot.

## Backend Minimum Changes

The backend should only be changed where necessary to support a coherent product demo. Do not expand the Agent architecture yet.

### Add Read-Only Dashboard Summary API

Endpoint:

```text
GET /api/dashboard-summary?user_id=...
```

Return:

- Recent interviews.
- Pending memory candidate count.
- Recent pending memory candidates.
- Active or resumable coaching sessions.
- Active or resumable mock interviews.
- Practice state summary.
- Recent failed traces.
- Evaluation summary.

This prevents the frontend from stitching too many unrelated APIs for the dashboard.

### Add Read-Only Interview Detail API

Endpoint:

```text
GET /api/interviews/:interview_id/detail?user_id=...
```

Return:

- Interview metadata.
- Transcript status and preview/content.
- Media file summaries.
- Transcription job summaries.
- Review report.
- Interview questions.
- Memory candidates for the interview.
- Coaching plan/task summary when present.
- Mock interview summary when present.

This is a read-only aggregation endpoint. It must not mutate business state.

### Add Global Memory Candidate Query

Endpoint:

```text
GET /api/memory-candidates?user_id=...&status=...&source_ref_type=...
```

Return candidates across review/coaching/mock sources.

Support filters:

- `user_id`
- `status`
- `source_ref_type`
- optional company/job filters if feasible without broad schema changes

This supports Memory Inbox without changing memory write boundaries.

### Optional Demo Seed

Prefer a script first, not necessarily an API:

```text
scripts/seed_demo_data.*
```

Goal:

- Provide stable demo data when real LLM/ASR is unavailable.
- Seed interviews, review-like records, memory candidates/items, practice states, traces, and evaluations if feasible.

If implemented as API, make it clearly local/demo-only and not part of production behavior.

### Avoid For Now

Do not add these in the first implementation pass:

- New business Agents.
- ReAct.
- MCP in the core flow.
- OpenAI function calling in the core flow.
- Full database redesign.
- Replacing existing coaching/mock state machines.
- Letting Agents directly write `memory_items`.

## Data Flow

The pages are connected by shared business state, not by a forced serial wizard.

```text
Interviews
  -> creates transcripts, reviews, questions, memory candidates

Memory Inbox
  -> accepts/rejects memory candidates
  -> creates confirmed memory_items

Coaching / Mock
  -> select confirmed memory_items and practice_states
  -> run long-session state machines
  -> update practice_states on formal answers
  -> optionally generate new memory candidates after completion

Engineering Trace
  -> reads traces/evaluations from Agent execution
  -> proves selected context, decisions, service actions, and failure handling
```

Coaching/mock should still be able to run with little or no confirmed memory. The UI should warn that context is limited rather than blocking the session.

## Error Handling

- Upload/transcription/review failure should not block coaching/mock pages globally.
- LLM unavailable errors should be shown explicitly.
- Trace/evaluation loading failure should not block the main training session.
- Memory candidate generation failure must not write formal memory.
- Coaching/mock parse or persistence failure must not leave dirty attempts/practice states.
- UI should distinguish business blocked states from infrastructure/model failures.

## Testing And Verification

### Backend

Add focused tests for new APIs:

- Dashboard summary aggregation.
- Interview detail aggregation.
- Global memory candidate filtering.

Keep existing tests as engineering evidence:

- State machine tests.
- Failure injection tests.
- Golden tests.
- Trace-based evaluation tests.

Run:

```bash
go test ./...
```

### Frontend

Verify:

- Multi-page navigation works.
- Dashboard loads summary.
- Interviews page and interview detail page are usable.
- Memory Inbox can accept/reject candidates.
- Coaching can start/resume/submit a session.
- Mock can start/resume/submit turns.
- Engineering trace/evaluation evidence is visible.
- API/LLM failures render clear messages.

Run:

```bash
cd frontend
npm run build
```

## Documentation And Resume Deliverables

Update or create:

- README centered on task-oriented Agent engineering.
- Demo guide based on multi-page workbench, not a serial stepper.
- Architecture diagram covering materials, memory, training sessions, trace/evaluation.
- Resume bullets.
- Interview talking points:
  - 2-minute project overview.
  - 5-minute technical deep dive.
  - Common questions and answers.

The main resume message:

```text
Built a task-oriented Agent system for interview review and second-round training.
LLM output is treated as structured decision input; server-side state machines control task progression,
persistence, memory boundaries, traceability, evaluation, and failure recovery.
```

## Acceptance Criteria

- The frontend is a navigable multi-page workbench, not one page with all actions.
- Users can independently manage interview materials, memory confirmation, coaching, and mock interviews.
- Coaching/mock are the primary Agent engineering showcase.
- Memory boundary is visible and preserved.
- Trace/evaluation evidence is visible enough for interview explanation.
- New backend changes are minimal aggregation/query support.
- `go test ./...` passes.
- `frontend npm run build` passes.
- README/demo/resume materials align with the task Agent engineering positioning.

## New Coding Session Handoff

Use this spec as the first input to a new implementation session.

Recommended prompt:

```text
请阅读 docs/superpowers/specs/2026-06-24-task-agent-workbench-design.md，
然后基于当前代码库制定实施计划。目标是把现有 Vue Demo Console 改造成多页面的任务型 Agent 面试训练工作台，
并只做必要的后端最小改动：Dashboard Summary API、Interview Detail API、全局 Memory Candidate 查询。

请不要新增业务 Agent，不要引入 ReAct/MCP/function calling 到主流程，不要破坏 memory_candidates -> accept/reject -> memory_items 边界。
优先保证产品闭环、可演示性、README/简历叙事一致性。

先给实施计划，不要直接大规模改代码。
```

If using the Superpowers workflow, the next skill after this brainstorming spec is `superpowers:writing-plans`.
