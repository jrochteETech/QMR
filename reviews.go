package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

var listOptions = map[string][]string{
	"businessUnits":       {"Central", "East", "GL", "MCT", "West"},
	"projectStatuses":     {"Active", "Completed"},
	"reviewTypes":         {"Initial Review", "Follow-Up Review"},
	"reviewStatuses":      {"Not Started", "In Progress", "Completed"},
	"reviewAreas":         {"Project Record Accessibility / Organization", "Sales Handoff / Kickoff", "Project Planning / Workbook Usage", "Risk Management", "Change Management", "Project Monitoring / Review Cadence", "Closeout / Lessons Learned"},
	"results":             {"3 - Meets Expectation", "2 - Partially Meets Expectation", "1 - Does Not Meet Expectation", "N/A - Not Applicable", "UTD - Unable to Determine"},
	"evidenceStatuses":    {"Expected Evidence Found", "Acceptable Alternate Evidence", "Incomplete Evidence", "Missing Evidence", "Inaccessible Evidence", "Misfiled Evidence", "Not Maintained as Expected", "Not Applicable"},
	"rootCauses":          {"Individual Adherence Gap", "Standard Unclear", "Tool / Template Friction", "Training Gap", "Workload / Capacity Constraint", "Governance / Enforcement Gap"},
	"systemRelatedValues": {"Yes", "No", "Further Evaluation Needed"},
	"actionTypes":         {"Project Correction", "Standard / Process", "Tool / Template", "Training / Communication", "Governance", "Further Evaluation"},
	"actionStatuses":      {"Open", "In Progress", "Closed", "Deferred"},
}

type project struct {
	ID                  int64      `json:"id"`
	ProjectCode         string     `json:"projectCode"`
	Name                string     `json:"name"`
	BusinessUnit        string     `json:"businessUnit"`
	Status              string     `json:"status"`
	Customer            string     `json:"customer"`
	ProjectManager      string     `json:"projectManager"`
	StartDate           *time.Time `json:"startDate"`
	EndDate             *time.Time `json:"endDate"`
	Notes               string     `json:"notes"`
	BACLaborRevenue     float64    `json:"bacLaborRevenue"`
	BACLaborCost        float64    `json:"bacLaborCost"`
	BACLaborMargin      *float64   `json:"bacLaborMargin"`
	ACTLabMargin        float64    `json:"actLabMargin"`
	LaborMarginVariance *float64   `json:"laborMarginVariance"`
	ReviewCount         int        `json:"reviewCount"`
	OpenActions         int        `json:"openActions"`
}

type reviewSummary struct {
	ID               int64      `json:"id"`
	ReviewType       string     `json:"reviewType"`
	Status           string     `json:"status"`
	ReviewDate       time.Time  `json:"reviewDate"`
	ReviewerName     string     `json:"reviewerName"`
	CompletedAt      *time.Time `json:"completedAt"`
	Answered         int        `json:"answered"`
	Total            int        `json:"total"`
	TotalTimeMinutes int        `json:"totalTimeMinutes"`
}

type answer struct {
	QuestionID             int64  `json:"questionId"`
	QuestionCode           string `json:"questionCode"`
	ReviewArea             string `json:"reviewArea"`
	QuestionText           string `json:"questionText"`
	Guidance               string `json:"guidance"`
	Result                 string `json:"result"`
	EvidenceStatus         string `json:"evidenceStatus"`
	EvidenceNotes          string `json:"evidenceNotes"`
	PrimaryRootCause       string `json:"primaryRootCause"`
	QualitySystemRelated   string `json:"qualitySystemRelated"`
	ReviewerNotes          string `json:"reviewerNotes"`
	ReviewTimeMinutes      int    `json:"reviewTimeMinutes"`
	PMClarificationUsed    bool   `json:"pmClarificationUsed"`
	PMClarificationSummary string `json:"pmClarificationSummary"`
	FollowUpRequired       bool   `json:"followUpRequired"`
	ActionCount            int    `json:"actionCount"`
}

func (application *app) options(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, listOptions)
}

func (application *app) getInstructions(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, application.instructions)
}

