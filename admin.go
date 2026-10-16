package main

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type questionDefinition struct {
	ID           int64  `json:"id"`
	QuestionCode string `json:"questionCode"`
	ReviewArea   string `json:"reviewArea"`
	QuestionText string `json:"questionText"`
	Guidance     string `json:"guidance"`
	SortOrder    int    `json:"sortOrder"`
	Active       bool   `json:"active"`
}

func (application *app) listQuestionDefinitions(response http.ResponseWriter, request *http.Request) {
	rows, err := application.db.Query(request.Context(), "SELECT id,question_code,review_area,question_text,guidance,sort_order,active FROM question_definitions ORDER BY review_area,sort_order,question_code")
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	questions := []questionDefinition{}
	for rows.Next() {
		var item questionDefinition
		if err := rows.Scan(&item.ID, &item.QuestionCode, &item.ReviewArea, &item.QuestionText, &item.Guidance, &item.SortOrder, &item.Active); err != nil {
			handleDBError(response, err)
			return
		}
		questions = append(questions, item)
	}
	writeJSON(response, http.StatusOK, questions)
}

func (application *app) createQuestionDefinition(response http.ResponseWriter, request *http.Request) {
	var input questionDefinition
	if readJSON(request, &input) != nil || !required(input.QuestionCode, input.ReviewArea, input.QuestionText) {
		writeError(response, http.StatusBadRequest, "code, review area, and question are required")
		return
	}
	var id int64
	err := application.db.QueryRow(request.Context(), `INSERT INTO question_definitions(question_code,review_area,question_text,guidance,sort_order,active) VALUES($1,$2,$3,$4,$5,TRUE) RETURNING id`, strings.TrimSpace(input.QuestionCode), input.ReviewArea, strings.TrimSpace(input.QuestionText), strings.TrimSpace(input.Guidance), input.SortOrder).Scan(&id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]int64{"id": id})
}

func (application *app) updateQuestionDefinition(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	var input questionDefinition
	if err != nil || readJSON(request, &input) != nil || !required(input.QuestionCode, input.ReviewArea, input.QuestionText) {
		writeError(response, http.StatusBadRequest, "invalid question")
		return
	}
	result, err := application.db.Exec(request.Context(), `UPDATE question_definitions SET question_code=$1,review_area=$2,question_text=$3,guidance=$4,sort_order=$5,active=$6,updated_at=NOW() WHERE id=$7`, input.QuestionCode, input.ReviewArea, input.QuestionText, input.Guidance, input.SortOrder, input.Active, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	if result.RowsAffected() == 0 {
		writeError(response, http.StatusNotFound, "question not found")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (application *app) listUsers(response http.ResponseWriter, request *http.Request) {
	rows, err := application.db.Query(request.Context(), "SELECT id,name,email,role,active,must_change_password FROM users ORDER BY name")
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	users := []user{}
	for rows.Next() {
		var item user
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.Active, &item.MustChangePassword); err != nil {
			handleDBError(response, err)
			return
		}
		users = append(users, item)
	}
	writeJSON(response, http.StatusOK, users)
}

func (application *app) listActiveUsers(response http.ResponseWriter, request *http.Request) {
	rows, err := application.db.Query(request.Context(), "SELECT id,name,email,role,active,must_change_password FROM users WHERE active=TRUE ORDER BY name")
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	users := []user{}
	for rows.Next() {
		var item user
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.Active, &item.MustChangePassword); err != nil {
			handleDBError(response, err)
			return
		}
		users = append(users, item)
	}
	writeJSON(response, http.StatusOK, users)
}

func (application *app) createUser(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if readJSON(request, &input) != nil || !required(input.Name, input.Email, input.Role) || (input.Role != "admin" && input.Role != "reviewer") {
		writeError(response, http.StatusBadRequest, "name, email, and a valid role are required")
		return
	}
	password, err := temporaryPassword()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "could not create password")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	var id int64
	err = application.db.QueryRow(request.Context(), `INSERT INTO users(name,email,password_hash,role,must_change_password) VALUES($1,$2,$3,$4,TRUE) RETURNING id`, strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Email)), string(hash), input.Role).Scan(&id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"id": id, "temporaryPassword": password})
}

func (application *app) resetPassword(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid user id")
		return
	}
	password, err := temporaryPassword()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "could not create password")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	transaction, err := application.db.Begin(request.Context())
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer func() { _ = transaction.Rollback(request.Context()) }()
	result, err := transaction.Exec(request.Context(), "UPDATE users SET password_hash=$1,must_change_password=TRUE,updated_at=NOW() WHERE id=$2", string(hash), id)
	if err == nil {
		_, err = transaction.Exec(request.Context(), "DELETE FROM sessions WHERE user_id=$1", id)
	}
	if err != nil {
		handleDBError(response, err)
		return
	}
	if result.RowsAffected() == 0 {
		writeError(response, http.StatusNotFound, "user not found")
		return
	}
	if err := transaction.Commit(request.Context()); err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"temporaryPassword": password})
}
