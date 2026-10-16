package main

import (
	"net/http"
	"strings"
	"time"
)

type resultRow struct {
	ReviewID             int64     `json:"reviewId"`
	QuestionID           int64     `json:"questionId"`
	ProjectID            int64     `json:"projectId"`
	ProjectCode          string    `json:"projectCode"`
	ProjectName          string    `json:"projectName"`
	ReviewType           string    `json:"reviewType"`
	ReviewStatus         string    `json:"reviewStatus"`
	ReviewDate           time.Time `json:"reviewDate"`
	QuestionCode         string    `json:"questionCode"`
	ReviewArea           string    `json:"reviewArea"`
	QuestionText         string    `json:"questionText"`
	Result               string    `json:"result"`
	EvidenceStatus       string    `json:"evidenceStatus"`
	PrimaryRootCause     string    `json:"primaryRootCause"`
	QualitySystemRelated string    `json:"qualitySystemRelated"`
	ActionCount          int       `json:"actionCount"`
}

type actionItem struct {
	ID              int64      `json:"id"`
	ReviewID        int64      `json:"reviewId"`
	QuestionID      int64      `json:"questionId"`
	ProjectID       int64      `json:"projectId"`
	ProjectCode     string     `json:"projectCode"`
	ProjectName     string     `json:"projectName"`
	QuestionCode    string     `json:"questionCode"`
	QuestionText    string     `json:"questionText"`
	ActionType      string     `json:"actionType"`
	Description     string     `json:"description"`
	OwnerID         int64      `json:"ownerId"`
	OwnerName       string     `json:"ownerName"`
	DueDate         *time.Time `json:"dueDate"`
	Status          string     `json:"status"`
	ClosureEvidence string     `json:"closureEvidence"`
	ClosedDate      *time.Time `json:"closedDate"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"createdAt"`
}

func (application *app) listResults(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	search := "%" + strings.TrimSpace(query.Get("search")) + "%"
	rows, err := application.db.Query(request.Context(), `
		SELECT r.id,q.question_id,p.id,p.project_code,p.name,r.review_type,r.status,r.review_date,q.question_code,q.review_area,q.question_text,
			COALESCE(a.result,''),COALESCE(a.evidence_status,''),COALESCE(a.primary_root_cause,''),COALESCE(a.quality_system_related,''),
			(SELECT COUNT(*) FROM actions ac WHERE ac.review_id=r.id AND ac.question_id=q.question_id)
		FROM reviews r JOIN projects p ON p.id=r.project_id JOIN review_questions q ON q.review_id=r.id
		LEFT JOIN review_answers a ON a.review_id=r.id AND a.question_id=q.question_id
		WHERE TRUE
			AND (p.project_code ILIKE $1 OR p.name ILIKE $1 OR q.question_code ILIKE $1 OR q.question_text ILIKE $1)
			AND ($2='' OR q.review_area=$2) AND ($3='' OR COALESCE(a.result,'')=$3) AND ($4='' OR r.status=$4)
		ORDER BY r.review_date DESC,p.project_code,q.review_area,q.sort_order,q.question_code`, search, query.Get("area"), query.Get("result"), query.Get("status"))
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	results := []resultRow{}
	for rows.Next() {
		var item resultRow
		if err := rows.Scan(&item.ReviewID, &item.QuestionID, &item.ProjectID, &item.ProjectCode, &item.ProjectName, &item.ReviewType, &item.ReviewStatus, &item.ReviewDate, &item.QuestionCode, &item.ReviewArea, &item.QuestionText, &item.Result, &item.EvidenceStatus, &item.PrimaryRootCause, &item.QualitySystemRelated, &item.ActionCount); err != nil {
			handleDBError(response, err)
			return
		}
		results = append(results, item)
	}
	writeJSON(response, http.StatusOK, results)
}

