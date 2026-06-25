package server

import (
	"strings"
	"testing"
)

func TestSelectMemoriesForCoaching_CompanyAndJobMatchRankHigher(t *testing.T) {
	s := newTestServer(t)
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "company-acme",
		UserID:     "user_001",
		MemoryType: MemoryTypeCompanyProfile,
		SubjectKey: "company:Acme",
		Content:    "Acme values distributed systems tradeoff discussion.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
		CreatedAt:  100,
		UpdatedAt:  100,
	})
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "job-acme-backend",
		UserID:     "user_001",
		MemoryType: MemoryTypeJobProfile,
		SubjectKey: "job:Acme:Backend Engineer",
		Content:    "Backend Engineer role cares about Redis and MySQL.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
		CreatedAt:  90,
		UpdatedAt:  90,
	})
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "company-other",
		UserID:     "user_001",
		MemoryType: MemoryTypeCompanyProfile,
		SubjectKey: "company:Other",
		Content:    "Other company frontend focus.",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
		CreatedAt:  200,
		UpdatedAt:  200,
	})

	result, err := s.SelectMemoriesForCoaching(MemorySelectionRequest{
		UserID:      "user_001",
		CompanyName: "Acme",
		JobTitle:    "Backend Engineer",
	})
	if err != nil {
		t.Fatalf("SelectMemoriesForCoaching() error = %v", err)
	}
	if len(result.MemoryItems) < 2 {
		t.Fatalf("selected memories length = %d, want >= 2", len(result.MemoryItems))
	}
	firstIDs := []string{result.MemoryItems[0].MemoryItem.MemoryID, result.MemoryItems[1].MemoryItem.MemoryID}
	if !containsString(firstIDs, "company-acme") || !containsString(firstIDs, "job-acme-backend") {
		t.Fatalf("top memories = %#v, want company and job matches first", firstIDs)
	}
	if result.MemoryItems[0].SelectionReason == "" {
		t.Fatalf("selection reason is empty")
	}
}

func TestSelectMemoriesForCoaching_UserWeaknessAndPreparationTipIncluded(t *testing.T) {
	s := newTestServer(t)
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "weakness",
		UserID:     "user_001",
		MemoryType: MemoryTypeUserWeakness,
		SubjectKey: "user:user_001",
		Content:    "Redis consistency weakness",
		Confidence: MemoryConfidenceHigh,
		Status:     MemoryItemStatusActive,
	})
	seedMemoryItem(t, s, MemoryItem{
		MemoryID:   "tip",
		UserID:     "user_001",
		MemoryType: MemoryTypePreparationTip,
		SubjectKey: "user:user_001",
		Content:    "Prepare delay double delete examples",
		Confidence: MemoryConfidenceMedium,
		Status:     MemoryItemStatusActive,
	})

	result, err := s.SelectMemoriesForCoaching(MemorySelectionRequest{UserID: "user_001"})
	if err != nil {
		t.Fatalf("SelectMemoriesForCoaching() error = %v", err)
	}
	ids := selectedMemoryIDs(result.MemoryItems)
	if !containsString(ids, "weakness") || !containsString(ids, "tip") {
		t.Fatalf("selected memory ids = %#v, want weakness and tip", ids)
	}
	for _, selected := range result.MemoryItems {
		if selected.Score <= 0 || selected.SelectionReason == "" {
			t.Fatalf("selected memory has invalid score/reason: %#v", selected)
		}
	}
}

func TestSelectMemoriesForMock_PrioritizesWeaknessQuestionPatternAndInterviewerFocus(t *testing.T) {
	s := newTestServer(t)
	for _, item := range []MemoryItem{
		{MemoryID: "weakness", UserID: "user_001", MemoryType: MemoryTypeUserWeakness, SubjectKey: "user:user_001", Content: "Concurrency weakness", Confidence: MemoryConfidenceHigh, Status: MemoryItemStatusActive},
		{MemoryID: "pattern", UserID: "user_001", MemoryType: MemoryTypeQuestionPattern, SubjectKey: "user:user_001", Content: "Interviewer often asks Redis follow-ups", Confidence: MemoryConfidenceHigh, Status: MemoryItemStatusActive},
		{MemoryID: "focus", UserID: "user_001", MemoryType: MemoryTypeInterviewerFocus, SubjectKey: "interviewer:1:professional_focus", Content: "Focuses on system design tradeoffs", Confidence: MemoryConfidenceHigh, Status: MemoryItemStatusActive},
		{MemoryID: "tip", UserID: "user_001", MemoryType: MemoryTypePreparationTip, SubjectKey: "user:user_001", Content: "Review notes before mock", Confidence: MemoryConfidenceHigh, Status: MemoryItemStatusActive},
	} {
		seedMemoryItem(t, s, item)
	}

	result, err := s.SelectMemoriesForMock(MemorySelectionRequest{UserID: "user_001", CurrentTask: MemorySelectorTaskMockStart})
	if err != nil {
		t.Fatalf("SelectMemoriesForMock() error = %v", err)
	}
	if len(result.MemoryItems) < 4 {
		t.Fatalf("selected memories length = %d, want 4", len(result.MemoryItems))
	}
	topThree := selectedMemoryIDs(result.MemoryItems[:3])
	for _, want := range []string{"weakness", "pattern", "focus"} {
		if !containsString(topThree, want) {
			t.Fatalf("top three = %#v, want %q", topThree, want)
		}
	}
	if result.MemoryItems[3].MemoryItem.MemoryID != "tip" {
		t.Fatalf("fourth memory = %q, want tip", result.MemoryItems[3].MemoryItem.MemoryID)
	}
}

