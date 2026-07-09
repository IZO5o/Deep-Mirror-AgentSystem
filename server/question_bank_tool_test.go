package server

import (
	"strings"
	"testing"
)

const testQuestionBankPath = "../testdata/question_bank.json"

func TestQuestionBankHasFiftyQuestions(t *testing.T) {
	bank, err := loadQuestionBank(testQuestionBankPath)
	if err != nil {
		t.Fatalf("loadQuestionBank() error = %v", err)
	}
	if len(bank.Questions) != 50 {
		t.Fatalf("question count = %d, want 50", len(bank.Questions))
	}
	seen := map[string]bool{}
	for _, q := range bank.Questions {
		if q.ID == "" || q.Topic == "" || q.Question == "" || q.Difficulty == "" {
			t.Fatalf("incomplete question: %+v", q)
		}
		if seen[q.ID] {
			t.Fatalf("duplicate question id: %s", q.ID)
		}
		seen[q.ID] = true
		if len(q.ExpectedPoints) == 0 || len(q.FollowUpHints) == 0 {
			t.Fatalf("question %s missing expected points or follow-up hints", q.ID)
		}
	}
}

func TestSearchQuestionsByTopicDifficultyAndCompany(t *testing.T) {
	got, err := SearchQuestionsFromFile(testQuestionBankPath, "Redis 分布式锁", "medium", "字节跳动", 2)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("SearchQuestionsFromFile() returned empty result")
	}
	if got[0].ID != "q_redis_001" {
		t.Fatalf("first question id = %s, want q_redis_001", got[0].ID)
	}
}

func TestFormatQuestionBankPromptSection(t *testing.T) {
	questions, err := SearchQuestionsFromFile(testQuestionBankPath, "秒杀系统", "hard", "阿里巴巴", 1)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	text := formatQuestionBankPromptSection(questions)
	for _, want := range []string{"=== 可选练习题 ===", "q_system_002", "expected_points", "follow_up_hints"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt section missing %q in %s", want, text)
		}
	}
}

func TestQuestionBankSearchEmptyResult(t *testing.T) {
	got, err := SearchQuestionsFromFile(testQuestionBankPath, "量子计算", "", "", 5)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result for non-matching topic, got %d questions", len(got))
	}
}

func TestQuestionBankDefaultLimit(t *testing.T) {
	got, err := SearchQuestionsFromFile(testQuestionBankPath, "", "", "", 0)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	if len(got) == 0 || len(got) > 3 {
		t.Fatalf("expected default limit 3, got %d questions", len(got))
	}
}

func TestQuestionBankFormatEmpty(t *testing.T) {
	text := formatQuestionBankPromptSection(nil)
	if !strings.Contains(text, "暂无匹配题库题") {
		t.Fatalf("empty format missing placeholder text: %s", text)
	}
}

func TestQuestionBankAliBabaCompanyTag(t *testing.T) {
	got, err := SearchQuestionsFromFile(testQuestionBankPath, "秒杀系统", "hard", "阿里巴巴", 5)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	found := false
	for _, q := range got {
		if q.ID == "q_system_002" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected q_system_002 in search results for 秒杀系统/阿里巴巴, got: %+v", got)
	}
}

func TestQuestionBankFileNotFound(t *testing.T) {
	_, err := SearchQuestionsFromFile("nonexistent.json", "", "", "", 3)
	if err == nil {
		t.Fatalf("expected error for nonexistent file, got nil")
	}
}

// TestQuestionBankExactDifficultyMatch verifies difficulty filtering.
func TestQuestionBankExactDifficultyMatch(t *testing.T) {
	// Difficulty matching adds bonus score, making hard questions rank higher.
	// A hard question should score strictly higher than a medium one
	// when topic also matches.
	got, err := SearchQuestionsFromFile(testQuestionBankPath, "微服务", "hard", "", 2)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	// With limit 2, hard questions should be ranked first
	if len(got) < 1 {
		t.Fatalf("expected at least 1 result, got 0")
	}
	if got[0].Difficulty != "hard" {
		t.Fatalf("expected first result to be hard difficulty, got %s for %s", got[0].Difficulty, got[0].ID)
	}
}

func TestQuestionBankScoreByCompanyMatchOnly(t *testing.T) {
	got, err := SearchQuestionsFromFile(testQuestionBankPath, "", "", "京东", 5)
	if err != nil {
		t.Fatalf("SearchQuestionsFromFile() error = %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected results for company 京东, got empty")
	}
	for _, q := range got {
		found := false
		for _, tag := range q.CompanyTags {
			if strings.Contains(tag, "京东") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("question %s has no matching company tag 京东, tags: %v", q.ID, q.CompanyTags)
		}
	}
}

func TestQuestionBankQuestionIDsConsistent(t *testing.T) {
	bank, err := loadQuestionBank(testQuestionBankPath)
	if err != nil {
		t.Fatalf("loadQuestionBank() error = %v", err)
	}
	for _, q := range bank.Questions {
		if q.Topic == "Redis" && !strings.HasPrefix(q.ID, "q_redis_") {
			t.Fatalf("Redis question %s has wrong id prefix", q.ID)
		}
		if q.Topic == "MySQL" && !strings.HasPrefix(q.ID, "q_mysql_") {
			t.Fatalf("MySQL question %s has wrong id prefix", q.ID)
		}
	}
}
