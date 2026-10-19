package main

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

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
