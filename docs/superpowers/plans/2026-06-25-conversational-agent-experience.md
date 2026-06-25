# Conversational Agent Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert Coaching and Mock from state-machine panels into Chinese-first long-conversation task Agent experiences while keeping server-controlled persistence, traceability, and the `memory_candidates -> accept/reject -> memory_items` boundary.

**Architecture:** Keep the existing Go services, tables, and fixed Agents. Add `submit_mode` as an API/service boundary, map new `user_intent/state_action` prompt fields to the existing turn/attempt/mock persistence paths, and make chat-only turns non-mutating. Refactor only the two Vue pages into chat-first layouts with evidence in secondary/collapsible panels.

**Tech Stack:** Go, Gin-style controllers, GORM, existing fake-agent Go tests, Vue 3, Vite, plain CSS.

---

## File Structure

- Modify `.gitignore`: add local/generated artifacts missing from the current ignore list, especially `.superpowers/` and local audio/video samples.
- Modify `vo/vo.go`: add `submit_mode` to coaching and mock submit requests.
- Modify `server/coaching_session_service.go`: add submit-mode constants, Chinese-first prompt schema, new `user_intent/state_action` fields, compatibility mapping, chat-only handling, and trace fields.
- Modify `server/coaching_session_service_test.go`: add TDD tests for submit mode, chat-only behavior, confirm-next behavior, unclear/smalltalk, and prompt schema.
- Modify `server/coaching_session_golden_test.go`: update existing golden requests to pass `submit_mode=formal_answer` where attempts are expected.
- Modify `server/mock_interview_service.go`: add submit-mode constants, Chinese-first prompt schema, new fields, chat/off-record handling, formal-answer persistence guard, and trace fields.
- Modify `server/mock_interview_service_test.go`: add TDD tests for mock default formal mode, off-record chat, unclear/smalltalk, and prompt schema.
- Modify `server/mock_interview_golden_test.go`: update existing formal-answer golden requests to use explicit or default formal mode and add trace assertions for new fields.
- Modify `server/agent_evaluation_service_test.go`: update trace fixtures to include `user_intent` and `state_action` while preserving the direct `memory_items` write-boundary check.
- Modify `frontend/src/api.js`: normalize submit helper bodies to pass `submit_mode`.
- Modify `frontend/src/api.test.mjs`: assert `submit_mode` is serialized for coaching and mock.
- Modify `frontend/src/pages/CoachingPage.vue`: replace dashboard-first page layout with top summary, chat stream, fixed composer, and collapsible evidence panels.
- Modify `frontend/src/pages/MockInterviewPage.vue`: same chat-first treatment for mock, with primary `回答` and secondary `场外提问`.
- Modify `frontend/src/styles.css`: add shared long-conversation layout, chat bubbles, compact top summary, fixed composer, and evidence panel styles.

Do not create new business Agents. Do not add ReAct, MCP, or function calling to the core flow. Do not add or redesign database tables.

---

### Task 1: Git Baseline And Ignore Hygiene

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Confirm repository state**

Run:

```bash
git rev-parse --is-inside-work-tree
```

Expected before implementation in this workspace:

```text
fatal: not a git repository (or any of the parent directories): .git
```

- [ ] **Step 2: Extend `.gitignore` before creating Git metadata**

Replace `.gitignore` with:

```gitignore
.env
.env.*
!.env.example
config.json
mcp-server.json

*.db
*.db-*
agent-web-base.db

.agent-web-base/
.superpowers/
.gocache/

frontend/node_modules/
frontend/dist/

*.mp3
*.mp4
*.wav
*.m4a
*.mov
```

This keeps `.env.example` and `config.example.json` trackable while excluding the current local DB, local config, generated runtime state, vendor install, build output, and large local media samples.

- [ ] **Step 3: Initialize Git**

Run:

```bash
git init
git status --short
```

Expected:

```text
Initialized empty Git repository in /Users/zhengzhan/MyProject/agent-web-base/.git/
```

`git status --short` should not list `frontend/node_modules/`, `agent-web-base.db`, `.env`, `config.json`, `.superpowers/`, `.agent-web-base/`, or `*.mp3`.

- [ ] **Step 4: Create baseline commit**

Run:

```bash
git add .
git status --short
git commit -m "chore: establish project baseline"
```

Expected:

```text
[main (root-commit) <commit-hash>] chore: establish project baseline
```

- [ ] **Step 5: Verify clean baseline**

Run:

```bash
git status --short
```

Expected: no output.

---

### Task 2: Add Submit Mode To API Contracts

**Files:**
- Modify: `vo/vo.go`
- Modify: `frontend/src/api.test.mjs`
- Modify: `frontend/src/api.js`

- [ ] **Step 1: Write the failing frontend API test**

In `frontend/src/api.test.mjs`, replace the existing coaching submit assertion:

```js
await assertPostBody(
  'submitCoachingTurn path/body',
  () => api.submitCoachingTurn('session_1', { user_input: 'answer', input_type: 'formal_answer' }),
  '/api/coaching-sessions/session_1/turns',
  { user_input: 'answer', input_type: 'formal_answer' },
)
```

with:

```js
await assertPostBody(
  'submitCoachingTurn path/body',
  () => api.submitCoachingTurn('session_1', { user_input: 'answer', submit_mode: 'formal_answer' }),
  '/api/coaching-sessions/session_1/turns',
  { user_input: 'answer', submit_mode: 'formal_answer' },
)
```

Replace the existing mock submit assertion:

```js
await assertPostBody(
  'submitMockTurn path/body',
  () => api.submitMockTurn('mock_1', { answer: 'my answer' }),
  '/api/mock-interviews/mock_1/turns',
  { answer: 'my answer' },
)
```

with:

```js
await assertPostBody(
  'submitMockTurn path/body',
  () => api.submitMockTurn('mock_1', { answer: 'my answer', submit_mode: 'chat' }),
  '/api/mock-interviews/mock_1/turns',
  { answer: 'my answer', submit_mode: 'chat' },
)
```

- [ ] **Step 2: Run the frontend API test**

Run:

```bash
cd frontend && npm run test:api
```

Expected: PASS. This test only verifies that the API helper forwards the body it is given, so no frontend implementation change should be required yet.

- [ ] **Step 3: Add `submit_mode` to request VOs**

In `vo/vo.go`, replace:

```go
type SubmitCoachingSessionTurnReq struct {
	UserInput string `json:"user_input" binding:"required"`
}
```

with:

```go
type SubmitCoachingSessionTurnReq struct {
	UserInput  string `json:"user_input" binding:"required"`
	SubmitMode string `json:"submit_mode,omitempty"`
}
```

Replace:

```go
type SubmitMockTurnReq struct {
	Answer string `json:"answer" binding:"required"`
}
```

with:

```go
type SubmitMockTurnReq struct {
	Answer     string `json:"answer" binding:"required"`
	SubmitMode string `json:"submit_mode,omitempty"`
}
```

- [ ] **Step 4: Run contract-adjacent tests**

Run:

```bash
go test ./server ./vo
cd frontend && npm run test:api
```

Expected: Go tests compile and pass; frontend API test passes.

- [ ] **Step 5: Commit**

Run:

```bash
git add vo/vo.go frontend/src/api.test.mjs frontend/src/api.js
git commit -m "feat: add submit mode to turn requests"
```

---

### Task 3: Coaching Intent Schema And Chat-Only Backend Behavior

**Files:**
- Modify: `server/coaching_session_service_test.go`
- Modify: `server/coaching_session_golden_test.go`
- Modify: `server/coaching_session_service.go`

- [ ] **Step 1: Add failing coaching tests**

Append these tests to `server/coaching_session_service_test.go` before helper functions:

```go
func TestSubmitCoachingSessionTurn_ChatModeDoesNotCreateAttemptEvenIfAgentScores(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingConversationDecisionJSON(
		CoachingUserIntentAskHint,
		CoachingStateActionRecordAttempt,
		"可以，先给你一个回答结构：背景、冲突、取舍、落地。你可以按这个结构试一版。",
		70,
		false,
		false,
	)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{
		UserInput:  "这个怎么组织更好？",
		SubmitMode: CoachingSubmitModeChat,
	})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if len(updated.Attempts) != 0 {
		t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
	}
	assertNoPracticeStates(t, s, "user_001")
	if updated.Session.Status != CoachingSessionStatusWaitingUserAnswer || updated.Session.CurrentTaskID != taskID {
		t.Fatalf("session = %#v, want waiting same task", updated.Session)
	}
	if got := updated.Turns[len(updated.Turns)-1].Content; !strings.Contains(got, "回答结构") {
		t.Fatalf("assistant content = %q, want visible Chinese chat message", got)
	}
	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceCoachingSession,
		SourceID:   session.Session.SessionID,
		StepName:   AgentTraceStepCoachingSessionTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	for _, want := range []string{`"submit_mode":"chat"`, `"user_intent":"ask_hint"`, `"state_action":"chat_only"`} {
		if !strings.Contains(trace.InputSnapshot+trace.ParsedDecision, want) {
			t.Fatalf("trace missing %s: input=%s parsed=%s", want, trace.InputSnapshot, trace.ParsedDecision)
		}
	}
	assertTraceNotContainsAction(t, trace, "recorded coaching_task_attempt")
	assertTraceNotContainsAction(t, trace, "updated practice_states")
}

func TestSubmitCoachingSessionTurn_FormalModeRecordsAttempt(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingConversationDecisionJSON(
		CoachingUserIntentAnswer,
		CoachingStateActionRecordAttempt,
		"这版回答达标，可以进入下一个重点。",
		88,
		true,
		true,
	)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{
		UserInput:  "我会从一致性、补偿和监控三个角度回答。",
		SubmitMode: CoachingSubmitModeFormalAnswer,
	})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if len(updated.Attempts) != 1 || !updated.Attempts[0].Passed || updated.Attempts[0].Score != 88 {
		t.Fatalf("attempts = %#v, want one passed score 88", updated.Attempts)
	}
	states, err := s.ListPracticeStates("user_001", "补齐 Redis 缓存一致性回答", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) != 1 || states[0].SourceID != updated.Attempts[0].AttemptID {
		t.Fatalf("practice states = %#v, want source attempt", states)
	}
}

func TestSubmitCoachingSessionTurn_ConfirmNextMovesWithoutSkipOrPractice(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	taskID := session.Session.CurrentTaskID
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingConversationDecisionJSON(
		CoachingUserIntentConfirmNext,
		CoachingStateActionMoveNext,
		"好的，我们进入下一个重点。",
		0,
		false,
		false,
	)

	updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{
		UserInput:  "好的，下一题吧",
		SubmitMode: CoachingSubmitModeChat,
	})
	if err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	if len(updated.Attempts) != 0 {
		t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
	}
	assertNoPracticeStates(t, s, "user_001")
	var original CoachingTask
	if err := s.db.First(&original, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("load original task: %v", err)
	}
	if original.Status == CoachingTaskStatusSkipped {
		t.Fatalf("task status = skipped, want confirm_next not skip_current")
	}
	if updated.Session.CurrentTaskID == taskID {
		t.Fatalf("current_task_id = %q, want moved to another runnable task", updated.Session.CurrentTaskID)
	}
}

func TestSubmitCoachingSessionTurn_UnclearAndSmalltalkDoNotAdvance(t *testing.T) {
	for _, tc := range []struct {
		name        string
		intent      string
		action      string
		userInput   string
		clarify     bool
		messageWant string
	}{
		{
			name:        "unclear",
			intent:      CoachingUserIntentUnclear,
			action:      CoachingStateActionChatOnly,
			userInput:   "这个算吗",
			clarify:     true,
			messageWant: "你是想直接进入下一题",
		},
		{
			name:        "smalltalk",
			intent:      CoachingUserIntentSmalltalk,
			action:      CoachingStateActionChatOnly,
			userInput:   "我有点紧张",
			clarify:     false,
			messageWant: "放慢一点",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, runners, plan := createCoachingSessionReadyPlan(t)
			session := startTestCoachingSession(t, s, plan.PlanID)
			taskID := session.Session.CurrentTaskID
			runners[agent.AgentTypeSecondRoundCoach].taskResponse = `{
  "visible_message": "` + tc.messageWant + `，我们先不推进状态。",
  "user_intent": "` + tc.intent + `",
  "state_action": "` + tc.action + `",
  "confidence": 0.45,
  "needs_clarification": ` + boolString(tc.clarify) + `,
  "score": 0,
  "passed": false,
  "feedback": "",
  "should_update_practice_state": false
}`

			updated, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{
				UserInput:  tc.userInput,
				SubmitMode: CoachingSubmitModeChat,
			})
			if err != nil {
				t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
			}
			if len(updated.Attempts) != 0 {
				t.Fatalf("attempts length = %d, want 0", len(updated.Attempts))
			}
			assertNoPracticeStates(t, s, "user_001")
			if updated.Session.CurrentTaskID != taskID || updated.Session.Status != CoachingSessionStatusWaitingUserAnswer {
				t.Fatalf("session = %#v, want same task waiting", updated.Session)
			}
			if !strings.Contains(updated.Turns[len(updated.Turns)-1].Content, tc.messageWant) {
				t.Fatalf("assistant content = %q, want %q", updated.Turns[len(updated.Turns)-1].Content, tc.messageWant)
			}
		})
	}
}
```

Add this helper after `sampleCoachingSessionDecisionJSON`:

```go
func sampleCoachingConversationDecisionJSON(intent string, stateAction string, message string, score int, passed bool, complete bool) string {
	return `{
  "visible_message": "` + message + `",
  "user_intent": "` + intent + `",
  "state_action": "` + stateAction + `",
  "confidence": 0.88,
  "needs_clarification": false,
  "score": ` + strconv.Itoa(score) + `,
  "passed": ` + boolString(passed) + `,
  "feedback": "` + message + `",
  "should_update_practice_state": ` + boolString(stateAction == CoachingStateActionRecordAttempt) + `,
  "should_complete_current_task": ` + boolString(complete) + `
}`
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./server -run 'TestSubmitCoachingSessionTurn_(ChatMode|FormalMode|ConfirmNext|Unclear)' -v
```

Expected: FAIL with undefined constants such as `CoachingSubmitModeChat`, `CoachingUserIntentAskHint`, or missing new behavior.

- [ ] **Step 3: Add coaching constants and output fields**

In `server/coaching_session_service.go`, add these constants after the existing coaching action constants:

```go
const (
	CoachingSubmitModeChat         = "chat"
	CoachingSubmitModeFormalAnswer = "formal_answer"

	CoachingUserIntentAnswer       = "answer"
	CoachingUserIntentAskHint      = "ask_hint"
	CoachingUserIntentAskExplain   = "ask_explain"
	CoachingUserIntentConfirmNext  = "confirm_next"
	CoachingUserIntentRetryCurrent = "retry_current"
	CoachingUserIntentSkipCurrent  = "skip_current"
	CoachingUserIntentSmalltalk    = "smalltalk"
	CoachingUserIntentUnclear      = "unclear"
	CoachingUserIntentPause        = "pause"

	CoachingStateActionChatOnly     = "chat_only"
	CoachingStateActionRecordAttempt = "record_attempt"
	CoachingStateActionAskRetry     = "ask_retry"
	CoachingStateActionMoveNext     = "move_next"
	CoachingStateActionStayCurrent  = "stay_current"
	CoachingStateActionPause        = "pause"
	CoachingStateActionComplete     = "complete"
)
```

Replace `coachingSessionAgentOutput` with:

```go
type coachingSessionAgentOutput struct {
	VisibleMessage            string  `json:"visible_message"`
	UserIntent                string  `json:"user_intent"`
	StateAction               string  `json:"state_action"`
	Confidence                float64 `json:"confidence"`
	NeedsClarification        bool    `json:"needs_clarification"`
	Score                     int     `json:"score"`
	Passed                    bool    `json:"passed"`
	Feedback                  string  `json:"feedback"`
	ShouldUpdatePracticeState bool    `json:"should_update_practice_state"`
	ShouldCompleteCurrentTask bool    `json:"should_complete_current_task"`
	ShouldPause               bool    `json:"should_pause"`

	InputType    string `json:"input_type"`
	AgentMessage string `json:"agent_message"`
	NextAction   string `json:"next_action"`
}
```

- [ ] **Step 4: Normalize submit mode and parsed output**

Add these helper functions near `normalizeCoachingInputType`:

```go
func normalizeCoachingSubmitMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case CoachingSubmitModeFormalAnswer:
		return CoachingSubmitModeFormalAnswer
	case CoachingSubmitModeChat, "":
		return CoachingSubmitModeChat
	default:
		return CoachingSubmitModeChat
	}
}

func normalizeCoachingUserIntent(intent string) string {
	switch strings.TrimSpace(intent) {
	case CoachingUserIntentAnswer,
		CoachingUserIntentAskHint,
		CoachingUserIntentAskExplain,
		CoachingUserIntentConfirmNext,
		CoachingUserIntentRetryCurrent,
		CoachingUserIntentSkipCurrent,
		CoachingUserIntentSmalltalk,
		CoachingUserIntentUnclear,
		CoachingUserIntentPause:
		return strings.TrimSpace(intent)
	default:
		return CoachingUserIntentUnclear
	}
}

func normalizeCoachingStateAction(action string) string {
	switch strings.TrimSpace(action) {
	case CoachingStateActionChatOnly,
		CoachingStateActionRecordAttempt,
		CoachingStateActionAskRetry,
		CoachingStateActionMoveNext,
		CoachingStateActionStayCurrent,
		CoachingStateActionPause,
		CoachingStateActionComplete:
		return strings.TrimSpace(action)
	default:
		return CoachingStateActionChatOnly
	}
}

func coerceCoachingOutputForSubmitMode(parsed coachingSessionAgentOutput, submitMode string) coachingSessionAgentOutput {
	if strings.TrimSpace(parsed.VisibleMessage) == "" {
		parsed.VisibleMessage = parsed.AgentMessage
	}
	if strings.TrimSpace(parsed.Feedback) == "" {
		parsed.Feedback = parsed.VisibleMessage
	}
	if strings.TrimSpace(parsed.UserIntent) == "" {
		switch normalizeCoachingInputType(parsed.InputType) {
		case CoachingInputTypeFormalAnswer:
			parsed.UserIntent = CoachingUserIntentAnswer
		case CoachingInputTypeHintRequest:
			parsed.UserIntent = CoachingUserIntentAskHint
		case CoachingInputTypeExplanationRequest:
			parsed.UserIntent = CoachingUserIntentAskExplain
		case CoachingInputTypeSkipTask:
			parsed.UserIntent = CoachingUserIntentSkipCurrent
		case CoachingInputTypePause:
			parsed.UserIntent = CoachingUserIntentPause
		default:
			parsed.UserIntent = CoachingUserIntentUnclear
		}
	}
	if strings.TrimSpace(parsed.StateAction) == "" {
		switch normalizeCoachingInputType(parsed.InputType) {
		case CoachingInputTypeFormalAnswer:
			parsed.StateAction = CoachingStateActionRecordAttempt
		case CoachingInputTypeSkipTask:
			parsed.StateAction = CoachingStateActionMoveNext
		case CoachingInputTypePause:
			parsed.StateAction = CoachingStateActionPause
		default:
			parsed.StateAction = CoachingStateActionChatOnly
		}
	}
	parsed.UserIntent = normalizeCoachingUserIntent(parsed.UserIntent)
	parsed.StateAction = normalizeCoachingStateAction(parsed.StateAction)
	if submitMode != CoachingSubmitModeFormalAnswer && parsed.StateAction == CoachingStateActionRecordAttempt {
		parsed.StateAction = CoachingStateActionChatOnly
		parsed.ShouldUpdatePracticeState = false
	}
	if parsed.StateAction == CoachingStateActionRecordAttempt {
		parsed.InputType = CoachingInputTypeFormalAnswer
		parsed.ShouldUpdatePracticeState = true
	} else if parsed.StateAction == CoachingStateActionPause {
		parsed.InputType = CoachingInputTypePause
		parsed.ShouldPause = true
	} else if parsed.UserIntent == CoachingUserIntentAskHint {
		parsed.InputType = CoachingInputTypeHintRequest
	} else if parsed.UserIntent == CoachingUserIntentAskExplain {
		parsed.InputType = CoachingInputTypeExplanationRequest
	} else if parsed.UserIntent == CoachingUserIntentSkipCurrent {
		parsed.InputType = CoachingInputTypeSkipTask
	} else {
		parsed.InputType = CoachingTurnTypeUserAnswer
	}
	if parsed.StateAction == CoachingStateActionMoveNext {
		parsed.NextAction = CoachingNextActionPromptNext
	} else if parsed.StateAction == CoachingStateActionAskRetry {
		parsed.NextAction = CoachingNextActionAskRetry
	} else if parsed.StateAction == CoachingStateActionPause {
		parsed.NextAction = CoachingNextActionPause
	} else if parsed.StateAction == CoachingStateActionComplete {
		parsed.NextAction = CoachingNextActionCompletePlan
	} else if strings.TrimSpace(parsed.NextAction) == "" {
		parsed.NextAction = CoachingNextActionContinue
	}
	return parsed
}
```

- [ ] **Step 5: Thread submit mode through prompt and trace**

In `SubmitCoachingSessionTurn`, after trimming `userInput`, add:

```go
submitMode := normalizeCoachingSubmitMode(req.SubmitMode)
```

Change:

```go
prompt := buildCoachingSessionTurnPrompt(plan, session, currentTask, tasks, turns, userInput)
```

to:

```go
prompt := buildCoachingSessionTurnPrompt(plan, session, currentTask, tasks, turns, userInput, submitMode)
```

Add this field to `inputSnapshot`:

```go
"submit_mode": submitMode,
```

After parsing, change:

```go
parsed, parseErr := parseCoachingSessionAgentOutput(result.Response)
```

to:

```go
parsed, parseErr := parseCoachingSessionAgentOutput(result.Response)
parsed = coerceCoachingOutputForSubmitMode(parsed, submitMode)
```

Only apply this after checking `parseErr == nil`.

- [ ] **Step 6: Replace the coaching prompt**

Change the signature:

```go
func buildCoachingSessionTurnPrompt(plan CoachingPlan, session CoachingSession, currentTask CoachingTask, tasks []CoachingTask, turns []CoachingSessionTurn, userInput string) string {
```

to:

```go
func buildCoachingSessionTurnPrompt(plan CoachingPlan, session CoachingSession, currentTask CoachingTask, tasks []CoachingTask, turns []CoachingSessionTurn, userInput string, submitMode string) string {
```

Replace the prompt string body with this Chinese-first schema and the same structured context fields already used by the current function:

```go
return fmt.Sprintf(`你是固定的 second_round_coach Agent，负责中文二面辅导长对话中的一轮回复。

只返回严格 JSON。不要返回 Markdown、代码块或 JSON 外解释。
不要写 memory_items。不要调用工具。不要直接改变业务状态；服务端会根据 state_action 持久化。

本轮 submit_mode: %s
- chat: 普通讨论、提示、解释、澄清、闲聊或导航，不记录 attempt，不更新 practice_states。
- formal_answer: 用户明确把本轮作为正式回答提交，只有这种模式才允许 record_attempt。

先判断 user_intent，再判断 state_action。不要把“下一题吧 / 继续 / 好的，下一个”误判为 skip_current；它们是 confirm_next + move_next。
只有“这题不会，跳过吧 / 放弃这题 / 这个先不练了”才是 skip_current。
低置信度时返回 unclear + chat_only + needs_clarification=true，不推进状态。

JSON schema:
{
  "visible_message": "中文用户可见回复",
  "user_intent": "answer|ask_hint|ask_explain|confirm_next|retry_current|skip_current|smalltalk|unclear|pause",
  "state_action": "chat_only|record_attempt|ask_retry|move_next|stay_current|pause|complete",
  "confidence": 0.82,
  "needs_clarification": false,
  "score": 0,
  "passed": false,
  "feedback": "中文反馈",
  "should_update_practice_state": false,
  "should_complete_current_task": false,
  "should_pause": false
}

规则:
- submit_mode=chat 时，即使用户像是在回答，也默认 state_action=chat_only，除非用户明确要求进入下一题、暂停或结束。
- submit_mode=formal_answer 且用户确实在回答当前任务时，使用 state_action=record_attempt，给 0-100 分、passed、feedback。
- smalltalk 和 unclear 必须 chat_only，不记录 attempt，不更新 practice_states。
- ask_hint 和 ask_explain 必须 chat_only，不记录 attempt，不更新 practice_states。
- confirm_next 使用 move_next，但不要标记当前任务 skipped。
- skip_current 使用 move_next，并在 visible_message 中说明这是跳过当前任务。
- 用户可见文本必须中文、自然，不暴露 input_type、next_action、state_action 等内部标签。

Coaching plan:
- plan_id: %s
- interview_id: %s
- target_round: %s
- company_name: %s
- job_title: %s

Session:
- session_id: %s
- status: %s
- current_task_id: %s
- progress_summary: %s

Current task:
- task_id: %s
- sequence: %d
- type: %s
- title: %s
- description: %s
- priority: %s

All tasks JSON:
%s

Recent turns JSON:
%s

User input:
%s`,
		submitMode,
		plan.PlanID,
		plan.InterviewID,
		plan.TargetRound,
		plan.CompanyName,
		plan.JobTitle,
		session.SessionID,
		session.Status,
		currentTask.TaskID,
		session.ProgressSummary,
		currentTask.TaskID,
		currentTask.Sequence,
		currentTask.TaskType,
		currentTask.Title,
		currentTask.Description,
		currentTask.Priority,
		string(tasksJSON),
		string(turnsJSON),
		userInput,
	)
```

- [ ] **Step 7: Update persistence logic to use `state_action`**

In `applyCoachingSessionAgentOutput`, set assistant content from `VisibleMessage`:

```go
assistantContent := parsed.VisibleMessage
if strings.TrimSpace(assistantContent) == "" {
	assistantContent = parsed.AgentMessage
}
```

Use `assistantContent` for `assistantTurn.Content` and `last_agent_message`.

Replace attempt creation condition:

```go
if inputType == CoachingInputTypeFormalAnswer {
```

with:

```go
if parsed.StateAction == CoachingStateActionRecordAttempt {
```

Replace practice update condition inside that block so it remains inside the formal attempt block and only runs when:

```go
if parsed.ShouldUpdatePracticeState {
	if err := s.runPracticeStateUpdateToolTx(tx, practiceStateUpdateToolInput{
		UserID:     session.UserID,
		Topics:     coachingTaskPracticeTopics(task),
		Score:      parsed.Score,
		Feedback:   parsed.Feedback,
		SourceType: PracticeStateSourceCoachingTaskAttempt,
		SourceID:   attempt.AttemptID,
	}); err != nil {
		return err
	}
}
```

Add a `move_next` branch before pause handling:

```go
if parsed.StateAction == CoachingStateActionMoveNext {
	if parsed.UserIntent == CoachingUserIntentSkipCurrent {
		if err := tx.Model(&CoachingTask{}).
			Where("task_id = ?", task.TaskID).
			Updates(map[string]any{"status": CoachingTaskStatusSkipped, "updated_at": now}).Error; err != nil {
			return err
		}
	} else if task.Status == CoachingTaskStatusTodo || task.Status == CoachingTaskStatusInProgress || task.Status == CoachingTaskStatusNeedsRevision {
		if err := tx.Model(&CoachingTask{}).
			Where("task_id = ?", task.TaskID).
			Updates(map[string]any{"status": CoachingTaskStatusDone, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	nextTask, hasNext, err := firstRunnableCoachingTask(tx, session.CoachingPlanID)
	if err != nil {
		return err
	}
	if hasNext {
		currentTaskID = nextTask.TaskID
		progressSummary = fmt.Sprintf("moved from task %d; next task %d: %s", task.Sequence, nextTask.Sequence, nextTask.Title)
		if nextTask.Status == CoachingTaskStatusTodo {
			if err := tx.Model(&CoachingTask{}).
				Where("task_id = ?", nextTask.TaskID).
				Updates(map[string]any{"status": CoachingTaskStatusInProgress, "updated_at": now}).Error; err != nil {
				return err
			}
		}
	} else {
		currentTaskID = ""
		nextStatus = CoachingSessionStatusCompleted
		progressSummary = "all coaching tasks completed"
		completedAt = now
	}
}
```

Remove or bypass the old `if inputType == CoachingInputTypeSkipTask` branch so `confirm_next` cannot mark the task skipped.

- [ ] **Step 8: Update trace actions**

Replace `coachingSessionTraceActions` with:

```go
func coachingSessionTraceActions(parsed coachingSessionAgentOutput) []string {
	actions := []string{
		"recorded coaching_session user turn",
		"recorded coaching_session assistant turn",
		"updated coaching_session state",
	}
	if parsed.StateAction == CoachingStateActionRecordAttempt {
		actions = append(actions, "recorded coaching_task_attempt")
		if parsed.ShouldUpdatePracticeState {
			actions = append(actions, "updated practice_states")
		} else {
			actions = append(actions, "skipped practice update")
		}
		if parsed.Passed && parsed.ShouldCompleteCurrentTask {
			actions = append(actions, "marked coaching_task done")
		}
	}
	if parsed.StateAction == CoachingStateActionMoveNext && parsed.UserIntent == CoachingUserIntentSkipCurrent {
		actions = append(actions, "marked coaching_task skipped")
	}
	if parsed.StateAction == CoachingStateActionMoveNext && parsed.UserIntent == CoachingUserIntentConfirmNext {
		actions = append(actions, "moved to next coaching_task")
	}
	if parsed.StateAction == CoachingStateActionChatOnly {
		actions = append(actions, "chat_only no attempt", "skipped practice update")
	}
	if parsed.ShouldPause || parsed.StateAction == CoachingStateActionPause {
		actions = append(actions, "paused coaching_session")
	}
	return actions
}
```

- [ ] **Step 9: Update formal-answer golden tests**

In `server/coaching_session_golden_test.go`, update every request that expects an attempt to include:

```go
SubmitMode: CoachingSubmitModeFormalAnswer,
```

For hint, explanation, skip, pause, unclear, and chat-only requests, use:

```go
SubmitMode: CoachingSubmitModeChat,
```

- [ ] **Step 10: Run coaching tests**

Run:

```bash
go test ./server -run 'Coaching|SubmitCoachingSessionTurn' -v
```

Expected: PASS.

- [ ] **Step 11: Commit**

Run:

```bash
git add server/coaching_session_service.go server/coaching_session_service_test.go server/coaching_session_golden_test.go
git commit -m "feat: separate coaching intent from state action"
```

---

### Task 4: Mock Submit Mode And Off-Record Chat Behavior

**Files:**
- Modify: `server/mock_interview_service_test.go`
- Modify: `server/mock_interview_golden_test.go`
- Modify: `server/mock_interview_service.go`

- [ ] **Step 1: Add failing mock tests**

Append these tests to `server/mock_interview_service_test.go` before helper functions:

```go
func TestSubmitMockTurn_DefaultFormalModeUpdatesPractice(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockConversationTurnJSON(MockUserIntentAnswer, MockStateActionRecordAttempt, "继续追问一致性。", 82, true),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{Answer: "我会先讲一致性边界。"}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	states, err := s.ListPracticeStates("user_001", "", "")
	if err != nil {
		t.Fatalf("ListPracticeStates() error = %v", err)
	}
	if len(states) == 0 {
		t.Fatalf("practice states length = 0, want formal answer update")
	}
}

func TestSubmitMockTurn_ChatModeIsOffRecordAndDoesNotUpdatePractice(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockConversationTurnJSON(MockUserIntentAskHint, MockStateActionChatOnly, "场外提示：可以先界定适用场景，再讲争议。现在回到刚才的问题。", 0, false),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{
		Answer:     "能场外提示一下吗？",
		SubmitMode: MockSubmitModeChat,
	})
	if err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	if !strings.Contains(turn.Content, "场外提示") {
		t.Fatalf("turn content = %q, want off-record Chinese hint", turn.Content)
	}
	assertNoPracticeStates(t, s, "user_001")
	got, err := s.GetMockInterview(mock.MockID)
	if err != nil {
		t.Fatalf("GetMockInterview() error = %v", err)
	}
	if got.CurrentTurn != 0 || got.Status != MockInterviewStatusWaitingAnswer {
		t.Fatalf("mock = %#v, want waiting without counted formal turn", got)
	}
	trace := mustFindSingleTrace(t, s, AgentDecisionTraceQuery{
		SourceType: AgentTraceSourceMockInterview,
		SourceID:   mock.MockID,
		StepName:   AgentTraceStepMockTurn,
		Status:     AgentDecisionTraceStatusSucceeded,
	})
	for _, want := range []string{`"submit_mode":"chat"`, `"user_intent":"ask_hint"`, `"state_action":"chat_only"`} {
		if !strings.Contains(trace.InputSnapshot+trace.ParsedDecision, want) {
			t.Fatalf("trace missing %s: input=%s parsed=%s", want, trace.InputSnapshot, trace.ParsedDecision)
		}
	}
	assertTraceNotContainsAction(t, trace, "updated practice_states")
}

func TestSubmitMockTurn_UnclearAndSmalltalkAreChatOnly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		intent    string
		message   string
		clarify   bool
	}{
		{name: "unclear", intent: MockUserIntentUnclear, message: "你是想回答问题，还是场外澄清？我先不计入正式回答。", clarify: true},
		{name: "smalltalk", intent: MockUserIntentSmalltalk, message: "可以，我们放慢一点。现在请继续回答刚才的问题。", clarify: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, runners := newTestServerWithFakeAgents(t)
			session, planID := createMockReadyInterview(t, s, runners)
			runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
				sampleMockStartJSON(),
				`{
  "visible_message": "` + tc.message + `",
  "user_intent": "` + tc.intent + `",
  "state_action": "` + MockStateActionChatOnly + `",
  "confidence": 0.42,
  "needs_clarification": ` + boolString(tc.clarify) + `,
  "score": 0,
  "feedback": "",
  "topic": "Redis",
  "weakness_tags": [],
  "next_action": "wait_for_answer",
  "should_update_practice_state": false,
  "practice_updates": [],
  "should_complete_mock": false,
  "follow_up_reason": ""
}`,
			}
			mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
				UserID: "user_001",
				PlanID: planID,
			})
			if err != nil {
				t.Fatalf("StartMockInterview() error = %v", err)
			}
			turn, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{
				Answer:     tc.name,
				SubmitMode: MockSubmitModeChat,
			})
			if err != nil {
				t.Fatalf("SubmitMockTurn() error = %v", err)
			}
			if !strings.Contains(turn.Content, tc.message) {
				t.Fatalf("turn content = %q, want %q", turn.Content, tc.message)
			}
			assertNoPracticeStates(t, s, "user_001")
		})
	}
}
```

Add this helper near existing sample mock JSON helpers:

```go
func sampleMockConversationTurnJSON(intent string, stateAction string, message string, score int, updatePractice bool) string {
	return `{
  "visible_message": "` + message + `",
  "user_intent": "` + intent + `",
  "state_action": "` + stateAction + `",
  "confidence": 0.9,
  "needs_clarification": false,
  "score": ` + strconv.Itoa(score) + `,
  "feedback": "` + message + `",
  "topic": "Redis 一致性",
  "weakness_tags": ["Redis"],
  "next_action": "ask_followup",
  "should_update_practice_state": ` + boolString(updatePractice) + `,
  "practice_updates": [{"topic":"Redis 一致性","score":` + strconv.Itoa(score) + `,"feedback":"` + message + `"}],
  "should_complete_mock": false,
  "follow_up_reason": "继续验证边界理解"
}`
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./server -run 'TestSubmitMockTurn_(DefaultFormalMode|ChatMode|Unclear)' -v
```

Expected: FAIL with undefined mock constants or chat-mode still updating/scoring as formal.

- [ ] **Step 3: Add mock constants and output fields**

In `server/mock_interview_service.go`, add near existing mock constants:

```go
const (
	MockSubmitModeChat         = "chat"
	MockSubmitModeFormalAnswer = "formal_answer"

	MockUserIntentAnswer       = "answer"
	MockUserIntentAskHint      = "ask_hint"
	MockUserIntentAskExplain   = "ask_explain"
	MockUserIntentConfirmNext  = "confirm_next"
	MockUserIntentRetryCurrent = "retry_current"
	MockUserIntentSkipCurrent  = "skip_current"
	MockUserIntentSmalltalk    = "smalltalk"
	MockUserIntentUnclear      = "unclear"
	MockUserIntentPause        = "pause"
	MockUserIntentEndMock      = "end_mock"

	MockStateActionChatOnly      = "chat_only"
	MockStateActionRecordAttempt = "record_attempt"
	MockStateActionAskRetry      = "ask_retry"
	MockStateActionMoveNext      = "move_next"
	MockStateActionStayCurrent   = "stay_current"
	MockStateActionPause         = "pause"
	MockStateActionComplete      = "complete"
)
```

Extend `mockTurnOutput` with these fields:

```go
VisibleMessage     string  `json:"visible_message"`
UserIntent         string  `json:"user_intent"`
StateAction        string  `json:"state_action"`
Confidence         float64 `json:"confidence"`
NeedsClarification bool    `json:"needs_clarification"`
```

Keep all existing fields for compatibility.

- [ ] **Step 4: Normalize mock submit mode and output**

Add helpers near `normalizeMockTurnOutput`:

```go
func normalizeMockSubmitMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case MockSubmitModeChat:
		return MockSubmitModeChat
	case MockSubmitModeFormalAnswer, "":
		return MockSubmitModeFormalAnswer
	default:
		return MockSubmitModeFormalAnswer
	}
}

func coerceMockOutputForSubmitMode(parsed mockTurnOutput, submitMode string) mockTurnOutput {
	if strings.TrimSpace(parsed.VisibleMessage) == "" {
		parsed.VisibleMessage = parsed.AgentMessage
	}
	if strings.TrimSpace(parsed.AgentMessage) == "" {
		parsed.AgentMessage = parsed.VisibleMessage
	}
	if strings.TrimSpace(parsed.UserIntent) == "" {
		switch parsed.InputType {
		case mockInputTypeHintRequest:
			parsed.UserIntent = MockUserIntentAskHint
		case mockInputTypeExplanationRequest:
			parsed.UserIntent = MockUserIntentAskExplain
		case mockInputTypeCancel:
			parsed.UserIntent = MockUserIntentEndMock
		default:
			parsed.UserIntent = MockUserIntentAnswer
		}
	}
	if strings.TrimSpace(parsed.StateAction) == "" {
		switch parsed.InputType {
		case mockInputTypeHintRequest, mockInputTypeExplanationRequest:
			parsed.StateAction = MockStateActionChatOnly
		case mockInputTypeCancel:
			parsed.StateAction = MockStateActionComplete
		default:
			parsed.StateAction = MockStateActionRecordAttempt
		}
	}
	if submitMode == MockSubmitModeChat && parsed.StateAction == MockStateActionRecordAttempt {
		parsed.StateAction = MockStateActionChatOnly
		parsed.ShouldUpdatePracticeState = false
	}
	if parsed.StateAction == MockStateActionChatOnly {
		if parsed.UserIntent == MockUserIntentAskExplain {
			parsed.InputType = mockInputTypeExplanationRequest
		} else {
			parsed.InputType = mockInputTypeHintRequest
		}
		parsed.NextAction = mockNextActionWaitForInput
		parsed.ShouldUpdatePracticeState = false
	}
	if parsed.StateAction == MockStateActionRecordAttempt {
		parsed.InputType = mockInputTypeFormalAnswer
	}
	if parsed.StateAction == MockStateActionComplete || parsed.UserIntent == MockUserIntentEndMock {
		parsed.InputType = mockInputTypeCancel
	}
	return parsed
}
```

- [ ] **Step 5: Thread submit mode through mock prompt and trace**

In `SubmitMockTurn`, add:

```go
submitMode := normalizeMockSubmitMode(req.SubmitMode)
```

Change:

```go
prompt := buildMockTurnPrompt(input, mock, turns, currentQuestion, req.Answer)
```

to:

```go
prompt := buildMockTurnPrompt(input, mock, turns, currentQuestion, req.Answer, submitMode)
```

Add to `inputSnapshot`:

```go
"submit_mode": submitMode,
```

After `parseMockTurnOutput`, add:

```go
parsed = coerceMockOutputForSubmitMode(parsed, submitMode)
```

- [ ] **Step 6: Replace the mock turn prompt**

Change signature:

```go
func buildMockTurnPrompt(input mockInput, mock MockInterview, turns []MockTurn, currentQuestion string, answer string) string {
```

to:

```go
func buildMockTurnPrompt(input mockInput, mock MockInterview, turns []MockTurn, currentQuestion string, answer string, submitMode string) string {
```

Include this exact Chinese-first instruction block at the top of the prompt:

```go
return fmt.Sprintf(`你是固定的 mock_interviewer Agent，正在进行中文文字模拟面试。

只返回严格 JSON。不要返回 Markdown、代码块或 JSON 外解释。
不要写 memory_items。不要创建 coaching plans。不要调用工具。

本轮 submit_mode: %s
- formal_answer: 候选人正在回答面试官问题，可以评分、追问、切换 topic，并在 should_update_practice_state=true 时让服务端更新 practice_states。
- chat: 场外提问、澄清、提示、暂停或闲聊，不评分，不更新 practice_states，不增加正式 current_turn。

先判断 user_intent，再判断 state_action。
smalltalk 和 unclear 必须 chat_only。
场外提问只短暂帮助，然后把用户带回当前面试问题。

JSON schema:
{
  "visible_message": "中文用户可见回复",
  "user_intent": "answer|ask_hint|ask_explain|confirm_next|retry_current|skip_current|smalltalk|unclear|pause|end_mock",
  "state_action": "chat_only|record_attempt|ask_retry|move_next|stay_current|pause|complete",
  "confidence": 0.82,
  "needs_clarification": false,
  "score": 72,
  "feedback": "中文反馈",
  "topic": "primary topic",
  "weakness_tags": ["string"],
  "next_action": "ask_followup|switch_topic|complete|wait_for_answer",
  "should_update_practice_state": true,
  "practice_updates": [{"topic":"string","score":72,"feedback":"string"}],
  "should_complete_mock": false,
  "follow_up_reason": "string"
}

规则:
- submit_mode=chat 必须 state_action=chat_only，should_update_practice_state=false。
- submit_mode=formal_answer 且用户在回答当前题，使用 state_action=record_attempt。
- 不要在场外提问中长篇教学，最多给简短提示或澄清。
- 用户可见文本必须中文、自然，不暴露 state_action、input_type、next_action 等内部标签。

Mock interview:
%s

Context:
%s

Review report:
%s

Structured questions:
%s

Selected memory_items:
%s

Selected practice_states:
%s

Coaching plan and tasks:
%s

Previous turns:
%s

Current interviewer question:
%s

Candidate answer:
%s`,
		submitMode,
		mockInterviewJSON(mock),
		mockSessionJSON(input.session),
		mockReportJSON(input.report),
		mockQuestionsJSON(input.questions),
		selectedMemoriesJSON(input.selection.MemoryItems),
		selectedPracticeStatesJSON(input.selection.PracticeStates),
		mockCoachingJSON(input.coachingPlan, input.coachingTasks),
		mockTurnsJSON(turns),
		currentQuestion,
		answer,
	)
```

- [ ] **Step 7: Use visible message for chat-only assistant turns**

In `SubmitMockTurn`, after parsed output coercion, set:

```go
assistantMessage := parsed.VisibleMessage
if strings.TrimSpace(assistantMessage) == "" {
	assistantMessage = parsed.AgentMessage
}
```

