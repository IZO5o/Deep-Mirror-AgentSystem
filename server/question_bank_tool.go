package server

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
)

const defaultQuestionBankPath = "testdata/question_bank.json"

type QuestionBank struct {
	Questions []QuestionBankQuestion `json:"questions"`
}

type QuestionBankQuestion struct {
	ID             string   `json:"id"`
	Topic          string   `json:"topic"`
	SubTopic       string   `json:"sub_topic"`
	Difficulty     string   `json:"difficulty"`
	CompanyTags    []string `json:"company_tags"`
	Question       string   `json:"question"`
	ExpectedPoints []string `json:"expected_points"`
	FollowUpHints  []string `json:"follow_up_hints"`
}

type questionBankScoredResult struct {
	Question QuestionBankQuestion
	Score    int
}

var questionBankCache struct {
	sync.Mutex
	path string
	bank QuestionBank
	err  error
}

func SearchQuestions(topic string, difficulty string, company string, limit int) ([]QuestionBankQuestion, error) {
	return SearchQuestionsFromFile(defaultQuestionBankPath, topic, difficulty, company, limit)
}

func SearchQuestionsFromFile(path string, topic string, difficulty string, company string, limit int) ([]QuestionBankQuestion, error) {
	if limit <= 0 {
		limit = 3
	}
	bank, err := loadQuestionBank(path)
	if err != nil {
		return nil, err
	}
	scored := make([]questionBankScoredResult, 0, len(bank.Questions))
	for _, q := range bank.Questions {
		score := scoreQuestionBankMatch(q, topic, difficulty, company)
		if score <= 0 {
			continue
		}
		scored = append(scored, questionBankScoredResult{Question: q, Score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Question.ID < scored[j].Question.ID
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]QuestionBankQuestion, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.Question)
	}
	return out, nil
}

func loadQuestionBank(path string) (QuestionBank, error) {
	questionBankCache.Lock()
	defer questionBankCache.Unlock()
	if questionBankCache.path == path && (len(questionBankCache.bank.Questions) > 0 || questionBankCache.err != nil) {
		return questionBankCache.bank, questionBankCache.err
	}
	data, err := os.ReadFile(path)
	if err != nil && path == defaultQuestionBankPath {
		data, err = os.ReadFile("../" + defaultQuestionBankPath)
	}
	if err != nil {
		questionBankCache.path = path
		questionBankCache.err = err
		return QuestionBank{}, err
	}
	var bank QuestionBank
	if err := json.Unmarshal(data, &bank); err != nil {
		questionBankCache.path = path
		questionBankCache.err = err
		return QuestionBank{}, err
	}
	questionBankCache.path = path
	questionBankCache.bank = bank
	questionBankCache.err = nil
	return bank, nil
}

func scoreQuestionBankMatch(q QuestionBankQuestion, topic string, difficulty string, company string) int {
	score := 1
	topicText := normalizeSearchText(topic)
	questionText := normalizeSearchText(q.Topic + " " + q.SubTopic + " " + q.Question + " " + strings.Join(q.ExpectedPoints, " "))
	if topicText != "" {
		if strings.Contains(questionText, topicText) || strings.Contains(topicText, normalizeSearchText(q.Topic)) || strings.Contains(topicText, normalizeSearchText(q.SubTopic)) {
			score += 30
		} else {
			return 0
		}
	}
	if difficulty != "" && strings.EqualFold(strings.TrimSpace(q.Difficulty), strings.TrimSpace(difficulty)) {
		score += 8
	}
	companyText := normalizeSearchText(company)
	for _, tag := range q.CompanyTags {
		if companyText != "" && strings.Contains(normalizeSearchText(tag), companyText) {
			score += 10
			break
		}
	}
	return score
}

func formatQuestionBankPromptSection(questions []QuestionBankQuestion) string {
	if len(questions) == 0 {
		return "=== 可选练习题 ===\n（暂无匹配题库题）"
	}
	lines := []string{"=== 可选练习题 ==="}
	for _, q := range questions {
		lines = append(lines, "- ["+q.ID+"] "+q.Question+"（topic: "+q.Topic+" / sub_topic: "+q.SubTopic+" / difficulty: "+q.Difficulty+" / companies: "+strings.Join(q.CompanyTags, ", ")+"）")
		if len(q.ExpectedPoints) > 0 {
			lines = append(lines, "  expected_points: "+strings.Join(q.ExpectedPoints, "、"))
		}
		if len(q.FollowUpHints) > 0 {
			lines = append(lines, "  follow_up_hints: "+strings.Join(q.FollowUpHints, "；"))
		}
	}
	return strings.Join(lines, "\n")
}
