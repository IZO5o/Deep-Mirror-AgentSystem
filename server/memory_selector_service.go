package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"agent-web-base/vo"
)

const (
	MemorySelectorTaskCoachingPlan = "coaching_plan"
	MemorySelectorTaskMockStart    = "mock_start"
	MemorySelectorTaskMockTurn     = "mock_turn"

	defaultMemorySelectionLimit        = 8
	defaultPracticeStateSelectionLimit = 8
)

type MemorySelectionRequest struct {
	UserID              string
	CompanyName         string
	JobTitle            string
	TargetRound         string
	CurrentTask         string
	LimitMemoryItems    int
	LimitPracticeStates int
}

type SelectedMemoryItem struct {
	MemoryItem      MemoryItem
	Score           int
	SelectionReason string
}

type SelectedPracticeState struct {
	PracticeState   PracticeState
	Score           int
	SelectionReason string
}

type MemorySelectionResult struct {
	MemoryItems    []SelectedMemoryItem
	PracticeStates []SelectedPracticeState
	DebugSummary   string
}

func (s *Server) SelectMemoriesForCoaching(req MemorySelectionRequest) (MemorySelectionResult, error) {
	req.CurrentTask = normalizeDefault(req.CurrentTask, MemorySelectorTaskCoachingPlan)
	req.LimitMemoryItems = normalizedSelectionLimit(req.LimitMemoryItems)
	req.LimitPracticeStates = normalizedPracticeStateLimit(req.LimitPracticeStates)

	memories, err := s.loadCandidateMemoryItems(req.UserID, coachingMemoryTypes())
	if err != nil {
		return MemorySelectionResult{}, err
	}
	states, err := s.loadCandidatePracticeStates(req.UserID)
	if err != nil {
		return MemorySelectionResult{}, err
	}

	result := MemorySelectionResult{
		MemoryItems:    rankMemoryItems(memories, req, scoreMemoryForCoaching, req.LimitMemoryItems),
		PracticeStates: rankPracticeStates(states, req, scorePracticeStateForCoaching, req.LimitPracticeStates),
	}
	result.DebugSummary = buildSelectionDebugSummary(result)
	return result, nil
}

func (s *Server) SelectMemoriesForMock(req MemorySelectionRequest) (MemorySelectionResult, error) {
	if strings.TrimSpace(req.CurrentTask) == "" {
		req.CurrentTask = MemorySelectorTaskMockStart
	}
	req.LimitMemoryItems = normalizedSelectionLimit(req.LimitMemoryItems)
	req.LimitPracticeStates = normalizedPracticeStateLimit(req.LimitPracticeStates)

	memories, err := s.loadCandidateMemoryItems(req.UserID, mockMemoryTypes())
	if err != nil {
		return MemorySelectionResult{}, err
	}
	states, err := s.loadCandidatePracticeStates(req.UserID)
	if err != nil {
		return MemorySelectionResult{}, err
	}

	result := MemorySelectionResult{
		MemoryItems:    rankMemoryItems(memories, req, scoreMemoryForMock, req.LimitMemoryItems),
		PracticeStates: rankPracticeStates(states, req, scorePracticeStateForMock, req.LimitPracticeStates),
	}
	result.DebugSummary = buildSelectionDebugSummary(result)
	return result, nil
}

func (s *Server) GetSelectedContextDebug(interviewID string, userID string, targetRound string, currentTask string) (vo.SelectedContextDebugVO, error) {
	var session InterviewSession
	if err := s.db.First(&session, "interview_id = ?", interviewID).Error; err != nil {
		return vo.SelectedContextDebugVO{}, err
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = session.UserID
	}
	targetRound = normalizeDefault(strings.TrimSpace(targetRound), session.InterviewRound)
	currentTask = normalizeDefault(strings.TrimSpace(currentTask), MemorySelectorTaskCoachingPlan)

	req := MemorySelectionRequest{
		UserID:      userID,
		CompanyName: session.CompanyName,
		JobTitle:    session.JobTitle,
		TargetRound: targetRound,
		CurrentTask: currentTask,
	}

	var selected MemorySelectionResult
	var err error
	switch currentTask {
	case MemorySelectorTaskCoachingPlan:
		selected, err = s.SelectMemoriesForCoaching(req)
	case MemorySelectorTaskMockStart, MemorySelectorTaskMockTurn:
		selected, err = s.SelectMemoriesForMock(req)
	default:
		return vo.SelectedContextDebugVO{}, fmt.Errorf("unsupported current_task %q", currentTask)
	}
	if err != nil {
		return vo.SelectedContextDebugVO{}, err
	}

	return vo.SelectedContextDebugVO{
		InterviewID:            interviewID,
		UserID:                 userID,
		CompanyName:            session.CompanyName,
		JobTitle:               session.JobTitle,
		TargetRound:            targetRound,
		CurrentTask:            currentTask,
		DebugSummary:           selected.DebugSummary,
		SelectedMemoryItems:    toSelectedMemoryItemVOs(selected.MemoryItems),
		SelectedPracticeStates: toSelectedPracticeStateVOs(selected.PracticeStates),
	}, nil
}