Use `assistantMessage` instead of `parsed.AgentMessage` for cancellation summary, hint/explanation/chat-only assistant turns, and action-turn message content.

For chat-only turns, keep the existing hint/explanation branch but ensure:

```go
updates["status"] = MockInterviewStatusWaitingAnswer
responseTurn = assistantTurn
```

Do not increment `current_turn` in this branch. Do not call `updatePracticeStatesFromMockTurnTx`.

- [ ] **Step 8: Update mock trace actions**

Update `mockTurnTraceActions` so chat-only always records skipped practice:

```go
if parsed.StateAction == MockStateActionChatOnly {
	actions = append(actions, "chat_only off_record", "skipped practice update")
}
if parsed.StateAction == MockStateActionRecordAttempt {
	if parsed.ShouldUpdatePracticeState {
		actions = append(actions, "updated practice_states")
	} else {
		actions = append(actions, "skipped practice update")
	}
}
```

Keep existing completed status action.

- [ ] **Step 9: Update mock golden tests**

In `server/mock_interview_golden_test.go`, set explicit `SubmitMode: MockSubmitModeFormalAnswer` for formal answer cases and `SubmitMode: MockSubmitModeChat` for hint/cancel/off-record cases. Add trace assertions for new parsed fields on one formal and one chat-only test:

```go
if !strings.Contains(trace.InputSnapshot+trace.ParsedDecision, `"submit_mode":"formal_answer"`) {
	t.Fatalf("trace missing submit_mode formal_answer: %#v", trace)
}
if !strings.Contains(trace.ParsedDecision, `"state_action":"record_attempt"`) {
	t.Fatalf("trace missing state_action record_attempt: %#v", trace)
}
```

- [ ] **Step 10: Run mock tests**

Run:

```bash
go test ./server -run 'Mock|SubmitMockTurn' -v
```

Expected: PASS.

- [ ] **Step 11: Commit**

Run:

```bash
git add server/mock_interview_service.go server/mock_interview_service_test.go server/mock_interview_golden_test.go
git commit -m "feat: add mock submit modes and off-record chat"
```

---

### Task 5: Preserve Memory Boundary And Trace Evaluation

**Files:**
- Modify: `server/agent_evaluation_service_test.go`
- Modify: `server/coaching_session_service_test.go`
- Modify: `server/mock_interview_service_test.go`

- [ ] **Step 1: Add boundary regression tests**

Append this test to `server/coaching_session_service_test.go`:

```go
func TestConversationalCoachingDoesNotWriteMemoryItems(t *testing.T) {
	s, runners, plan := createCoachingSessionReadyPlan(t)
	session := startTestCoachingSession(t, s, plan.PlanID)
	runners[agent.AgentTypeSecondRoundCoach].taskResponse = sampleCoachingConversationDecisionJSON(
		CoachingUserIntentSmalltalk,
		CoachingStateActionChatOnly,
		"我记住这是你担心的点，但长期记忆仍需要候选确认流程。",
		0,
		false,
		false,
	)
	if _, err := s.SubmitCoachingSessionTurn(context.Background(), session.Session.SessionID, vo.SubmitCoachingSessionTurnReq{
		UserInput:  "我以后都想重点练 Redis",
		SubmitMode: CoachingSubmitModeChat,
	}); err != nil {
		t.Fatalf("SubmitCoachingSessionTurn() error = %v", err)
	}
	items, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("memory_items length = %d, want 0 without accept/reject candidate flow", len(items))
	}
}
```

Append this test to `server/mock_interview_service_test.go`:

```go
func TestConversationalMockDoesNotWriteMemoryItems(t *testing.T) {
	s, runners := newTestServerWithFakeAgents(t)
	session, planID := createMockReadyInterview(t, s, runners)
	runners[agent.AgentTypeMockInterviewer].taskResponses = []string{
		sampleMockStartJSON(),
		sampleMockConversationTurnJSON(MockUserIntentSmalltalk, MockStateActionChatOnly, "可以，先回到当前问题。", 0, false),
	}
	mock, err := s.StartMockInterview(context.Background(), session.InterviewID, vo.StartMockInterviewReq{
		UserID: "user_001",
		PlanID: planID,
	})
	if err != nil {
		t.Fatalf("StartMockInterview() error = %v", err)
	}
	if _, err := s.SubmitMockTurn(context.Background(), mock.MockID, vo.SubmitMockTurnReq{
		Answer:     "我以后想一直练系统设计",
		SubmitMode: MockSubmitModeChat,
	}); err != nil {
		t.Fatalf("SubmitMockTurn() error = %v", err)
	}
	items, err := s.ListMemoryItems("user_001")
	if err != nil {
		t.Fatalf("ListMemoryItems() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("memory_items length = %d, want 0 without accept/reject candidate flow", len(items))
	}
}
```

- [ ] **Step 2: Run boundary tests**

Run:

```bash
go test ./server -run 'Conversational.*MemoryItems|memory_items_write_boundary' -v
```

Expected: PASS. The services should already avoid direct `memory_items` writes; failures indicate a real boundary regression.

- [ ] **Step 3: Update trace evaluation fixtures**

Update relevant fixture strings from:

```go
ParsedDecision: `{"input_type":"formal_answer","should_update_practice_state":true}`,
```

to:

```go
ParsedDecision: `{"input_type":"formal_answer","user_intent":"answer","state_action":"record_attempt","should_update_practice_state":true}`,
```

Do not weaken the `memory_items_write_boundary` check.

- [ ] **Step 4: Run evaluation tests**

Run:

```bash
go test ./server -run AgentEvaluation -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add server/coaching_session_service_test.go server/mock_interview_service_test.go server/agent_evaluation_service_test.go
git commit -m "test: preserve memory boundary for conversational turns"
```

---

### Task 6: Coaching Chat-First UI

**Files:**
- Modify: `frontend/src/pages/CoachingPage.vue`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Replace visible English labels with Chinese product labels**

In `frontend/src/pages/CoachingPage.vue`, keep the existing script state/loading functions, but replace the template with this structure:

```vue
<template>
  <section class="page conversation-page">
    <div class="conversation-topbar">
      <div>
        <span class="page-kicker">二面辅导</span>
        <h1>Coaching</h1>
        <p class="muted">{{ coachingSummaryLine }}</p>
      </div>
      <div class="topbar-actions">
        <StatusBadge :status="pageStatus" />
        <button class="secondary" type="button" @click="evidenceOpen = !evidenceOpen">
          {{ evidenceOpen ? '收起证据' : '展开证据' }}
        </button>
      </div>
    </div>

    <section class="conversation-setup panel">
      <form class="coaching-context-grid" @submit.prevent="loadInterview">
        <label>
          interview_id
          <input v-model.trim="context.interview_id" autocomplete="off" placeholder="interview_id" />
        </label>
        <label>
          轮次
          <input v-model.trim="context.target_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          剩余天数
          <input v-model.number="context.remaining_days" min="0" type="number" />
        </label>
        <label>
          user_id
          <input v-model.trim="context.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <div class="context-actions">
          <button class="secondary" type="submit" :disabled="!canUseInterview || isLoading('coachingInterview')">加载面试</button>
          <button class="primary" type="button" :disabled="!canUseInterview || isLoading('generateCoachingPlan')" @click="generatePlan">
            生成计划
          </button>
          <button class="secondary" type="button" :disabled="!canUseInterview || isLoading('getCoachingPlan')" @click="getPlan">
            读取计划
          </button>
          <button class="primary" type="button" :disabled="!selectedPlanId || !contextUserId || isLoading('startCoachingSession')" @click="startOrResumeSession">
            开始/继续辅导
          </button>
        </div>
      </form>
    </section>

    <div class="conversation-shell" :class="{ 'evidence-collapsed': !evidenceOpen }">
      <main class="chat-main panel">
        <div class="chat-summary-line">
          <strong>当前训练重点：{{ currentTask?.title || '尚未开始' }}</strong>
          <span>状态：{{ session?.status || '未开始' }}</span>
        </div>

        <div class="long-chat-stream">
          <EmptyState v-if="!chatMessages.length" title="还没有对话" message="开始辅导后，这里会显示和导师的长对话。" />
          <article v-for="message in chatMessages" :key="message.id" class="chat-bubble" :class="message.role">
            <span>{{ message.role === 'user' ? '我' : '导师' }}</span>
            <p>{{ message.content }}</p>
          </article>
        </div>

        <form class="fixed-composer" @submit.prevent="submitChatTurn">
          <textarea v-model="turnInput" rows="4" :disabled="!canSubmitTurn" placeholder="可以提问、让导师提示，或先聊清楚思路。" />
          <div class="composer-actions">
            <button class="secondary" type="button" :disabled="!canSubmitTurn || isLoading('submitCoachingTurn')" @click="submitChatTurn">
              发送
            </button>
            <button class="primary" type="button" :disabled="!canSubmitTurn || isLoading('submitCoachingTurn')" @click="submitFormalTurn">
              作为正式回答提交
            </button>
          </div>
        </form>
      </main>

      <aside v-show="evidenceOpen" class="evidence-panel">
        <details open class="evidence-section">
          <summary>训练计划 / 任务</summary>
          <p class="muted">{{ plan?.focus_summary || plan?.overall_strategy || '还没有训练计划。' }}</p>
          <div class="task-list compact">
            <article v-for="task in tasks" :key="task.task_id" class="task-card" :class="{ active: task.task_id === currentTask?.task_id }">
              <header class="task-head">
                <div>
                  <strong>#{{ task.sequence || '-' }} {{ task.title || '未命名任务' }}</strong>
                  <small>{{ task.task_type || '-' }} · day {{ task.day_index || '-' }}</small>
                </div>
                <StatusBadge :status="task.status || 'unknown'" />
              </header>
              <p>{{ task.description || '没有描述。' }}</p>
            </article>
          </div>
        </details>
        <details class="evidence-section">
          <summary>Attempts</summary>
          <EmptyState v-if="!attempts.length" title="还没有正式回答" message="点击“作为正式回答提交”后会生成 attempt。" />
          <article v-for="attempt in attempts" :key="attempt.attempt_id" class="attempt-card">
            <header class="task-head">
              <div>
                <strong>Attempt #{{ attempt.attempt_index || '-' }}</strong>
                <small>{{ attempt.attempt_id || '-' }}</small>
              </div>
              <StatusBadge :status="attempt.passed ? 'passed' : 'needs_revision'" />
            </header>
            <p>{{ attempt.feedback || '-' }}</p>
          </article>
        </details>
        <details class="evidence-section">
          <summary>Practice States</summary>
          <EmptyState v-if="!practiceStates.length" title="还没有练习状态" message="正式回答通过服务端规则更新 practice state。" />
          <article v-for="item in practiceStates" :key="item.state_id || `${item.topic}-${item.dimension}`" class="practice-card">
            <strong>{{ item.topic || '未命名 topic' }}</strong>
            <p>{{ item.last_feedback || '暂无反馈。' }}</p>
            <small>{{ item.source_type || '-' }} · {{ item.source_id || '-' }}</small>
          </article>
        </details>
        <details class="evidence-section">
          <summary>Trace / Timeline</summary>
          <RouterLink v-if="selectedSessionId" class="text-link" :to="traceLink">打开 Trace</RouterLink>
          <div class="timeline compact">
            <TurnTimelineItem v-for="turn in turns" :key="turn.turn_id" :turn="turn" kind="coaching" />
          </div>
        </details>
      </aside>
    </div>
  </section>
</template>
```