func (application *app) listActions(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	search := "%" + strings.TrimSpace(query.Get("search")) + "%"
	rows, err := application.db.Query(request.Context(), `
		SELECT a.id,a.review_id,a.question_id,p.id,p.project_code,p.name,q.question_code,q.question_text,a.action_type,a.description,a.owner_id,u.name,a.due_date,a.status,a.closure_evidence,a.closed_date,a.notes,a.created_at
		FROM actions a JOIN reviews r ON r.id=a.review_id JOIN projects p ON p.id=r.project_id JOIN review_questions q ON q.review_id=a.review_id AND q.question_id=a.question_id JOIN users u ON u.id=a.owner_id
		WHERE (p.project_code ILIKE $1 OR p.name ILIKE $1 OR q.question_text ILIKE $1 OR a.description ILIKE $1 OR u.name ILIKE $1)
			AND ($2='' OR a.status=$2) AND ($3='' OR a.owner_id::text=$3) AND ($4='' OR a.action_type=$4)
		ORDER BY CASE WHEN a.status IN ('Closed','Deferred') THEN 1 ELSE 0 END,a.due_date NULLS LAST,a.created_at DESC`, search, query.Get("status"), query.Get("owner"), query.Get("type"))
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	actions := []actionItem{}
	for rows.Next() {
		var item actionItem
		if err := rows.Scan(&item.ID, &item.ReviewID, &item.QuestionID, &item.ProjectID, &item.ProjectCode, &item.ProjectName, &item.QuestionCode, &item.QuestionText, &item.ActionType, &item.Description, &item.OwnerID, &item.OwnerName, &item.DueDate, &item.Status, &item.ClosureEvidence, &item.ClosedDate, &item.Notes, &item.CreatedAt); err != nil {
			handleDBError(response, err)
			return
		}
		actions = append(actions, item)
	}
	writeJSON(response, http.StatusOK, actions)
}

type actionInput struct {
	ReviewID        int64  `json:"reviewId"`
	QuestionID      int64  `json:"questionId"`
	ActionType      string `json:"actionType"`
	Description     string `json:"description"`
	OwnerID         int64  `json:"ownerId"`
	DueDate         string `json:"dueDate"`
	Status          string `json:"status"`
	ClosureEvidence string `json:"closureEvidence"`
	Notes           string `json:"notes"`
}

func (application *app) createAction(response http.ResponseWriter, request *http.Request) {
	var input actionInput
	if readJSON(request, &input) != nil || input.ReviewID == 0 || input.QuestionID == 0 || input.OwnerID == 0 || !required(input.ActionType, input.Description) {
		writeError(response, http.StatusBadRequest, "review, question, type, description, and owner are required")
		return
	}
	dueDate, err := parseDate(input.DueDate)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid due date")
		return
	}
	var id int64
	err = application.db.QueryRow(request.Context(), `INSERT INTO actions(review_id,question_id,action_type,description,owner_id,due_date,status,closure_evidence,notes,created_by) VALUES($1,$2,$3,$4,$5,$6,'Open','',$7,$8) RETURNING id`, input.ReviewID, input.QuestionID, input.ActionType, strings.TrimSpace(input.Description), input.OwnerID, dueDate, strings.TrimSpace(input.Notes), currentUser(request).ID).Scan(&id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]int64{"id": id})
}

func (application *app) updateAction(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	var input actionInput
	if err != nil || readJSON(request, &input) != nil || input.OwnerID == 0 || !required(input.ActionType, input.Description, input.Status) {
		writeError(response, http.StatusBadRequest, "invalid action")
		return
	}
	dueDate, dateErr := parseDate(input.DueDate)
	if dateErr != nil {
		writeError(response, http.StatusBadRequest, "invalid due date")
		return
	}
	var existingOwnerID int64
	if err := application.db.QueryRow(request.Context(), "SELECT owner_id FROM actions WHERE id=$1", id).Scan(&existingOwnerID); err != nil {
		handleDBError(response, err)
		return
	}
	current := currentUser(request)
	if current.Role != "admin" && existingOwnerID != current.ID {
		writeError(response, http.StatusForbidden, "only the action owner or an administrator can update this action")
		return
	}
	var closedDate any
	if input.Status == "Closed" {
		closedDate = time.Now()
	}
	result, err := application.db.Exec(request.Context(), `UPDATE actions SET action_type=$1,description=$2,owner_id=$3,due_date=$4,status=$5,closure_evidence=$6,closed_date=$7,notes=$8,updated_at=NOW() WHERE id=$9`, input.ActionType, input.Description, input.OwnerID, dueDate, input.Status, input.ClosureEvidence, closedDate, input.Notes, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	if result.RowsAffected() == 0 {
		writeError(response, http.StatusNotFound, "action not found")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}
