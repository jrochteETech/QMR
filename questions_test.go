package main

import "testing"

func TestLoadQuestions(t *testing.T) {
	questions, err := loadQuestions(assets)
	if err != nil {
		t.Fatalf("loadQuestions() error = %v", err)
	}
	if len(questions) != 21 {
		t.Fatalf("expected 21 controlled questions, got %d", len(questions))
	}
	if questions[0].QuestionCode != "1.1" {
		t.Fatalf("expected first question code 1.1, got %q", questions[0].QuestionCode)
	}
	for _, question := range questions {
		if question.Guidance == "" {
			t.Fatalf("question %s has no guidance", question.QuestionCode)
		}
	}
}