- [ ] **Step 2: Add chat computed state and submit methods**

In the `<script setup>` of `CoachingPage.vue`, add:

```js
const evidenceOpen = ref(false)

const coachingSummaryLine = computed(() => {
  const focus = currentTask.value?.title || plan.value?.focus_summary || '尚未加载训练重点'
  const status = session.value?.status || '未开始'
  return `当前训练重点：${focus} / 状态：${status}`
})

const chatMessages = computed(() =>
  turns.value
    .filter((turn) => ['user', 'assistant'].includes(String(turn.role || '').toLowerCase()))
    .map((turn) => ({
      id: turn.turn_id,
      role: String(turn.role || '').toLowerCase() === 'user' ? 'user' : 'assistant',
      content: turn.content || turn.feedback || '',
    }))
    .filter((message) => message.content.trim()),
)
```

Replace `submitTurn` with:

```js
async function submitWithMode(submitMode) {
  const text = turnInput.value.trim()
  if (!text || !selectedSessionId.value) return
  const loaded = await runWithStatus(
    'submitCoachingTurn',
    () => api.submitCoachingTurn(selectedSessionId.value, { user_input: text, submit_mode: submitMode }),
    submitMode === 'formal_answer' ? '正式回答已提交' : '消息已发送',
  )
  turnInput.value = ''
  syncSessionDetail(loaded)
  await loadPracticeStates()
}

async function submitChatTurn() {
  await submitWithMode('chat')
}

async function submitFormalTurn() {
  await submitWithMode('formal_answer')
}
```

Remove or stop using the old `submitTurn` button handler.

- [ ] **Step 3: Add shared conversation CSS**

Append to `frontend/src/styles.css`:

```css
.conversation-page {
  display: grid;
  gap: 14px;
}

.conversation-topbar {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.topbar-actions {
  align-items: center;
  display: flex;
  gap: 8px;
}

.conversation-shell {
  align-items: start;
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(0, 1fr) 360px;
}

.conversation-shell.evidence-collapsed {
  grid-template-columns: minmax(0, 1fr);
}

.chat-main {
  min-height: 680px;
  padding: 0;
}

.chat-summary-line {
  align-items: center;
  border-bottom: 1px solid #dfe6ef;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  padding: 12px 14px;
}

.long-chat-stream {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 560px;
  min-height: 420px;
  overflow-y: auto;
  padding: 16px;
}

.chat-bubble {
  border: 1px solid #dfe6ef;
  border-radius: 8px;
  display: grid;
  gap: 5px;
  max-width: 76%;
  padding: 10px 12px;
}

.chat-bubble span {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
}

.chat-bubble p {
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.chat-bubble.user {
  align-self: flex-end;
  background: #eef4ff;
  border-color: #bfd0f3;
}

.chat-bubble.assistant {
  align-self: flex-start;
  background: #ffffff;
}

.fixed-composer {
  border-top: 1px solid #dfe6ef;
  display: grid;
  gap: 10px;
  padding: 12px;
}

.composer-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.evidence-panel {
  display: grid;
  gap: 10px;
}

.evidence-section {
  background: #ffffff;
  border: 1px solid #d7dde6;
  border-radius: 8px;
  padding: 10px;
}

.evidence-section > summary {
  cursor: pointer;
  font-weight: 700;
}
```

- [ ] **Step 4: Run frontend build**

Run:

```bash
cd frontend && npm run build
```

Expected: PASS.

- [ ] **Step 5: Manual UI acceptance**

Run:

```bash
cd frontend && npm run dev
```

Open `http://127.0.0.1:5173/coaching`. Verify:

- The main visual area is a long chat.
- Internal labels such as `skip_task`, `formal_answer`, `prompt_next_task`, and `continue_current_task` do not appear in chat bubbles.
- Evidence sections are secondary/collapsible.
- `发送` posts `{ user_input, submit_mode: "chat" }`.
- `作为正式回答提交` posts `{ user_input, submit_mode: "formal_answer" }`.

- [ ] **Step 6: Commit**

Run:

```bash
git add frontend/src/pages/CoachingPage.vue frontend/src/styles.css
git commit -m "feat: make coaching chat first"
```

---

### Task 7: Mock Chat-First UI

**Files:**
- Modify: `frontend/src/pages/MockInterviewPage.vue`
- Modify: `frontend/src/styles.css`
- Modify: `frontend/src/api.test.mjs`

- [ ] **Step 1: Refactor mock page template to conversation-first**

In `frontend/src/pages/MockInterviewPage.vue`, keep current script data loaders, but replace the template with the same layout pattern as Coaching:

```vue
<template>
  <section class="page conversation-page">
    <div class="conversation-topbar">
      <div>
        <span class="page-kicker">模拟面试</span>
        <h1>Mock Interview</h1>
        <p class="muted">{{ mockSummaryLine }}</p>
      </div>
      <div class="topbar-actions">
        <StatusBadge :status="pageStatus" />
        <button class="secondary" type="button" @click="evidenceOpen = !evidenceOpen">
          {{ evidenceOpen ? '收起证据' : '展开证据' }}
        </button>
      </div>
    </div>

    <section class="conversation-setup panel">
      <form class="mock-context-grid" @submit.prevent="loadInterview">
        <label>
          interview_id
          <input v-model.trim="context.interview_id" autocomplete="off" placeholder="interview_id" @change="rememberInterview" />
        </label>
        <label>
          plan_id
          <input v-model.trim="context.plan_id" autocomplete="off" placeholder="optional plan_id" @change="rememberPlan" />
        </label>
        <label>
          轮次
          <input v-model.trim="context.target_round" autocomplete="off" placeholder="second_round" />
        </label>
        <label>
          user_id
          <input v-model.trim="context.user_id" autocomplete="off" placeholder="user_001" />
        </label>
        <div class="context-actions">
          <button class="secondary" type="submit" :disabled="!canUseInterview || isLoading('mockInterviewDetail')">加载面试</button>
          <button class="primary" type="button" :disabled="!canUseInterview || isLoading('startMockInterview')" @click="startOrResumeMock">
            开始/继续 Mock
          </button>
          <button class="secondary" type="button" :disabled="!selectedMockId || isLoading('getMockInterview')" @click="refreshMock">
            刷新
          </button>
        </div>
      </form>
    </section>

    <div class="conversation-shell" :class="{ 'evidence-collapsed': !evidenceOpen }">
      <main class="chat-main panel">
        <div class="chat-summary-line">
          <strong>当前题目：{{ currentQuestion }}</strong>
          <span>状态：{{ mock?.status || '未开始' }}</span>
        </div>

        <div class="long-chat-stream">
          <EmptyState v-if="!chatMessages.length" title="还没有对话" message="开始 Mock 后，这里会显示面试官和你的长对话。" />
          <article v-for="message in chatMessages" :key="message.id" class="chat-bubble" :class="message.role">
            <span>{{ message.role === 'user' ? '我' : '面试官' }}</span>
            <p>{{ message.content }}</p>
          </article>
        </div>

        <form class="fixed-composer" @submit.prevent="submitFormalAnswer">
          <textarea v-model="answerInput" rows="4" :disabled="!canAnswerMock" placeholder="默认作为正式回答提交。需要提示或澄清时，用场外提问。" />
          <div class="composer-actions">
            <button class="primary" type="button" :disabled="!canSubmitTurn || isLoading('submitMockTurn')" @click="submitFormalAnswer">
              回答
            </button>
            <button class="secondary" type="button" :disabled="!canSubmitTurn || isLoading('submitMockTurn')" @click="submitOffRecordQuestion">
              场外提问
            </button>
          </div>
        </form>
      </main>

      <aside v-show="evidenceOpen" class="evidence-panel">
        <details open class="evidence-section">
          <summary>面试状态</summary>
          <dl class="field-stack">
            <div class="field-row">
              <dt>mock_id</dt>
              <dd>{{ mock?.mock_id || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>当前 topic</dt>
              <dd>{{ mock?.current_topic || '-' }}</dd>
            </div>
            <div class="field-row">
              <dt>正式轮次</dt>
              <dd>{{ mock?.current_turn ?? '-' }}</dd>
            </div>
          </dl>
        </details>
        <details class="evidence-section">
          <summary>最近反馈</summary>
          <EmptyState v-if="!latestResultTurn && !mock?.last_feedback && !mock?.final_summary" title="还没有反馈" message="提交回答后会显示最近反馈。" />
          <article v-else class="result-card">
            <StatusBadge :status="latestResultStatus" />
            <p>{{ latestFeedback }}</p>
            <small>score {{ latestResultTurn?.score ?? '-' }} · {{ latestNextAction }}</small>
          </article>
        </details>
        <details class="evidence-section">
          <summary>Practice States</summary>
          <EmptyState v-if="!practiceStates.length" title="还没有练习状态" message="正式回答可能更新 practice state。" />
          <article v-for="item in practiceStates" :key="item.state_id || `${item.topic}-${item.dimension}`" class="practice-card">
            <strong>{{ item.topic || '未命名 topic' }}</strong>
            <p>{{ item.last_feedback || '暂无反馈。' }}</p>
            <small>{{ item.source_type || '-' }} · {{ item.source_id || '-' }}</small>
          </article>
        </details>
        <details class="evidence-section">
          <summary>Trace / Timeline</summary>
          <RouterLink v-if="selectedMockId" class="text-link" :to="traceLink">打开 Trace</RouterLink>
          <div class="timeline compact">
            <article v-for="turn in timeline" :key="turn.turn_id || `opening-${turn.turn_index}`" class="timeline-entry">
              <TurnTimelineItem :turn="turn" kind="mock" />
            </article>
          </div>
        </details>
      </aside>
    </div>
  </section>
</template>
```

- [ ] **Step 2: Add mock chat computed state and submit methods**

Add to the script:

```js
const evidenceOpen = ref(false)

const mockSummaryLine = computed(() => {
  const question = currentQuestion.value && currentQuestion.value !== '-' ? currentQuestion.value : '尚未开始'
  const status = mock.value?.status || '未开始'
  return `当前题目：${question} / 状态：${status}`
})

const chatMessages = computed(() =>
  timeline.value
    .filter((turn) => ['user', 'assistant'].includes(String(turn.role || '').toLowerCase()))
    .map((turn) => ({
      id: turn.turn_id || `turn-${turn.turn_index}-${turn.role}`,
      role: String(turn.role || '').toLowerCase() === 'user' ? 'user' : 'assistant',
      content: turn.content || turn.next_question || turn.feedback || turn.interviewer_question || '',
    }))
    .filter((message) => message.content.trim()),
)
```

Replace `submitTurn` with:

```js
async function submitWithMode(submitMode) {
  if (!canSubmitTurn.value) return
  await runWithStatus(
    'submitMockTurn',
    () => api.submitMockTurn(mock.value.mock_id, { answer: answerInput.value.trim(), submit_mode: submitMode }),
    submitMode === 'chat' ? '场外提问已发送' : '回答已提交',
  )
  answerInput.value = ''
  await refreshMock()
}

async function submitFormalAnswer() {
  await submitWithMode('formal_answer')
}

async function submitOffRecordQuestion() {
  await submitWithMode('chat')
}
```

- [ ] **Step 3: Run frontend tests and build**

Run:

```bash
cd frontend && npm run test:api
cd frontend && npm run build
```

Expected: PASS.

- [ ] **Step 4: Manual UI acceptance**

Run:

```bash
cd frontend && npm run dev
```

Open `http://127.0.0.1:5173/mock`. Verify:

- The main visual area is a chat with interviewer/user bubbles.
- `回答` sends `{ answer, submit_mode: "formal_answer" }`.
- `场外提问` sends `{ answer, submit_mode: "chat" }`.
- Task state, practice states, attempts/feedback, timeline, and trace are visible only in secondary evidence sections.
- Internal labels such as `ask_followup`, `switch_topic`, `formal_answer`, and `hint_request` do not appear in the main chat bubbles.

- [ ] **Step 5: Commit**

Run:

```bash
git add frontend/src/pages/MockInterviewPage.vue frontend/src/styles.css frontend/src/api.test.mjs
git commit -m "feat: make mock interview chat first"
```

---

### Task 8: Final Verification And Product Acceptance

**Files:**
- No code changes unless verification exposes a defect.

- [ ] **Step 1: Run full backend tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend tests and build**

Run:

```bash
cd frontend && npm run test:api
cd frontend && npm run build
```

Expected: PASS.

- [ ] **Step 3: Check generated/local artifacts are ignored**

Run:

```bash
git status --short --ignored
```

Expected:

- Source changes are either committed or intentionally staged for the final commit.
- Ignored output may include `!! frontend/dist/`, `!! frontend/node_modules/`, `!! agent-web-base.db`, `!! .env`, `!! config.json`, `!! .superpowers/`, and local media files.

- [ ] **Step 4: Verify acceptance criteria manually**

Use the running app and a fake or real LLM configuration to verify:

- Coaching main area reads as a Chinese mentor conversation.
- Mock main area reads as a Chinese interviewer conversation.
- Top summary bar shows current focus/question and status.
- Evidence panel is secondary/collapsible.
- Coaching `发送` does not create attempts or practice state.
- Coaching `作为正式回答提交` creates a coaching attempt and updates practice state when the model returns `record_attempt`.
- Mock `回答` defaults to formal answer mode and can update practice state.
- Mock `场外提问` does not update practice state or increment formal current turn.
- `好的，下一题吧` maps to `confirm_next/move_next` and does not mark the task skipped.
- `我有点紧张` and ambiguous messages stay chat-only.
- No service action or code path writes `memory_items` directly from coaching/mock turns.

- [ ] **Step 5: Final polish commit**

If final verification produced source fixes, run:

```bash
git add .
git commit -m "fix: polish conversational agent experience"
```

Otherwise keep the task commits as the final branch state.

---

## Self-Review

Spec coverage:

- Git baseline and `.gitignore`: Task 1.
- Chat-first Coaching/Mock UI: Tasks 6 and 7.
- Hybrid submit mode: Tasks 2, 3, 4, 6, and 7.
- Backend `submit_mode`: Tasks 2, 3, and 4.
- Chinese-first prompts: Tasks 3 and 4.
- Separate `user_intent` and `state_action`: Tasks 3 and 4.
- `smalltalk` and `unclear` fallback: Tasks 3 and 4.
- `下一题吧` not treated as skip: Task 3.
- Memory boundary preserved: Task 5 and Task 8.
- Backend behavior tests before implementation: Tasks 3, 4, and 5 start with failing tests.
- No new business Agent, no ReAct/MCP/function calling, no database redesign: enforced in file structure and task constraints.

Placeholder scan:

- No `TBD`, no deferred unspecified implementation, no broad “add tests” steps without concrete tests.

Type consistency:

- `submit_mode` is the JSON field in VO and frontend.
- Coaching modes use `CoachingSubmitModeChat` and `CoachingSubmitModeFormalAnswer`.
- Mock modes use `MockSubmitModeChat` and `MockSubmitModeFormalAnswer`.
- New parsed fields are consistently named `visible_message`, `user_intent`, `state_action`, `confidence`, and `needs_clarification`.