func (s *Server) loadCandidateMemoryItems(userID string, allowedTypes []string) ([]MemoryItem, error) {
	var memories []MemoryItem
	if err := s.db.Where("user_id = ? AND status = ? AND memory_type IN ?", userID, MemoryItemStatusActive, allowedTypes).
		Order("updated_at desc, created_at desc").
		Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

func (s *Server) loadCandidatePracticeStates(userID string) ([]PracticeState, error) {
	var states []PracticeState
	if err := s.db.Where("user_id = ?", userID).
		Order("updated_at desc, created_at desc").
		Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}

func mockMemoryTypes() []string {
	return []string{
		MemoryTypeUserWeakness,
		MemoryTypeCompanyProfile,
		MemoryTypeJobProfile,
		MemoryTypeInterviewerFocus,
		MemoryTypeQuestionPattern,
		MemoryTypePreparationTip,
		MemoryTypeUserStrength,
	}
}

func normalizedSelectionLimit(limit int) int {
	if limit <= 0 {
		return defaultMemorySelectionLimit
	}
	return limit
}

func normalizedPracticeStateLimit(limit int) int {
	if limit <= 0 {
		return defaultPracticeStateSelectionLimit
	}
	return limit
}

type memoryScoreFunc func(MemoryItem, MemorySelectionRequest) (int, []string)

func rankMemoryItems(memories []MemoryItem, req MemorySelectionRequest, scoreFn memoryScoreFunc, limit int) []SelectedMemoryItem {
	selected := make([]SelectedMemoryItem, 0, len(memories))
	for _, memory := range memories {
		score, reasons := scoreFn(memory, req)
		if score <= 0 {
			continue
		}
		selected = append(selected, SelectedMemoryItem{
			MemoryItem:      memory,
			Score:           score,
			SelectionReason: strings.Join(uniqueNonEmptyStrings(reasons), "; "),
		})
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Score != selected[j].Score {
			return selected[i].Score > selected[j].Score
		}
		return latestTimestamp(selected[i].MemoryItem.UpdatedAt, selected[i].MemoryItem.CreatedAt) > latestTimestamp(selected[j].MemoryItem.UpdatedAt, selected[j].MemoryItem.CreatedAt)
	})
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

type practiceScoreFunc func(PracticeState, MemorySelectionRequest) (int, []string)

func rankPracticeStates(states []PracticeState, req MemorySelectionRequest, scoreFn practiceScoreFunc, limit int) []SelectedPracticeState {
	selected := make([]SelectedPracticeState, 0, len(states))
	for _, state := range states {
		score, reasons := scoreFn(state, req)
		if score <= 0 {
			continue
		}
		selected = append(selected, SelectedPracticeState{
			PracticeState:   state,
			Score:           score,
			SelectionReason: strings.Join(uniqueNonEmptyStrings(reasons), "; "),
		})
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Score != selected[j].Score {
			return selected[i].Score > selected[j].Score
		}
		return latestTimestamp(selected[i].PracticeState.UpdatedAt, selected[i].PracticeState.CreatedAt) > latestTimestamp(selected[j].PracticeState.UpdatedAt, selected[j].PracticeState.CreatedAt)
	})
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func scoreMemoryForCoaching(memory MemoryItem, req MemorySelectionRequest) (int, []string) {
	score := 1
	reasons := []string{"active memory_item"}
	text := memorySearchText(memory)
	subject := normalizeSearchText(memory.SubjectKey)
	matchScore, matchReasons := scoreCompanyJobMatches(text, subject, req)
	if isContextScopedMemory(memory.MemoryType) && hasCompanyOrJobContext(req) && matchScore == 0 {
		return 0, nil
	}

	switch memory.MemoryType {
	case MemoryTypeUserWeakness:
		score += 25
		reasons = append(reasons, "user weakness is important for preparation planning")
	case MemoryTypePreparationTip:
		score += 20
		reasons = append(reasons, "preparation tip supports coaching tasks")
	case MemoryTypeCompanyProfile:
		score += 12
		reasons = append(reasons, "company profile can shape next-round strategy")
	case MemoryTypeJobProfile:
		score += 12
		reasons = append(reasons, "job profile can shape role-specific preparation")
	case MemoryTypeQuestionPattern:
		score += 14
		reasons = append(reasons, "question pattern indicates likely follow-up risk")
	case MemoryTypeInterviewerFocus:
		score += 12
		reasons = append(reasons, "interviewer focus informs preparation priorities")
	case MemoryTypeUserStrength:
		score += 8
		reasons = append(reasons, "user strength helps balance the plan")
	}

	score += matchScore
	reasons = append(reasons, matchReasons...)
	score += scoreConfidence(memory.Confidence, &reasons)
	score += scoreTargetRoundMatch(text, req.TargetRound, &reasons)
	score += scoreTaskMatch(text, req.CurrentTask, &reasons)
	score += recencyBonus(memory.UpdatedAt, memory.CreatedAt, &reasons)
	return score, reasons
}

func scoreMemoryForMock(memory MemoryItem, req MemorySelectionRequest) (int, []string) {
	score := 1
	reasons := []string{"active memory_item"}
	text := memorySearchText(memory)
	subject := normalizeSearchText(memory.SubjectKey)
	matchScore, matchReasons := scoreCompanyJobMatches(text, subject, req)
	if isContextScopedMemory(memory.MemoryType) && hasCompanyOrJobContext(req) && matchScore == 0 {
		return 0, nil
	}

	switch memory.MemoryType {
	case MemoryTypeUserWeakness:
		score += 35
		reasons = append(reasons, "user weakness should drive mock interview pressure")
	case MemoryTypeQuestionPattern:
		score += 30
		reasons = append(reasons, "question pattern informs follow-up questions")
	case MemoryTypeInterviewerFocus:
		score += 28
		reasons = append(reasons, "interviewer focus informs mock interviewer style")
	case MemoryTypeCompanyProfile:
		score += 12
		reasons = append(reasons, "company profile keeps mock context relevant")
	case MemoryTypeJobProfile:
		score += 12
		reasons = append(reasons, "job profile keeps mock context relevant")
	case MemoryTypePreparationTip:
		score += 8
		reasons = append(reasons, "preparation tip can guide feedback")
	case MemoryTypeUserStrength:
		score += 5
		reasons = append(reasons, "user strength helps balance feedback")
	}

	score += matchScore
	reasons = append(reasons, matchReasons...)
	score += scoreConfidence(memory.Confidence, &reasons)
	score += scoreTargetRoundMatch(text, req.TargetRound, &reasons)
	score += scoreTaskMatch(text, req.CurrentTask, &reasons)
	score += recencyBonus(memory.UpdatedAt, memory.CreatedAt, &reasons)
	return score, reasons
}

func scorePracticeStateForCoaching(state PracticeState, req MemorySelectionRequest) (int, []string) {
	score := 1
	reasons := []string{"practice state for current user"}
	score += lowMasteryScore(state.MasteryScore, &reasons)
	if state.AttemptCount > 0 {
		score += 8
		reasons = append(reasons, "has practice attempts")
	}
	if strings.TrimSpace(state.LastFeedback) != "" {
		score += 10
		reasons = append(reasons, "has recent feedback")
	}
	score += practiceKeywordScore(state, req, &reasons)
	score += practiceRecencyBonus(state, &reasons)
	return score, reasons
}

func scorePracticeStateForMock(state PracticeState, req MemorySelectionRequest) (int, []string) {
	score := 1
	reasons := []string{"practice state for current user"}
	score += lowMasteryScore(state.MasteryScore, &reasons) * 2
	if state.LastScore > 0 && state.LastScore < 70 {
		score += 18
		reasons = append(reasons, "last score is low")
	}
	if state.AttemptCount >= 2 && state.MasteryScore < 70 {
		score += 15
		reasons = append(reasons, "repeated attempts still have low mastery")
	} else if state.AttemptCount > 0 {
		score += 6
		reasons = append(reasons, "has practice attempts")
	}
	if strings.TrimSpace(state.LastFeedback) != "" {
		score += 12
		reasons = append(reasons, "has recent feedback")
	}
	score += practiceKeywordScore(state, req, &reasons)
	score += practiceRecencyBonus(state, &reasons)
	return score, reasons
}

func memorySearchText(memory MemoryItem) string {
	return normalizeSearchText(strings.Join([]string{
		memory.MemoryType,
		memory.SubjectKey,
		memory.Content,
		memory.Evidence,
		memory.Confidence,
	}, " "))
}

func scoreCompanyJobMatches(text string, subject string, req MemorySelectionRequest) (int, []string) {
	score := 0
	reasons := []string{}
	company := normalizeSearchText(req.CompanyName)
	job := normalizeSearchText(req.JobTitle)
	if company != "" {
		if subject == "company:"+company {
			score += 35
			reasons = append(reasons, "subject_key exactly matches current company")
		}
		if strings.Contains(text, company) {
			score += 12
			reasons = append(reasons, "matches current company")
		}
	}
	if company != "" && job != "" && subject == "job:"+company+":"+job {
		score += 40
		reasons = append(reasons, "subject_key exactly matches current job")
	}
	if job != "" && strings.Contains(text, job) {
		score += 12
		reasons = append(reasons, "matches current job title")
	}
	return score, reasons
}

func isContextScopedMemory(memoryType string) bool {
	switch memoryType {
	case MemoryTypeCompanyProfile, MemoryTypeJobProfile:
		return true
	default:
		return false
	}
}

func hasCompanyOrJobContext(req MemorySelectionRequest) bool {
	return strings.TrimSpace(req.CompanyName) != "" || strings.TrimSpace(req.JobTitle) != ""
}

func scoreConfidence(confidence string, reasons *[]string) int {
	switch confidence {
	case MemoryConfidenceHigh:
		*reasons = append(*reasons, "high confidence")
		return 10
	case MemoryConfidenceMedium:
		*reasons = append(*reasons, "medium confidence")
		return 5
	default:
		return 0
	}
}

func scoreTargetRoundMatch(text string, targetRound string, reasons *[]string) int {
	target := normalizeSearchText(targetRound)
	if target == "" || !strings.Contains(text, target) {
		return 0
	}
	*reasons = append(*reasons, "matches target round")
	return 8
}

func scoreTaskMatch(text string, currentTask string, reasons *[]string) int {
	task := normalizeSearchText(strings.ReplaceAll(currentTask, "_", " "))
	if task == "" || !containsAnyToken(text, strings.Fields(task)) {
		return 0
	}
	*reasons = append(*reasons, "matches current task")
	return 4
}

func recencyBonus(updatedAt int64, createdAt int64, reasons *[]string) int {
	if latestTimestamp(updatedAt, createdAt) <= 0 {
		return 0
	}
	*reasons = append(*reasons, "recent memory")
	return 2
}

func lowMasteryScore(score int, reasons *[]string) int {
	switch {
	case score <= 0:
		return 0
	case score < 40:
		*reasons = append(*reasons, "very low mastery")
		return 30
	case score < 60:
		*reasons = append(*reasons, "low mastery")
		return 22
	case score < 75:
		*reasons = append(*reasons, "medium mastery needs reinforcement")
		return 12
	default:
		return 3
	}
}

func practiceKeywordScore(state PracticeState, req MemorySelectionRequest, reasons *[]string) int {
	text := normalizeSearchText(strings.Join([]string{
		state.Topic,
		state.Dimension,
		state.LastFeedback,
	}, " "))
	keywords := selectionKeywords(req)
	if !containsAnyToken(text, keywords) {
		return 0
	}
	*reasons = append(*reasons, "topic or feedback matches current context")
	return 12
}

func practiceRecencyBonus(state PracticeState, reasons *[]string) int {
	if latestTimestamp(state.UpdatedAt, state.LastPracticedAt) <= 0 {
		return 0
	}
	*reasons = append(*reasons, "recent practice")
	return 3
}

func selectionKeywords(req MemorySelectionRequest) []string {
	raw := []string{req.CompanyName, req.JobTitle, req.TargetRound, strings.ReplaceAll(req.CurrentTask, "_", " ")}
	tokens := make([]string, 0, len(raw)*2)
	for _, value := range raw {
		cleaned := normalizeSearchText(value)
		if cleaned == "" {
			continue
		}
		tokens = append(tokens, cleaned)
		tokens = append(tokens, strings.Fields(cleaned)...)
	}
	return uniqueNonEmptyStrings(tokens)
}

func containsAnyToken(text string, tokens []string) bool {
	for _, token := range tokens {
		token = normalizeSearchText(token)
		if token == "" {
			continue
		}
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func normalizeSearchText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func latestTimestamp(values ...int64) int64 {
	var latest int64
	for _, value := range values {
		if value > latest {
			latest = value
		}
	}
	return latest
}

func buildSelectionDebugSummary(result MemorySelectionResult) string {
	return fmt.Sprintf("selected %d memory_items and %d practice_states", len(result.MemoryItems), len(result.PracticeStates))
}

func selectedMemoriesJSON(memories []SelectedMemoryItem) string {
	payload := make([]map[string]any, 0, len(memories))
	for _, selected := range memories {
		m := selected.MemoryItem
		payload = append(payload, map[string]any{
			"memory_id":        m.MemoryID,
			"memory_type":      m.MemoryType,
			"subject_key":      m.SubjectKey,
			"content":          m.Content,
			"evidence":         m.Evidence,
			"confidence":       m.Confidence,
			"score":            selected.Score,
			"selection_reason": selected.SelectionReason,
		})
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func selectedPracticeStatesJSON(states []SelectedPracticeState) string {
	payload := make([]map[string]any, 0, len(states))
	for _, selected := range states {
		state := selected.PracticeState
		payload = append(payload, map[string]any{
			"state_id":          state.StateID,
			"topic":             state.Topic,
			"dimension":         state.Dimension,
			"mastery_score":     state.MasteryScore,
			"attempt_count":     state.AttemptCount,
			"last_score":        state.LastScore,
			"last_feedback":     state.LastFeedback,
			"last_practiced_at": state.LastPracticedAt,
			"source_type":       state.SourceType,
			"source_id":         state.SourceID,
			"score":             selected.Score,
			"selection_reason":  selected.SelectionReason,
		})
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func toSelectedMemoryItemVOs(items []SelectedMemoryItem) []vo.SelectedMemoryItemVO {
	result := make([]vo.SelectedMemoryItemVO, 0, len(items))
	for _, selected := range items {
		item := selected.MemoryItem
		result = append(result, vo.SelectedMemoryItemVO{
			MemoryID:        item.MemoryID,
			UserID:          item.UserID,
			MemoryType:      item.MemoryType,
			SubjectKey:      item.SubjectKey,
			Content:         item.Content,
			Evidence:        item.Evidence,
			Confidence:      item.Confidence,
			Score:           selected.Score,
			SelectionReason: selected.SelectionReason,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}
	return result
}

func toSelectedPracticeStateVOs(states []SelectedPracticeState) []vo.SelectedPracticeStateVO {
	result := make([]vo.SelectedPracticeStateVO, 0, len(states))
	for _, selected := range states {
		state := selected.PracticeState
		result = append(result, vo.SelectedPracticeStateVO{
			StateID:         state.StateID,
			UserID:          state.UserID,
			Topic:           state.Topic,
			Dimension:       state.Dimension,
			MasteryScore:    state.MasteryScore,
			AttemptCount:    state.AttemptCount,
			LastScore:       state.LastScore,
			LastFeedback:    state.LastFeedback,
			LastPracticedAt: state.LastPracticedAt,
			SourceType:      state.SourceType,
			SourceID:        state.SourceID,
			Score:           selected.Score,
			SelectionReason: selected.SelectionReason,
			CreatedAt:       state.CreatedAt,
			UpdatedAt:       state.UpdatedAt,
		})
	}
	return result
}
