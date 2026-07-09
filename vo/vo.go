package vo

// R 是统一的 JSON 响应包装
type R struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func OK(data any) R {
	return R{Code: 0, Msg: "ok", Data: data}
}

func Err(code int, msg string) R {
	return R{Code: code, Msg: msg}
}

// CreateConversationReq POST /conversation 请求体
type CreateConversationReq struct {
	UserID string `json:"user_id" binding:"required"`
	Title  string `json:"title"`
}

// UpdateConversationReq PATCH /conversation/{id} 请求体
type UpdateConversationReq struct {
	Title string `json:"title" binding:"required"`
}

// CreateMessageReq POST /conversation/{id}/message 请求体
type CreateMessageReq struct {
	UserID          string `json:"user_id" binding:"required"`
	Query           string `json:"query" binding:"required"`
	ParentMessageID string `json:"parent_message_id"`
	AgentType       string `json:"agent_type,omitempty"`
}

// ConversationVO GET /conversation 列表项
type ConversationVO struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Title          string `json:"title"`
	CreatedAt      int64  `json:"created_at"`
}

// RoundMessageVO 是一条 LLM round 消息的精简视图
type RoundMessageVO struct {
	Role      string       `json:"role"`                 // user / assistant / tool
	Content   string       `json:"content,omitempty"`    // 文本内容
	ToolCalls []ToolCallVO `json:"tool_calls,omitempty"` // assistant 发起的 tool call
	ToolName  string       `json:"tool_name,omitempty"`  // tool 消息的工具名
	ToolID    string       `json:"tool_id,omitempty"`    // tool 消息对应的 call_id
}

// ToolCallVO 是一次 tool call 的精简视图
type ToolCallVO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatMessageVO GET /conversation/{id}/message 列表项
type ChatMessageVO struct {
	MessageID       string           `json:"message_id"`
	ConversationID  string           `json:"conversation_id"`
	ParentMessageID string           `json:"parent_message_id"`
	AgentType       string           `json:"agent_type"`
	Query           string           `json:"query"`
	Response        string           `json:"response"`
	Model           string           `json:"model"`
	CreatedAt       int64            `json:"created_at"`
	Rounds          []RoundMessageVO `json:"rounds,omitempty"`
}

// CreateInterviewReq POST /interviews 请求体
type CreateInterviewReq struct {
	UserID         string `json:"user_id" binding:"required"`
	CompanyName    string `json:"company_name" binding:"required"`
	JobTitle       string `json:"job_title"`
	InterviewRound string `json:"interview_round"`
	InterviewType  string `json:"interview_type"`
	OccurredAt     int64  `json:"occurred_at,omitempty"`
}

// UpsertInterviewTranscriptReq PUT /interviews/{id}/transcript 请求体
type UpsertInterviewTranscriptReq struct {
	UserID     string `json:"user_id" binding:"required"`
	Content    string `json:"content" binding:"required"`
	SourceType string `json:"source_type,omitempty"`
	Language   string `json:"language,omitempty"`
}