func TestSelectPracticeStates_PrioritizesLowMastery(t *testing.T) {
	s := newTestServer(t)
	seedSelectorPracticeState(t, s, PracticeState{
		StateID:      "low",
		UserID:       "user_001",
		Topic:        "Redis consistency",
		Dimension:    PracticeDimensionBackendKnowledge,
		MasteryScore: 35,
		LastScore:    50,
		LastFeedback: "Need clearer Redis tradeoffs",
		AttemptCount: 2,
	})
	seedSelectorPracticeState(t, s, PracticeState{
		StateID:      "high",
		UserID:       "user_001",
		Topic:        "Project intro",
		Dimension:    PracticeDimensionCommunication,
		MasteryScore: 90,
		LastScore:    90,
		AttemptCount: 1,
	})

	result, err := s.SelectMemoriesForMock(MemorySelectionRequest{UserID: "user_001", JobTitle: "Backend Engineer"})
	if err != nil {
		t.Fatalf("SelectMemoriesForMock() error = %v", err)
	}
	if len(result.PracticeStates) < 2 {
		t.Fatalf("selected states length = %d, want 2", len(result.PracticeStates))
	}
	if result.PracticeStates[0].PracticeState.StateID != "low" {
		t.Fatalf("top practice state = %q, want low", result.PracticeStates[0].PracticeState.StateID)
	}
	if !strings.Contains(result.PracticeStates[0].SelectionReason, "low mastery") {
		t.Fatalf("selection_reason = %q, want low mastery", result.PracticeStates[0].SelectionReason)
	}
}

func TestSelectorFiltersByUserAndActiveStatus(t *testing.T) {
	s := newTestServer(t)
	seedMemoryItem(t, s, MemoryItem{MemoryID: "active", UserID: "user_001", MemoryType: MemoryTypeUserWeakness, SubjectKey: "user:user_001", Content: "active weakness", Status: MemoryItemStatusActive})
	seedMemoryItem(t, s, MemoryItem{MemoryID: "archived", UserID: "user_001", MemoryType: MemoryTypeUserWeakness, SubjectKey: "user:user_001", Content: "archived weakness", Status: MemoryItemStatusArchived})
	seedMemoryItem(t, s, MemoryItem{MemoryID: "other-user", UserID: "user_002", MemoryType: MemoryTypeUserWeakness, SubjectKey: "user:user_002", Content: "other weakness", Status: MemoryItemStatusActive})

	result, err := s.SelectMemoriesForCoaching(MemorySelectionRequest{UserID: "user_001"})
	if err != nil {
		t.Fatalf("SelectMemoriesForCoaching() error = %v", err)
	}
	ids := selectedMemoryIDs(result.MemoryItems)
	if len(ids) != 1 || ids[0] != "active" {
		t.Fatalf("selected memory ids = %#v, want only active", ids)
	}
}

func seedMemoryItem(t *testing.T, s *Server, item MemoryItem) {
	t.Helper()
	if item.SourceCandidateID == "" {
		item.SourceCandidateID = item.MemoryID + "-candidate"
	}
	if item.Confidence == "" {
		item.Confidence = MemoryConfidenceMedium
	}
	if item.Status == "" {
		item.Status = MemoryItemStatusActive
	}
	if err := s.db.Create(&item).Error; err != nil {
		t.Fatalf("seed memory item: %v", err)
	}
}

func seedSelectorPracticeState(t *testing.T, s *Server, state PracticeState) {
	t.Helper()
	if err := s.db.Create(&state).Error; err != nil {
		t.Fatalf("seed practice state: %v", err)
	}
}

func selectedMemoryIDs(items []SelectedMemoryItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.MemoryItem.MemoryID)
	}
	return ids
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