func (application *app) dashboard(response http.ResponseWriter, request *http.Request) {
	var metrics struct {
		ActiveProjects   int     `json:"activeProjects"`
		OpenActions      int     `json:"openActions"`
		OverdueActions   int     `json:"overdueActions"`
		CompletedReviews int     `json:"completedReviews"`
		ConformanceRate  float64 `json:"conformanceRate"`
		EvidenceGapRate  float64 `json:"evidenceGapRate"`
	}
	err := application.db.QueryRow(request.Context(), `
		SELECT
			(SELECT COUNT(*) FROM projects WHERE status = 'Active'),
			(SELECT COUNT(*) FROM actions WHERE status NOT IN ('Closed', 'Deferred')),
			(SELECT COUNT(*) FROM actions WHERE status NOT IN ('Closed', 'Deferred') AND due_date < CURRENT_DATE),
			(SELECT COUNT(*) FROM reviews WHERE status = 'Completed'),
			COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE result LIKE '3 -%') / NULLIF(COUNT(*) FILTER (WHERE result LIKE '1 -%' OR result LIKE '2 -%' OR result LIKE '3 -%'), 0) FROM review_answers), 0),
			COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE evidence_status IN ('Incomplete Evidence', 'Missing Evidence', 'Inaccessible Evidence', 'Misfiled Evidence', 'Not Maintained as Expected')) / NULLIF(COUNT(*) FILTER (WHERE evidence_status <> '' AND evidence_status <> 'Not Applicable'), 0) FROM review_answers), 0)`).
		Scan(&metrics.ActiveProjects, &metrics.OpenActions, &metrics.OverdueActions, &metrics.CompletedReviews, &metrics.ConformanceRate, &metrics.EvidenceGapRate)
	if err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, metrics)
}

func (application *app) listProjects(response http.ResponseWriter, request *http.Request) {
	search := "%" + strings.TrimSpace(request.URL.Query().Get("search")) + "%"
	status := request.URL.Query().Get("status")
	rows, err := application.db.Query(request.Context(), `
		SELECT p.id, p.project_code, p.name, p.business_unit, p.status, p.customer, p.project_manager,
			p.start_date, p.end_date, p.notes, p.bac_labor_revenue, p.bac_labor_cost,
			CASE WHEN p.bac_labor_cost <> 0 THEN 100 * (1 - p.bac_labor_cost / p.bac_labor_revenue) END,
			p.act_lab_margin,
			CASE WHEN p.bac_labor_cost <> 0 THEN p.act_lab_margin - 100 * (1 - p.bac_labor_cost / p.bac_labor_revenue) END,
			COUNT(DISTINCT r.id), COUNT(DISTINCT a.id) FILTER (WHERE a.status NOT IN ('Closed', 'Deferred'))
		FROM projects p
		LEFT JOIN reviews r ON r.project_id = p.id
		LEFT JOIN actions a ON a.review_id = r.id
		WHERE (p.project_code ILIKE $1 OR p.name ILIKE $1 OR p.customer ILIKE $1) AND ($2 = '' OR p.status = $2)
		GROUP BY p.id ORDER BY p.updated_at DESC`, search, status)
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	projects := []project{}
	for rows.Next() {
		var item project
		if err := rows.Scan(&item.ID, &item.ProjectCode, &item.Name, &item.BusinessUnit, &item.Status, &item.Customer, &item.ProjectManager, &item.StartDate, &item.EndDate, &item.Notes, &item.BACLaborRevenue, &item.BACLaborCost, &item.BACLaborMargin, &item.ACTLabMargin, &item.LaborMarginVariance, &item.ReviewCount, &item.OpenActions); err != nil {
			handleDBError(response, err)
			return
		}
		projects = append(projects, item)
	}
	writeJSON(response, http.StatusOK, projects)
}

type projectInput struct {
	ProjectCode     string  `json:"projectCode"`
	Name            string  `json:"name"`
	BusinessUnit    string  `json:"businessUnit"`
	Status          string  `json:"status"`
	Customer        string  `json:"customer"`
	ProjectManager  string  `json:"projectManager"`
	StartDate       string  `json:"startDate"`
	EndDate         string  `json:"endDate"`
	Notes           string  `json:"notes"`
	BACLaborRevenue float64 `json:"bacLaborRevenue"`
	BACLaborCost    float64 `json:"bacLaborCost"`
	ACTLabMargin    float64 `json:"actLabMargin"`
}

