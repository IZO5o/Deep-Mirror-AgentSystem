package server

import (
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

type Conversation struct {
	ConversationID string `gorm:"primaryKey"`
	UserID         string `gorm:"index"`
	Title          string
	CreatedAt      int64
}

type ChatMessage struct {
	MessageID       string `gorm:"primaryKey"`
	UserID          string `gorm:"index"`
	ConversationID  string `gorm:"index"`
	ParentMessageID string
	AgentType       string `gorm:"index"`

	Query    string // 用户的原始提问
	Response string // 模型的最终输出
	Rounds   string // 用户提问到模型结束 tool loop 之间所有的 llm 请求，以 json 存储

	Model string // 使用的模型
	Usage string

	CreatedAt int64
}

type InterviewSession struct {
	InterviewID    string `gorm:"primaryKey"`
	UserID         string `gorm:"index"`
	CompanyName    string
	JobTitle       string
	InterviewRound string `gorm:"index"`
	InterviewType  string `gorm:"index"`
	Status         string `gorm:"index"`
	OccurredAt     int64
	CreatedAt      int64
	UpdatedAt      int64
}

type InterviewTranscript struct {
	TranscriptID string `gorm:"primaryKey"`
	InterviewID  string `gorm:"uniqueIndex"`
	UserID       string `gorm:"index"`
	SourceType   string `gorm:"index"`
	Content      string
	Language     string
	CreatedAt    int64
	UpdatedAt    int64
}

type TranscriptSegment struct {
	SegmentID          string `gorm:"primaryKey"`
	InterviewID        string `gorm:"index"`
	TranscriptID       string `gorm:"index"`
	UserID             string `gorm:"index"`
	Sequence           int
	StartOffset        int
	EndOffset          int
	Content            string
	CharCount          int
	Summary            string
	SpeakerRoleNotes   string
	QuestionCandidates string
	KeyEvidence        string
	UncertainParts     string
	RawAgentOutput     string
	Status             string `gorm:"index"`
	ErrorMessage       string
	CreatedAt          int64
	UpdatedAt          int64
}

type MediaFile struct {
	MediaID          string `gorm:"primaryKey"`
	InterviewID      string `gorm:"index"`
	UserID           string `gorm:"index"`
	OriginalFilename string
	StoredFilename   string
	StoragePath      string
	ContentType      string
	MediaType        string `gorm:"index"`
	FileExt          string
	SizeBytes        int64
	Status           string `gorm:"index"`
	ErrorMessage     string
	CreatedAt        int64
	UpdatedAt        int64
}

type TranscriptionJob struct {
	JobID              string `gorm:"primaryKey"`
	MediaID            string `gorm:"uniqueIndex"`
	InterviewID        string `gorm:"index"`
	UserID             string `gorm:"index"`
	Status             string `gorm:"index"`
	InputMediaPath     string
	ExtractedAudioPath string
	ASRProvider        string
	ASRModel           string
	Language           string
	TranscriptID       string
	ErrorMessage       string
	StartedAt          int64
	FinishedAt         int64
	CreatedAt          int64
	UpdatedAt          int64
}

type InterviewQuestion struct {
	QuestionID            string `gorm:"primaryKey"`
	InterviewID           string `gorm:"index"`
	UserID                string `gorm:"index"`
	Sequence              int
	Question              string
	Answer                string
	TopicTags             string
	Difficulty            string
	AnswerQuality         string
	WeaknessSummary       string
	ImprovementSuggestion string
	EvidenceText          string
	CreatedAt             int64
	UpdatedAt             int64
}

type InterviewReviewReport struct {
	ReportID             string `gorm:"primaryKey"`
	InterviewID          string `gorm:"uniqueIndex"`
	UserID               string `gorm:"index"`
	OverallSummary       string
	Strengths            string
	Weaknesses           string
	FollowUpRisks        string
	SuggestedPreparation string
	RawAgentOutput       string
	Status               string `gorm:"index"`
	CreatedAt            int64
	UpdatedAt            int64
}

type MemoryCandidate struct {
	CandidateID    string `gorm:"primaryKey"`
	UserID         string `gorm:"index"`
	InterviewID    string `gorm:"index"`
	MemoryType     string `gorm:"index"`
	SubjectKey     string `gorm:"index"`
	Content        string
	Evidence       string
	Confidence     string
	Status         string `gorm:"index"`
	Source         string `gorm:"index"`
	SourceRefType  string `gorm:"index"`
	SourceRefID    string `gorm:"index"`
	RawAgentOutput string
	CreatedAt      int64
	UpdatedAt      int64
}

type MemoryItem struct {
	MemoryID          string `gorm:"primaryKey"`
	UserID            string `gorm:"index"`
	MemoryType        string `gorm:"index"`
	SubjectKey        string `gorm:"index"`
	Content           string
	Evidence          string
	Confidence        string
	SourceCandidateID string `gorm:"uniqueIndex"`
	SourceInterviewID string `gorm:"index"`
	Status            string `gorm:"index"`
	CreatedAt         int64
	UpdatedAt         int64
}

type CoachingPlan struct {
	PlanID          string `gorm:"primaryKey"`
	UserID          string `gorm:"index"`
	InterviewID     string `gorm:"index"`
	PracticeGoalID  string `gorm:"index"`
	SourceType      string `gorm:"index"`
	TargetRound     string `gorm:"index"`
	RemainingDays   int
	CompanyName     string
	JobTitle        string
	OverallStrategy string
	FocusSummary    string
	RawAgentOutput  string
	Status          string `gorm:"index"`
	CreatedAt       int64
	UpdatedAt       int64
}

type PracticeGoal struct {
	GoalID         string `gorm:"primaryKey"`
	UserID         string `gorm:"index"`
	CompanyName    string
	JobTitle       string
	TargetRound    string `gorm:"index"`
	JobDescription string
	FocusTopics    string
	RemainingDays  int
	Status         string `gorm:"index"`
	CreatedAt      int64
	UpdatedAt      int64
}

type CoachingTask struct {
	TaskID           string `gorm:"primaryKey"`
	PlanID           string `gorm:"index"`
	UserID           string `gorm:"index"`
	InterviewID      string `gorm:"index"`
	PracticeGoalID   string `gorm:"index"`
	SourceType       string `gorm:"index"`
	Sequence         int
	DayIndex         int
	TaskType         string `gorm:"index"`
	Title            string
	Description      string
	RelatedMemoryIDs string
	Priority         string `gorm:"index"`
	Status           string `gorm:"index"`
	CreatedAt        int64
	UpdatedAt        int64
}

type CoachingSession struct {
	SessionID            string `gorm:"primaryKey"`
	UserID               string `gorm:"index"`
	InterviewID          string `gorm:"index"`
	PracticeGoalID       string `gorm:"index"`
	CoachingPlanID       string `gorm:"index"`
	CurrentTaskID        string `gorm:"index"`
	Status               string `gorm:"index"`
	ProgressSummary      string
	LastAgentMessage     string
	ErrorMessage         string
	FailedRetryCount     int
	AgentPersistentState *string `gorm:"type:json"`
	StartedAt            int64
	LastActiveAt         int64
	CompletedAt          int64
	CreatedAt            int64
	UpdatedAt            int64
}

type CoachingSessionTurn struct {
	TurnID         string `gorm:"primaryKey"`
	SessionID      string `gorm:"index"`
	CoachingPlanID string `gorm:"index"`
	CoachingTaskID string `gorm:"index"`
	Role           string `gorm:"index"`
	TurnType       string `gorm:"index"`
	Content        string
	AgentAction    string
	Score          int
	Feedback       string
	RawAgentOutput string
	ErrorMessage   string
	CreatedAt      int64
}

type CoachingTaskAttempt struct {
	AttemptID      string `gorm:"primaryKey"`
	SessionID      string `gorm:"index"`
	CoachingTaskID string `gorm:"index"`
	UserAnswer     string
	Score          int
	Feedback       string
	Passed         bool
	AttemptIndex   int
	RawAgentOutput string
	ErrorMessage   string
	CreatedAt      int64
}

type MockInterview struct {
	MockID               string `gorm:"primaryKey"`
	UserID               string `gorm:"index"`
	InterviewID          string `gorm:"index"`
	PracticeGoalID       string `gorm:"index"`
	PlanID               string `gorm:"index"`
	TargetRound          string `gorm:"index"`
	Status               string `gorm:"index"`
	CurrentTurn          int
	CurrentTopic         string
	OverallGoal          string
	FirstQuestion        string
	LastFeedback         string
	ErrorMessage         string
	FailedRetryCount     int
	FinalSummary         string
	AgentPersistentState *string `gorm:"type:json"`
	RawAgentOutput       string
	CreatedAt            int64
	UpdatedAt            int64
}

type MockTurn struct {
	TurnID              string `gorm:"primaryKey"`
	MockID              string `gorm:"index"`
	UserID              string `gorm:"index"`
	InterviewID         string `gorm:"index"`
	PracticeGoalID      string `gorm:"index"`
	TurnIndex           int
	Role                string `gorm:"index"`
	TurnType            string `gorm:"index"`
	Phase               string `gorm:"index"`
	AgentAction         string
	Content             string
	InterviewerQuestion string
	UserAnswer          string
	Feedback            string
	Score               int
	FollowUpReason      string
	TimeLimitSeconds    int
	TimePressureStyle   string
	WarnAtSeconds       int
	TopicTags           string
	NextQuestion        string
	RawAgentOutput      string
	ErrorMessage        string
	CreatedAt           int64
	UpdatedAt           int64
}

type PracticeState struct {
	StateID         string `gorm:"primaryKey"`
	UserID          string `gorm:"index;uniqueIndex:idx_practice_user_topic"`
	Topic           string `gorm:"index;uniqueIndex:idx_practice_user_topic"`
	Dimension       string `gorm:"index"`
	MasteryScore    int
	AttemptCount    int
	LastScore       int
	LastFeedback    string
	LastPracticedAt int64
	SourceType      string `gorm:"index"`
	SourceID        string `gorm:"index"`
	CreatedAt       int64
	UpdatedAt       int64
}

type AgentDecisionTrace struct {
	TraceID                 string `gorm:"primaryKey"`
	UserID                  string `gorm:"index"`
	InterviewID             string `gorm:"index"`
	AgentType               string `gorm:"index"`
	SourceType              string `gorm:"index"`
	SourceID                string `gorm:"index"`
	StepName                string `gorm:"index"`
	SelectedContextSnapshot string
	InputSnapshot           string
	RawAgentOutput          string
	ParsedDecision          string
	ServiceActions          string
	Status                  string `gorm:"index"`
	ErrorMessage            string
	CreatedAt               int64
}

// MemoryEvent records a factual practice/interview observation for timeline/audit use.
type MemoryEvent struct {
	EventID     string `gorm:"primaryKey"`
	UserID      string `gorm:"index"`
	SourceType  string `gorm:"index"`
	SourceID    string `gorm:"index"`
	Topic       string
	Observation string
	ScoreTrend  string
	CreatedAt   int64
}

func InitDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	err = db.AutoMigrate(
		&Conversation{},
		&ChatMessage{},
		&InterviewSession{},
		&InterviewTranscript{},
		&TranscriptSegment{},
		&MediaFile{},
		&TranscriptionJob{},
		&InterviewQuestion{},
		&InterviewReviewReport{},
		&MemoryCandidate{},
		&MemoryItem{},
		&CoachingPlan{},
		&PracticeGoal{},
		&CoachingTask{},
		&CoachingSession{},
		&CoachingSessionTurn{},
		&CoachingTaskAttempt{},
		&MockInterview{},
		&MockTurn{},
		&PracticeState{},
		&AgentDecisionTrace{},
		&MemoryEvent{},
	)
	if err != nil {
		return nil, err
	}
	return db, nil
}