// InterviewSessionVO 是一场面试记录的响应结构
type InterviewSessionVO struct {
	InterviewID    string `json:"interview_id"`
	UserID         string `json:"user_id"`
	CompanyName    string `json:"company_name"`
	JobTitle       string `json:"job_title"`
	InterviewRound string `json:"interview_round"`
	InterviewType  string `json:"interview_type"`
	Status         string `json:"status"`
	OccurredAt     int64  `json:"occurred_at,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// InterviewTranscriptVO 是面试转写文本的响应结构
type InterviewTranscriptVO struct {
	TranscriptID string `json:"transcript_id"`
	InterviewID  string `json:"interview_id"`
	UserID       string `json:"user_id"`
	SourceType   string `json:"source_type"`
	Content      string `json:"content"`
	Language     string `json:"language"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// MediaFileVO 是用户上传的面试音视频文件记录。
type MediaFileVO struct {
	MediaID          string `json:"media_id"`
	InterviewID      string `json:"interview_id"`
	UserID           string `json:"user_id"`
	OriginalFilename string `json:"original_filename"`
	StoredFilename   string `json:"stored_filename"`
	StoragePath      string `json:"storage_path"`
	ContentType      string `json:"content_type"`
	MediaType        string `json:"media_type"`
	FileExt          string `json:"file_ext"`
	SizeBytes        int64  `json:"size_bytes"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

// TranscriptionJobVO 是异步转写任务记录。
type TranscriptionJobVO struct {
	JobID              string `json:"job_id"`
	MediaID            string `json:"media_id"`
	InterviewID        string `json:"interview_id"`
	UserID             string `json:"user_id"`
	Status             string `json:"status"`
	InputMediaPath     string `json:"input_media_path"`
	ExtractedAudioPath string `json:"extracted_audio_path,omitempty"`
	ASRProvider        string `json:"asr_provider"`
	ASRModel           string `json:"asr_model"`
	Language           string `json:"language"`
	TranscriptID       string `json:"transcript_id,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
	StartedAt          int64  `json:"started_at,omitempty"`
	FinishedAt         int64  `json:"finished_at,omitempty"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

// UploadInterviewMediaVO 是上传接口返回的数据。
type UploadInterviewMediaVO struct {
	MediaFile        MediaFileVO        `json:"media_file"`
	TranscriptionJob TranscriptionJobVO `json:"transcription_job"`
}

// InterviewDetailVO 是面试详情页的只读聚合视图。
type InterviewDetailVO struct {
	Interview           InterviewSessionVO       `json:"interview"`
	Transcript          *InterviewTranscriptVO   `json:"transcript,omitempty"`
	MediaFiles          []MediaFileVO            `json:"media_files"`
	TranscriptionJobs   []TranscriptionJobVO     `json:"transcription_jobs"`
	ReviewReport        *InterviewReviewReportVO `json:"review_report,omitempty"`
	Questions           []InterviewQuestionVO    `json:"questions"`
	MemoryCandidates    []MemoryCandidateVO      `json:"memory_candidates"`
	CoachingPlan        *CoachingPlanVO          `json:"coaching_plan,omitempty"`
	CoachingTasks       []CoachingTaskVO         `json:"coaching_tasks"`
	LatestMockInterview *MockInterviewVO         `json:"latest_mock_interview,omitempty"`
}

// InterviewQuestionVO 是从面试转写中抽取的结构化问答
type InterviewQuestionVO struct {
	QuestionID            string   `json:"question_id"`
	InterviewID           string   `json:"interview_id"`
	UserID                string   `json:"user_id"`
	Sequence              int      `json:"sequence"`
	Question              string   `json:"question"`
	Answer                string   `json:"answer"`
	TopicTags             []string `json:"topic_tags"`
	Difficulty            string   `json:"difficulty"`
	AnswerQuality         string   `json:"answer_quality"`
	WeaknessSummary       string   `json:"weakness_summary"`
	ImprovementSuggestion string   `json:"improvement_suggestion"`
	EvidenceText          string   `json:"evidence_text,omitempty"`
	CreatedAt             int64    `json:"created_at"`
	UpdatedAt             int64    `json:"updated_at"`
}

// InterviewReviewReportVO 是一场面试的整体复盘报告
type InterviewReviewReportVO struct {
	ReportID             string   `json:"report_id"`
	InterviewID          string   `json:"interview_id"`
	UserID               string   `json:"user_id"`
	OverallSummary       string   `json:"overall_summary"`
	Strengths            []string `json:"strengths"`
	Weaknesses           []string `json:"weaknesses"`
	FollowUpRisks        []string `json:"follow_up_risks"`
	SuggestedPreparation []string `json:"suggested_preparation"`
	RawAgentOutput       string   `json:"raw_agent_output,omitempty"`
	Status               string   `json:"status"`
	CreatedAt            int64    `json:"created_at"`
	UpdatedAt            int64    `json:"updated_at"`
}

// TranscriptSegmentVO exposes read-only segment extraction state for debugging long transcript review.
type TranscriptSegmentVO struct {
	SegmentID          string `json:"segment_id"`
	InterviewID        string `json:"interview_id"`
	TranscriptID       string `json:"transcript_id"`
	UserID             string `json:"user_id"`
	Sequence           int    `json:"sequence"`
	StartOffset        int    `json:"start_offset"`
	EndOffset          int    `json:"end_offset"`
	CharCount          int    `json:"char_count"`
	ContentPreview     string `json:"content_preview,omitempty"`
	Summary            string `json:"summary"`
	SpeakerRoleNotes   any    `json:"speaker_role_notes"`
	QuestionCandidates any    `json:"question_candidates"`
	KeyEvidence        any    `json:"key_evidence"`
	UncertainParts     any    `json:"uncertain_parts"`
	Status             string `json:"status"`
	ErrorMessage       string `json:"error_message,omitempty"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

// MemoryCandidateVO 是等待用户确认的长期记忆候选项
type MemoryCandidateVO struct {
	CandidateID    string `json:"candidate_id"`
	UserID         string `json:"user_id"`
	InterviewID    string `json:"interview_id"`
	MemoryType     string `json:"memory_type"`
	SubjectKey     string `json:"subject_key"`
	Content        string `json:"content"`
	Evidence       string `json:"evidence"`
	Confidence     string `json:"confidence"`
	Status         string `json:"status"`
	Source         string `json:"source"`
	SourceRefType  string `json:"source_ref_type,omitempty"`
	SourceRefID    string `json:"source_ref_id,omitempty"`
	RawAgentOutput string `json:"raw_agent_output,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// MemoryItemVO 是用户确认后的结构化长期记忆
type MemoryItemVO struct {
	MemoryID          string `json:"memory_id"`
	UserID            string `json:"user_id"`
	MemoryType        string `json:"memory_type"`
	SubjectKey        string `json:"subject_key"`
	Content           string `json:"content"`
	Evidence          string `json:"evidence"`
	Confidence        string `json:"confidence"`
	SourceCandidateID string `json:"source_candidate_id"`
	SourceInterviewID string `json:"source_interview_id"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

// GenerateCoachingPlanReq POST /interviews/{id}/coaching-plan 请求体
type GenerateCoachingPlanReq struct {
	UserID        string `json:"user_id" binding:"required"`
	TargetRound   string `json:"target_round"`
	RemainingDays int    `json:"remaining_days"`
}

type GeneratePracticeGoalCoachingPlanReq struct {
	UserID        string `json:"user_id" binding:"required"`
	TargetRound   string `json:"target_round"`
	RemainingDays int    `json:"remaining_days"`
}

// UpdateCoachingTaskReq PATCH /coaching-tasks/{id} 请求体
type UpdateCoachingTaskReq struct {
	Status string `json:"status" binding:"required"`
}

// CreatePracticeGoalReq POST /practice-goals 请求体
type CreatePracticeGoalReq struct {
	UserID         string   `json:"user_id" binding:"required"`
	CompanyName    string   `json:"company_name"`
	JobTitle       string   `json:"job_title"`
	TargetRound    string   `json:"target_round"`
	JobDescription string   `json:"job_description"`
	FocusTopics    []string `json:"focus_topics"`
	RemainingDays  int      `json:"remaining_days"`
}

// UpdatePracticeGoalReq PATCH /practice-goals/{id} 请求体
type UpdatePracticeGoalReq struct {
	CompanyName    string   `json:"company_name"`
	JobTitle       string   `json:"job_title"`
	TargetRound    string   `json:"target_round"`
	JobDescription string   `json:"job_description"`
	FocusTopics    []string `json:"focus_topics"`
	RemainingDays  int      `json:"remaining_days"`
	Status         string   `json:"status"`
}

// PracticeGoalVO 是独立练习目标，不绑定 interview_id。
type PracticeGoalVO struct {
	GoalID         string   `json:"goal_id"`
	UserID         string   `json:"user_id"`
	InterviewID    string   `json:"interview_id"`
	CompanyName    string   `json:"company_name"`
	JobTitle       string   `json:"job_title"`
	TargetRound    string   `json:"target_round"`
	JobDescription string   `json:"job_description"`
	FocusTopics    []string `json:"focus_topics"`
	RemainingDays  int      `json:"remaining_days"`
	Status         string   `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

// CoachingPlanVO 是二面准备计划
type CoachingPlanVO struct {
	PlanID          string `json:"plan_id"`
	UserID          string `json:"user_id"`
	InterviewID     string `json:"interview_id"`
	PracticeGoalID  string `json:"practice_goal_id,omitempty"`
	SourceType      string `json:"source_type"`
	TargetRound     string `json:"target_round"`
	RemainingDays   int    `json:"remaining_days"`
	CompanyName     string `json:"company_name"`
	JobTitle        string `json:"job_title"`
	OverallStrategy string `json:"overall_strategy"`
	FocusSummary    string `json:"focus_summary"`
	RawAgentOutput  string `json:"raw_agent_output,omitempty"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// CoachingTaskVO 是二面准备计划中的任务
type CoachingTaskVO struct {
	TaskID           string   `json:"task_id"`
	PlanID           string   `json:"plan_id"`
	UserID           string   `json:"user_id"`
	InterviewID      string   `json:"interview_id"`
	PracticeGoalID   string   `json:"practice_goal_id,omitempty"`
	SourceType       string   `json:"source_type"`
	Sequence         int      `json:"sequence"`
	DayIndex         int      `json:"day_index"`
	TaskType         string   `json:"task_type"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	RelatedMemoryIDs []string `json:"related_memory_ids"`
	Priority         string   `json:"priority"`
	Status           string   `json:"status"`
	CreatedAt        int64    `json:"created_at"`
	UpdatedAt        int64    `json:"updated_at"`
}

// SubmitCoachingSessionTurnReq POST /coaching-sessions/{id}/turns 请求体
type SubmitCoachingSessionTurnReq struct {
	UserInput  string `json:"user_input" binding:"required"`
	SubmitMode string `json:"submit_mode,omitempty"`
}

// CoachingSessionVO 是 plan 级别二面辅导长会话
type CoachingSessionVO struct {
	SessionID            string `json:"session_id"`
	UserID               string `json:"user_id"`
	InterviewID          string `json:"interview_id"`
	PracticeGoalID       string `json:"practice_goal_id,omitempty"`
	CoachingPlanID       string `json:"coaching_plan_id"`
	CurrentTaskID        string `json:"current_task_id,omitempty"`
	Status               string `json:"status"`
	ProgressSummary      string `json:"progress_summary"`
	LastAgentMessage     string `json:"last_agent_message"`
	ErrorMessage         string `json:"error_message,omitempty"`
	FailedRetryCount     int    `json:"failed_retry_count,omitempty"`
	StartedAt            int64  `json:"started_at,omitempty"`
	LastActiveAt         int64  `json:"last_active_at,omitempty"`
	CompletedAt          int64  `json:"completed_at,omitempty"`
	CreatedAt            int64  `json:"created_at"`
	UpdatedAt            int64  `json:"updated_at"`
	AgentPersistentState any    `json:"agent_persistent_state,omitempty"`
}

// CoachingSessionTurnVO 是二面辅导会话中的一轮交互或状态记录
type CoachingSessionTurnVO struct {
	TurnID         string `json:"turn_id"`
	SessionID      string `json:"session_id"`
	CoachingPlanID string `json:"coaching_plan_id"`
	CoachingTaskID string `json:"coaching_task_id,omitempty"`
	Role           string `json:"role"`
	TurnType       string `json:"turn_type"`
	Content        string `json:"content"`
	AgentAction    string `json:"agent_action,omitempty"`
	Score          int    `json:"score,omitempty"`
	Feedback       string `json:"feedback,omitempty"`
	RawAgentOutput string `json:"raw_agent_output,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

// CoachingTaskAttemptVO 是当前 session 内针对某个 task 的正式回答尝试
type CoachingTaskAttemptVO struct {
	AttemptID      string `json:"attempt_id"`
	SessionID      string `json:"session_id"`
	CoachingTaskID string `json:"coaching_task_id"`
	UserAnswer     string `json:"user_answer"`
	Score          int    `json:"score"`
	Feedback       string `json:"feedback"`
	Passed         bool   `json:"passed"`
	AttemptIndex   int    `json:"attempt_index"`
	RawAgentOutput string `json:"raw_agent_output,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

// CoachingSessionDetailVO 聚合 session 当前状态、任务、轮次和尝试记录
type CoachingSessionDetailVO struct {
	Session     CoachingSessionVO       `json:"session"`
	CurrentTask *CoachingTaskVO         `json:"current_task,omitempty"`
	Tasks       []CoachingTaskVO        `json:"tasks"`
	Turns       []CoachingSessionTurnVO `json:"turns"`
	Attempts    []CoachingTaskAttemptVO `json:"attempts"`
}

// StartMockInterviewReq POST /interviews/{id}/mock-interviews 请求体
type StartMockInterviewReq struct {
	UserID      string `json:"user_id" binding:"required"`
	PlanID      string `json:"plan_id,omitempty"`
	TargetRound string `json:"target_round"`
	FocusTopic  string `json:"focus_topic,omitempty"`
}

type StartPracticeGoalMockReq struct {
	UserID      string `json:"user_id" binding:"required"`
	PlanID      string `json:"plan_id,omitempty"`
	TargetRound string `json:"target_round"`
	FocusTopic  string `json:"focus_topic,omitempty"`
}

// SubmitMockTurnReq POST /mock-interviews/{id}/turns 请求体
type SubmitMockTurnReq struct {
	Answer     string `json:"answer"`
	SubmitMode string `json:"submit_mode,omitempty"`
	Trigger    string `json:"trigger,omitempty"`
}

// MockInterviewVO 是一次模拟面试会话
type MockInterviewVO struct {
	MockID               string `json:"mock_id"`
	UserID               string `json:"user_id"`
	InterviewID          string `json:"interview_id"`
	PracticeGoalID       string `json:"practice_goal_id,omitempty"`
	PlanID               string `json:"plan_id,omitempty"`
	TargetRound          string `json:"target_round"`
	Status               string `json:"status"`
	CurrentTurn          int    `json:"current_turn"`
	CurrentTopic         string `json:"current_topic,omitempty"`
	OverallGoal          string `json:"overall_goal"`
	FirstQuestion        string `json:"first_question"`
	LastFeedback         string `json:"last_feedback,omitempty"`
	ErrorMessage         string `json:"error_message,omitempty"`
	FailedRetryCount     int    `json:"failed_retry_count,omitempty"`
	FinalSummary         string `json:"final_summary,omitempty"`
	RawAgentOutput       string `json:"raw_agent_output,omitempty"`
	CreatedAt            int64  `json:"created_at"`
	UpdatedAt            int64  `json:"updated_at"`
	AgentPersistentState any    `json:"agent_persistent_state,omitempty"`
}

// MockTurnVO 是模拟面试的一轮问答和反馈
type MockTurnVO struct {
	TurnID              string   `json:"turn_id"`
	MockID              string   `json:"mock_id"`
	UserID              string   `json:"user_id"`
	InterviewID         string   `json:"interview_id"`
	PracticeGoalID      string   `json:"practice_goal_id,omitempty"`
	TurnIndex           int      `json:"turn_index"`
	Role                string   `json:"role,omitempty"`
	TurnType            string   `json:"turn_type,omitempty"`
	Phase               string   `json:"phase,omitempty"`
	AgentAction         string   `json:"agent_action,omitempty"`
	Content             string   `json:"content,omitempty"`
	InterviewerQuestion string   `json:"interviewer_question"`
	UserAnswer          string   `json:"user_answer"`
	Feedback            string   `json:"feedback"`
	Score               int      `json:"score"`
	FollowUpReason      string   `json:"follow_up_reason"`
	TimeLimitSeconds    int      `json:"time_limit_seconds,omitempty"`
	TimePressureStyle   string   `json:"time_pressure_style,omitempty"`
	WarnAtSeconds       int      `json:"warn_at_seconds,omitempty"`
	TopicTags           []string `json:"topic_tags"`
	NextQuestion        string   `json:"next_question"`
	RawAgentOutput      string   `json:"raw_agent_output,omitempty"`
	ErrorMessage        string   `json:"error_message,omitempty"`
	CreatedAt           int64    `json:"created_at"`
	UpdatedAt           int64    `json:"updated_at"`
}

// PracticeStateVO 是用户在模拟训练中的动态掌握状态
type PracticeStateVO struct {
	StateID         string `json:"state_id"`
	UserID          string `json:"user_id"`
	Topic           string `json:"topic"`
	Dimension       string `json:"dimension"`
	MasteryScore    int    `json:"mastery_score"`
	AttemptCount    int    `json:"attempt_count"`
	LastScore       int    `json:"last_score"`
	LastFeedback    string `json:"last_feedback"`
	LastPracticedAt int64  `json:"last_practiced_at"`
	SourceType      string `json:"source_type"`
	SourceID        string `json:"source_id"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// SelectedMemoryItemVO is a debug view of one MemorySelector memory decision.
type SelectedMemoryItemVO struct {
	MemoryID        string `json:"memory_id"`
	UserID          string `json:"user_id"`
	MemoryType      string `json:"memory_type"`
	SubjectKey      string `json:"subject_key"`
	Content         string `json:"content"`
	Evidence        string `json:"evidence"`
	Confidence      string `json:"confidence"`
	Score           int    `json:"score"`
	SelectionReason string `json:"selection_reason"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// SelectedPracticeStateVO is a debug view of one MemorySelector practice-state decision.
type SelectedPracticeStateVO struct {
	StateID         string `json:"state_id"`
	UserID          string `json:"user_id"`
	Topic           string `json:"topic"`
	Dimension       string `json:"dimension"`
	MasteryScore    int    `json:"mastery_score"`
	AttemptCount    int    `json:"attempt_count"`
	LastScore       int    `json:"last_score"`
	LastFeedback    string `json:"last_feedback"`
	LastPracticedAt int64  `json:"last_practiced_at"`
	SourceType      string `json:"source_type"`
	SourceID        string `json:"source_id"`
	Score           int    `json:"score"`
	SelectionReason string `json:"selection_reason"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// SelectedContextDebugVO is a dynamic, read-only MemorySelector debug response.
type SelectedContextDebugVO struct {
	InterviewID            string                    `json:"interview_id"`
	UserID                 string                    `json:"user_id"`
	CompanyName            string                    `json:"company_name"`
	JobTitle               string                    `json:"job_title"`
	TargetRound            string                    `json:"target_round"`
	CurrentTask            string                    `json:"current_task"`
	DebugSummary           string                    `json:"debug_summary"`
	SelectedMemoryItems    []SelectedMemoryItemVO    `json:"selected_memory_items"`
	SelectedPracticeStates []SelectedPracticeStateVO `json:"selected_practice_states"`
}

// AgentDecisionTraceVO exposes a read-only Agent execution trace for debugging and evaluation.
type AgentDecisionTraceVO struct {
	TraceID                 string `json:"trace_id"`
	UserID                  string `json:"user_id"`
	InterviewID             string `json:"interview_id"`
	AgentType               string `json:"agent_type"`
	SourceType              string `json:"source_type"`
	SourceID                string `json:"source_id"`
	StepName                string `json:"step_name"`
	SelectedContextSnapshot string `json:"selected_context_snapshot,omitempty"`
	InputSnapshot           string `json:"input_snapshot,omitempty"`
	RawAgentOutput          string `json:"raw_agent_output,omitempty"`
	ParsedDecision          string `json:"parsed_decision,omitempty"`
	ServiceActions          string `json:"service_actions,omitempty"`
	Status                  string `json:"status"`
	ErrorMessage            string `json:"error_message,omitempty"`
	CreatedAt               int64  `json:"created_at"`
}

// AgentEvaluationReportVO is a real-time rule evaluation report over Agent decision traces.
type AgentEvaluationReportVO struct {
	TotalTraces  int                       `json:"total_traces"`
	PassedTraces int                       `json:"passed_traces"`
	FailedTraces int                       `json:"failed_traces"`
	Results      []AgentEvaluationResultVO `json:"results"`
}

type AgentEvaluationResultVO struct {
	TraceID     string                   `json:"trace_id"`
	AgentType   string                   `json:"agent_type"`
	SourceType  string                   `json:"source_type"`
	SourceID    string                   `json:"source_id"`
	StepName    string                   `json:"step_name"`
	TraceStatus string                   `json:"trace_status"`
	Passed      bool                     `json:"passed"`
	Score       int                      `json:"score"`
	Checks      []AgentEvaluationCheckVO `json:"checks"`
}

type AgentEvaluationCheckVO struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

// DashboardSummaryVO is a read-only aggregate for the agent workbench dashboard.
type DashboardSummaryVO struct {
	RecentInterviews            []InterviewSessionVO         `json:"recent_interviews"`
	PendingMemoryCandidateCount int                          `json:"pending_memory_candidate_count"`
	RecentPendingCandidates     []MemoryCandidateVO          `json:"recent_pending_candidates"`
	ActiveCoachingSessions      []CoachingSessionVO          `json:"active_coaching_sessions"`
	ActiveMockInterviews        []MockInterviewVO            `json:"active_mock_interviews"`
	PracticeStateSummary        PracticeStateSummaryVO       `json:"practice_state_summary"`
	RecentFailedTraces          []AgentDecisionTraceVO       `json:"recent_failed_traces"`
	EvaluationSummary           DashboardEvaluationSummaryVO `json:"evaluation_summary"`
}

type PracticeStateSummaryVO struct {
	TotalStates         int `json:"total_states"`
	AverageMasteryScore int `json:"average_mastery_score"`
	WeakStateCount      int `json:"weak_state_count"`
	RecentAttemptCount  int `json:"recent_attempt_count"`
}

type DashboardEvaluationSummaryVO struct {
	TotalTraces  int `json:"total_traces"`
	PassedTraces int `json:"passed_traces"`
	FailedTraces int `json:"failed_traces"`
}