func (application *app) createProject(response http.ResponseWriter, request *http.Request) {
	var input projectInput
	if readJSON(request, &input) != nil || !required(input.ProjectCode, input.Name, input.BusinessUnit, input.Status) {
		writeError(response, http.StatusBadRequest, "project code, name, business unit, and status are required")
		return
	}
	startDate, startErr := parseDate(input.StartDate)
	endDate, endErr := parseDate(input.EndDate)
	if startErr != nil || endErr != nil {
		writeError(response, http.StatusBadRequest, "dates must use YYYY-MM-DD")
		return
	}
	var id int64
	err := application.db.QueryRow(request.Context(), `
		INSERT INTO projects (project_code, name, business_unit, status, customer, project_manager, start_date, end_date, notes, bac_labor_revenue, bac_labor_cost, act_lab_margin, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`, strings.TrimSpace(input.ProjectCode), strings.TrimSpace(input.Name), input.BusinessUnit, input.Status, strings.TrimSpace(input.Customer), strings.TrimSpace(input.ProjectManager), startDate, endDate, strings.TrimSpace(input.Notes), input.BACLaborRevenue, input.BACLaborCost, input.ACTLabMargin, currentUser(request).ID).Scan(&id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]int64{"id": id})
}

func (application *app) updateProject(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	var input projectInput
	if err != nil || readJSON(request, &input) != nil || !required(input.ProjectCode, input.Name, input.BusinessUnit, input.Status) {
		writeError(response, http.StatusBadRequest, "invalid project")
		return
	}
	startDate, startErr := parseDate(input.StartDate)
	endDate, endErr := parseDate(input.EndDate)
	if startErr != nil || endErr != nil {
		writeError(response, http.StatusBadRequest, "dates must use YYYY-MM-DD")
		return
	}
	result, err := application.db.Exec(request.Context(), `UPDATE projects SET project_code=$1,name=$2,business_unit=$3,status=$4,customer=$5,project_manager=$6,start_date=$7,end_date=$8,notes=$9,bac_labor_revenue=$10,bac_labor_cost=$11,act_lab_margin=$12,updated_at=NOW() WHERE id=$13`, input.ProjectCode, input.Name, input.BusinessUnit, input.Status, input.Customer, input.ProjectManager, startDate, endDate, input.Notes, input.BACLaborRevenue, input.BACLaborCost, input.ACTLabMargin, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	if result.RowsAffected() == 0 {
		writeError(response, http.StatusNotFound, "project not found")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (application *app) getProject(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid project id")
		return
	}
	var item project
	err = application.db.QueryRow(request.Context(), `SELECT id, project_code, name, business_unit, status, customer, project_manager, start_date, end_date, notes, bac_labor_revenue, bac_labor_cost, CASE WHEN bac_labor_cost <> 0 THEN 100 * (1 - bac_labor_cost / bac_labor_revenue) END, act_lab_margin, CASE WHEN bac_labor_cost <> 0 THEN act_lab_margin - 100 * (1 - bac_labor_cost / bac_labor_revenue) END, 0, 0 FROM projects WHERE id=$1`, id).
		Scan(&item.ID, &item.ProjectCode, &item.Name, &item.BusinessUnit, &item.Status, &item.Customer, &item.ProjectManager, &item.StartDate, &item.EndDate, &item.Notes, &item.BACLaborRevenue, &item.BACLaborCost, &item.BACLaborMargin, &item.ACTLabMargin, &item.LaborMarginVariance, &item.ReviewCount, &item.OpenActions)
	if err != nil {
		handleDBError(response, err)
		return
	}
	rows, err := application.db.Query(request.Context(), `
		SELECT r.id,r.review_type,r.status,r.review_date,u.name,r.completed_at,
			COUNT(a.id) FILTER (WHERE a.result <> ''), COUNT(q.question_id), COALESCE(SUM(a.review_time_minutes),0)
		FROM reviews r JOIN users u ON u.id=r.reviewer_id
		LEFT JOIN review_questions q ON q.review_id = r.id
		LEFT JOIN review_answers a ON a.review_id=r.id AND a.question_id=q.question_id
		WHERE r.project_id=$1 GROUP BY r.id,u.name ORDER BY r.review_date DESC,r.id DESC`, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	reviews := []reviewSummary{}
	for rows.Next() {
		var review reviewSummary
		if err := rows.Scan(&review.ID, &review.ReviewType, &review.Status, &review.ReviewDate, &review.ReviewerName, &review.CompletedAt, &review.Answered, &review.Total, &review.TotalTimeMinutes); err != nil {
			handleDBError(response, err)
			return
		}
		reviews = append(reviews, review)
	}
	writeJSON(response, http.StatusOK, map[string]any{"project": item, "reviews": reviews})
}

func (application *app) createReview(response http.ResponseWriter, request *http.Request) {
	projectID, err := pathID(request, "id")
	var input struct {
		ReviewType string `json:"reviewType"`
		ReviewDate string `json:"reviewDate"`
	}
	if err != nil || readJSON(request, &input) != nil || !required(input.ReviewType, input.ReviewDate) {
		writeError(response, http.StatusBadRequest, "review type and date are required")
		return
	}
	reviewDate, err := parseDate(input.ReviewDate)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid review date")
		return
	}
	transaction, err := application.db.Begin(request.Context())
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer func() { _ = transaction.Rollback(request.Context()) }()
	var id int64
	err = transaction.QueryRow(request.Context(), `INSERT INTO reviews(project_id,review_type,review_date,reviewer_id) VALUES($1,$2,$3,$4) RETURNING id`, projectID, input.ReviewType, reviewDate, currentUser(request).ID).Scan(&id)
	if err == nil {
		_, err = transaction.Exec(request.Context(), `INSERT INTO review_questions(review_id,question_id,question_code,review_area,question_text,guidance,sort_order) SELECT $1,id,question_code,review_area,question_text,guidance,sort_order FROM question_definitions WHERE active`, id)
	}
	if err != nil {
		handleDBError(response, err)
		return
	}
	if err := transaction.Commit(request.Context()); err != nil {
		handleDBError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]int64{"id": id})
}

func (application *app) getReview(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid review id")
		return
	}
	var summary reviewSummary
	var projectID int64
	var projectName string
	err = application.db.QueryRow(request.Context(), `SELECT r.id,r.review_type,r.status,r.review_date,u.name,r.completed_at,r.project_id,p.name FROM reviews r JOIN users u ON u.id=r.reviewer_id JOIN projects p ON p.id=r.project_id WHERE r.id=$1`, id).
		Scan(&summary.ID, &summary.ReviewType, &summary.Status, &summary.ReviewDate, &summary.ReviewerName, &summary.CompletedAt, &projectID, &projectName)
	if err != nil {
		handleDBError(response, err)
		return
	}
	rows, err := application.db.Query(request.Context(), `
		SELECT q.question_id,q.question_code,q.review_area,q.question_text,q.guidance,
			COALESCE(a.result,''),COALESCE(a.evidence_status,''),COALESCE(a.evidence_notes,''),COALESCE(a.primary_root_cause,''),COALESCE(a.quality_system_related,''),COALESCE(a.reviewer_notes,''),COALESCE(a.review_time_minutes,0),
			COALESCE(a.pm_clarification_used,FALSE),COALESCE(a.pm_clarification_summary,''),COALESCE(a.follow_up_required,FALSE),
			(SELECT COUNT(*) FROM actions ac WHERE ac.review_id=$1 AND ac.question_id=q.question_id)
		FROM review_questions q LEFT JOIN review_answers a ON a.question_id=q.question_id AND a.review_id=q.review_id
		WHERE q.review_id=$1 ORDER BY q.sort_order,q.question_code`, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer rows.Close()
	answers := []answer{}
	for rows.Next() {
		var item answer
		if err := rows.Scan(&item.QuestionID, &item.QuestionCode, &item.ReviewArea, &item.QuestionText, &item.Guidance, &item.Result, &item.EvidenceStatus, &item.EvidenceNotes, &item.PrimaryRootCause, &item.QualitySystemRelated, &item.ReviewerNotes, &item.ReviewTimeMinutes, &item.PMClarificationUsed, &item.PMClarificationSummary, &item.FollowUpRequired, &item.ActionCount); err != nil {
			handleDBError(response, err)
			return
		}
		answers = append(answers, item)
	}
	summary.Total = len(answers)
	for _, item := range answers {
		if item.Result != "" {
			summary.Answered++
		}
	}
	actionRows, err := application.db.Query(request.Context(), `
		SELECT a.id,a.review_id,a.question_id,p.id,p.project_code,p.name,q.question_code,q.question_text,a.action_type,a.description,a.owner_id,u.name,a.due_date,a.status,a.closure_evidence,a.closed_date,a.notes,a.created_at
		FROM actions a JOIN reviews r ON r.id=a.review_id JOIN projects p ON p.id=r.project_id JOIN review_questions q ON q.review_id=a.review_id AND q.question_id=a.question_id JOIN users u ON u.id=a.owner_id
		WHERE a.review_id=$1 ORDER BY a.question_id,a.created_at`, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	defer actionRows.Close()
	actions := []actionItem{}
	for actionRows.Next() {
		var item actionItem
		if err := actionRows.Scan(&item.ID, &item.ReviewID, &item.QuestionID, &item.ProjectID, &item.ProjectCode, &item.ProjectName, &item.QuestionCode, &item.QuestionText, &item.ActionType, &item.Description, &item.OwnerID, &item.OwnerName, &item.DueDate, &item.Status, &item.ClosureEvidence, &item.ClosedDate, &item.Notes, &item.CreatedAt); err != nil {
			handleDBError(response, err)
			return
		}
		actions = append(actions, item)
	}
	writeJSON(response, http.StatusOK, map[string]any{"review": summary, "projectId": projectID, "projectName": projectName, "answers": answers, "actions": actions})
}

func (application *app) saveAnswer(response http.ResponseWriter, request *http.Request) {
	reviewID, reviewErr := strconv.ParseInt(request.PathValue("reviewID"), 10, 64)
	questionID, questionErr := strconv.ParseInt(request.PathValue("questionID"), 10, 64)
	var input answer
	if reviewErr != nil || questionErr != nil || readJSON(request, &input) != nil || input.ReviewTimeMinutes < 0 {
		writeError(response, http.StatusBadRequest, "invalid answer")
		return
	}
	_, err := application.db.Exec(request.Context(), `
		INSERT INTO review_answers(review_id,question_id,result,evidence_status,evidence_notes,primary_root_cause,quality_system_related,reviewer_notes,review_time_minutes,pm_clarification_used,pm_clarification_summary,follow_up_required,updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT(review_id,question_id) DO UPDATE SET result=EXCLUDED.result,evidence_status=EXCLUDED.evidence_status,evidence_notes=EXCLUDED.evidence_notes,primary_root_cause=EXCLUDED.primary_root_cause,quality_system_related=EXCLUDED.quality_system_related,reviewer_notes=EXCLUDED.reviewer_notes,review_time_minutes=EXCLUDED.review_time_minutes,pm_clarification_used=EXCLUDED.pm_clarification_used,pm_clarification_summary=EXCLUDED.pm_clarification_summary,follow_up_required=EXCLUDED.follow_up_required,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, reviewID, questionID, input.Result, input.EvidenceStatus, input.EvidenceNotes, input.PrimaryRootCause, input.QualitySystemRelated, input.ReviewerNotes, input.ReviewTimeMinutes, input.PMClarificationUsed, conditionalText(input.PMClarificationUsed, input.PMClarificationSummary), input.FollowUpRequired, currentUser(request).ID)
	if err != nil {
		handleDBError(response, err)
		return
	}
	_, _ = application.db.Exec(request.Context(), "UPDATE reviews SET status=CASE WHEN status='Not Started' THEN 'In Progress' ELSE status END,updated_at=NOW() WHERE id=$1", reviewID)
	response.WriteHeader(http.StatusNoContent)
}

func (application *app) updateReviewStatus(response http.ResponseWriter, request *http.Request) {
	id, err := pathID(request, "id")
	var input struct {
		Status string `json:"status"`
	}
	if err != nil || readJSON(request, &input) != nil || !contains(listOptions["reviewStatuses"], input.Status) {
		writeError(response, http.StatusBadRequest, "invalid review status")
		return
	}
	_, err = application.db.Exec(request.Context(), `UPDATE reviews SET status=$1,completed_at=CASE WHEN $1='Completed' THEN NOW() ELSE NULL END,updated_at=NOW() WHERE id=$2`, input.Status, id)
	if err != nil {
		handleDBError(response, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func conditionalText(enabled bool, value string) string {
	if !enabled {
		return ""
	}
	return strings.TrimSpace(value)
}
