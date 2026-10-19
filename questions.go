package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
)

type controlledQuestion struct {
	QuestionCode string `json:"questionCode"`
	ReviewArea   string `json:"reviewArea"`
	QuestionText string `json:"questionText"`
	Guidance     string `json:"guidance"`
	SortOrder    int    `json:"sortOrder"`
}

func loadQuestions(files fs.FS) ([]controlledQuestion, error) {
	content, err := fs.ReadFile(files, "Web/questions.json")
	if err != nil {
		return nil, fmt.Errorf("read questions: %w", err)
	}
	var questions []controlledQuestion
	if err := json.Unmarshal(content, &questions); err != nil {
		return nil, fmt.Errorf("decode questions: %w", err)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("question catalog is empty")
	}
	seen := make(map[string]bool, len(questions))
	for _, question := range questions {
		if !required(question.QuestionCode, question.ReviewArea, question.QuestionText) {
			return nil, fmt.Errorf("question catalog contains an incomplete question")
		}
		if seen[question.QuestionCode] {
			return nil, fmt.Errorf("question catalog contains duplicate code %s", question.QuestionCode)
		}
		seen[question.QuestionCode] = true
	}
	return questions, nil
}

func (application *app) syncQuestionDefinitions(ctx context.Context) error {
	transaction, err := application.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin question synchronization: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	if _, err := transaction.Exec(ctx, "UPDATE question_definitions SET active=FALSE,updated_at=NOW() WHERE active=TRUE"); err != nil {
		return fmt.Errorf("deactivate question definitions: %w", err)
	}
	for _, question := range application.questions {
		_, err := transaction.Exec(ctx, `
			INSERT INTO question_definitions(question_code,review_area,question_text,guidance,sort_order,active)
			VALUES($1,$2,$3,$4,$5,TRUE)
			ON CONFLICT(question_code) DO UPDATE SET
				review_area=EXCLUDED.review_area,
				question_text=EXCLUDED.question_text,
				guidance=EXCLUDED.guidance,
				sort_order=EXCLUDED.sort_order,
				active=TRUE,
				updated_at=NOW()`,
			question.QuestionCode, question.ReviewArea, question.QuestionText, question.Guidance, question.SortOrder)
		if err != nil {
			return fmt.Errorf("synchronize question %s: %w", question.QuestionCode, err)
		}
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO review_questions (review_id,question_id,question_code,review_area,question_text,guidance,sort_order)
		SELECT r.id,q.id,q.question_code,q.review_area,q.question_text,q.guidance,q.sort_order
		FROM reviews r CROSS JOIN question_definitions q
		WHERE q.active
		ON CONFLICT(review_id,question_id) DO NOTHING`); err != nil {
		return fmt.Errorf("backfill controlled questions: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit question synchronization: %w", err)
	}
	return nil
}
