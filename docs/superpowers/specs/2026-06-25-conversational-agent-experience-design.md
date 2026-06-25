# Conversational Agent Experience Design

## Goal

Improve the coaching and mock interview experience from a visible state-machine console into a natural long-conversation task Agent experience.

The system should keep its task-oriented engineering strengths:

- server-controlled state machines
- explicit persistence boundaries
- practice state updates only when appropriate
- trace and evaluation evidence
- no direct Agent writes to `memory_items`

But users should experience the two main Agents as conversational:

- `second_round_coach`: a Chinese-speaking practice mentor
- `mock_interviewer`: a realistic interviewer with limited off-record support

## Problems Observed

### Version Management

The workspace is currently not a Git repository. Code sessions cannot reliably:

- inspect diffs
- create commits
- rollback changes
- isolate implementation phases
- review generated build artifacts

This should be fixed before the next implementation phase.

### UI Feels Like A State Machine Console

The current Coaching and Mock pages expose too much operational state in the main visual area:

- session state
- task list
- attempt cards
- action labels
- timeline event metadata
- trace-style action names

The main user experience should be a chat-like long conversation. Task lists, attempts, practice evidence, and traces should be available, but as secondary panels.

### Agent Behavior Feels Rigid

Current prompts force user input into a small set of actions, such as:

```text
formal_answer
hint_request
explanation_request
skip_task
pause
```

This makes the Agent behave like a classifier rather than a conversational coach/interviewer.

Observed issue:

- User says "好的，下一题吧".
- Agent classifies it as `skip_task`.
- The session loops around skipping/prompting rather than naturally moving forward.

### Language Is Inconsistent

UI labels and Agent responses mix English and Chinese. Since this project is intended for Chinese interview preparation and the user prefers Chinese interaction, the main product and Agent interaction should default to Chinese.

Engineering labels can remain in Trace/Debug panels.

## Product Direction

The product should distinguish between:

- user-facing conversation
- internal intent
- state-machine action
- engineering trace

The user should mainly see conversation. The system should still record internal decisions for trace/evaluation.

## UI Design

### Shared Layout For Coaching And Mock

Use a long-conversation layout:

```text
top summary bar
main chat stream
fixed bottom composer
right collapsible evidence panel
```

### Top Summary Bar

Show one compact line of context.

Coaching:

```text
当前训练重点：Redis 分布式锁争议点表达
状态：等待回答
```

Mock:

```text
当前题目：请解释 Redlock 的适用场景和争议
状态：等待回答
```

Do not show full task lists in the main view.

### Main Chat Stream

Show natural user/assistant bubbles.

Do not show internal labels such as:

```text
skip_task
prompt_next_task
continue_current_task
formal_answer
ask_retry
```

Those belong in debug/trace panels only.

### Composer

Coaching composer:

- default action: `发送`
- explicit action: `作为正式回答提交`

Mock composer:

- default action: `回答`
- secondary action: `场外提问`

This creates a hybrid model:

- Natural conversation remains easy.
- Formal answer submission remains explicit and reliable.

### Right Evidence Panel

Default collapsed or visually secondary.

Sections:

- training plan / task list
- attempts
- practice states
- trace link
- evaluation summary
- raw/debug details

The evidence panel is useful for interviews and debugging, but should not dominate the user experience.

## Interaction Model

### Submit Mode

Add a frontend/backend request field:

```json
{
  "user_input": "string",
  "submit_mode": "chat|formal_answer"
}
```

Coaching:

- `chat`: normal discussion, hints, explanations, clarification, navigation
- `formal_answer`: user wants this message scored as an answer attempt

Mock:

Use the same field but interpret modes as:

- `formal_answer`: candidate answer to the interviewer
- `chat`: off-record question, clarification, hint request, pause, or meta discussion

The field should default safely:

- Coaching default: `chat`
- Mock default: `formal_answer`

### Internal Intent

Introduce a lightweight internal intent layer. This is not a full NLU system.

Supported first-pass intents:

```text
answer
ask_hint
ask_explain
confirm_next
retry_current
skip_current
smalltalk
unclear
pause
```

Mock may also use:

```text
end_mock
```

### State Action

Separate user intent from state action.

Possible state actions:

```text
chat_only
record_attempt
ask_retry
move_next
stay_current
pause
complete
```

Intent answers "what did the user mean?"

State action answers "what should the service do?"

### Output Schema

Coaching and mock turn prompts should return strict JSON, but with user-facing message separated from internal control:

```json
{
  "visible_message": "中文回复",
  "user_intent": "answer|ask_hint|ask_explain|confirm_next|retry_current|skip_current|smalltalk|unclear|pause",
  "state_action": "chat_only|record_attempt|ask_retry|move_next|stay_current|pause|complete",
  "confidence": 0.82,
  "needs_clarification": false,
  "score": 0,
  "passed": false,
  "feedback": "string",
  "should_update_practice_state": false
}
```

The old fields can be mapped for compatibility during implementation, but the user-facing design should follow the new separation.

## Intent Rules

### Confirm Next

Examples:

```text
下一题吧
继续
好的，下一个
进入下一题
可以开始下一个重点
```

These should map to:

```text
user_intent = confirm_next
state_action = move_next
```

They should not be treated as `skip_current` unless the user clearly says they are giving up on the current task.

### Skip Current

Examples:

```text
这题不会，跳过吧
放弃这题
这个先不练了
换一个，我不想做这个
```

These can map to:

```text
user_intent = skip_current
state_action = move_next
```

The visible message should acknowledge the skip and explain the consequence.

### Unclear

When confidence is low, do not advance state.

Return:

```text
user_intent = unclear
state_action = chat_only
needs_clarification = true
```

Example visible message:

```text
你是想直接进入下一题，还是想先放弃当前这题？如果只是想继续，我可以直接带你进入下一个重点。
```

### Smalltalk

Examples:

```text
我有点紧张
这个我之前面试也遇到过
你讲慢一点
我想一下
```

Return:

```text
user_intent = smalltalk
state_action = chat_only
```

No attempt, no score, no practice state update.

## Coaching Agent Behavior

Coaching should feel like a Chinese-speaking practice mentor.

Allowed:

- explain concepts
- provide hints
- help structure answers
- ask the user to try a version
- discuss tradeoffs
- clarify user intent
- summarize what to improve

Avoid:

- direct full answer dumping as the default
- scoring every user message
- forcing every turn into the current task
- exposing internal state labels in user-facing text

Preferred pattern:

1. Give a structure or hint.
2. Ask the user to say a version.
3. Only score when the user clicks `作为正式回答提交` or clearly submits a formal answer.

## Mock Agent Behavior

Mock should feel like a realistic interviewer.

Allowed:

- ask one question at a time
- ask follow-ups
- switch topics
- end the mock
- briefly handle off-record questions

Avoid:

- long coaching explanations during interview mode
- over-teaching before the user answers
- scoring off-record questions
- updating practice state for hints or meta discussion

Default mode is formal answer. `场外提问` lets the user step outside the interview briefly.

## Backend Boundary

Keep service-controlled persistence.

Only these should record attempts/update practice states:

- Coaching `submit_mode=formal_answer` with `state_action=record_attempt`
- Mock formal candidate answers with `should_update_practice_state=true`

These should not update practice state:

- chat
- hints
- explanations
- smalltalk
- unclear
- pause
- off-record mock questions

Memory boundary remains unchanged:

```text
memory_candidates -> accept/reject -> memory_items
```

No Agent should directly write `memory_items`.

## Trace And Evaluation

Trace should record the new fields:

- `submit_mode`
- `user_intent`
- `state_action`
- `confidence`
- `needs_clarification`
- `visible_message`

This becomes an interview talking point:

```text
The system separates conversational intent from service-controlled state action,
so the Agent can speak naturally while the server still controls persistence and failure boundaries.
```

Evaluation checks can later verify:

- chat-only turns do not create attempts
- smalltalk/unclear do not update practice state
- formal answers can create attempts
- memory boundary remains intact
- low-confidence ambiguity asks clarification instead of advancing state

## Git Baseline

Before implementation, initialize version control if the workspace is still not a Git repository.

Recommended baseline:

- `git init`
- add `.gitignore` entries for generated/local artifacts:
  - `frontend/dist/`
  - `agent-web-base.db`
  - `.superpowers/`
  - `.agent-web-base/`
  - local audio/video samples if not intended for version control
  - `.env`
  - `config.json`
- commit current project as baseline before the conversational experience change

This is required for safer iteration and review.

## Implementation Scope

First implementation pass:

1. Git baseline.
2. Coaching/Mock layout changed to chat-first UI.
3. Add composer modes:
   - Coaching: `发送`, `作为正式回答提交`
   - Mock: `回答`, `场外提问`
4. Backend request supports `submit_mode`.
5. Prompt defaults to Chinese.
6. Prompt separates `user_intent` and `state_action`.
7. Add `smalltalk` and `unclear`.
8. Add tests for:
   - confirm_next is not treated as skip
   - chat mode does not create attempt/practice state
   - formal answer mode records attempt
   - unclear does not advance state

Out of scope:

- vector memory
- new business Agents
- ReAct
- MCP
- function calling in the core flow
- broad NLU engine
- complete redesign of coaching/mock database tables

## Acceptance Criteria

- Coaching page feels like a long conversation with a mentor.
- Mock page feels like a long conversation with an interviewer.
- Main chat does not expose internal action labels.
- Task lists and traces are available but secondary/collapsible.
- System defaults to Chinese user-facing text.
- `下一题吧` / `继续` are not misclassified as `skip_current`.
- `smalltalk` and `unclear` do not create attempts or update practice state.
- Formal answer submission is explicit and reliable.
- Existing memory boundary is preserved.
- `go test ./...` passes.
- `cd frontend && npm run build` passes.

## New Implementation Session Handoff

Recommended planning prompt:

```text
[$superpowers:writing-plans]

请阅读 docs/superpowers/specs/2026-06-25-conversational-agent-experience-design.md，
基于当前代码库写一个详细实施计划，保存到 docs/superpowers/plans/。

目标是把 Coaching / Mock 从状态机面板改造成长对话任务 Agent 体验：
先建立 Git baseline，然后实现 chat-first UI、混合提交模式、默认中文、轻量 user_intent/state_action 分离、
smalltalk/unclear 兜底，并保持 memory_candidates -> accept/reject -> memory_items 边界。

先规划，不要直接实现。每个任务要可独立测试和验收。
```

Recommended execution prompt after plan approval:

```text
[$superpowers:subagent-driven-development]

请阅读刚生成的计划文件，按计划使用 subagent-driven-development 执行。

要求：
1. 先完成 Git baseline。
2. 每个任务独立执行，任务之间必须有检查点和 review。
3. 后端行为改动必须优先写测试。
4. 前端每阶段运行 npm run build。
5. 后端每阶段运行 go test ./...。
6. 不新增业务 Agent，不引入 ReAct/MCP/function calling 到主流程。
7. 不破坏 memory_candidates -> accept/reject -> memory_items 边界。
8. 每完成一个阶段，汇报改了哪些文件、验证结果、剩余风险。
```
