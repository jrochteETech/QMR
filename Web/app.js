const state = {
  user: null,
  options: {},
  users: [],
  currentReview: null,
};

const appRoot = document.querySelector("#app");
const drawerRoot = document.querySelector("#drawer-root");
const modal = document.querySelector("#modal");
const toast = document.querySelector("#toast");

const icons = {
  dashboard: "&#9638;",
  projects: "&#9636;",
  questions: "?",
  actions: "&#10003;",
  instructions: "&#8505;",
  admin: "&#9881;",
};

async function api(path, options = {}) {
  const config = { ...options, headers: { ...(options.headers || {}) } };
  if (config.body && typeof config.body !== "string") {
    config.headers["Content-Type"] = "application/json";
    config.body = JSON.stringify(config.body);
  }
  const response = await fetch(path, config);
  if (response.status === 204) return null;
  let payload = null;
  try { payload = await response.json(); } catch { payload = null; }
  if (!response.ok) {
    const error = new Error(payload?.error || "Request failed");
    error.status = response.status;
    throw error;
  }
  return payload;
}

function escapeHTML(value = "") {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function dateValue(value) { return value ? String(value).slice(0, 10) : ""; }
function formatDate(value) {
  if (!value) return "-";
  const [year, month, day] = dateValue(value).split("-");
  return `${month}/${day}/${year}`;
}
function formatPercent(value) { return `${Number(value || 0).toFixed(1)}%`; }
function formatMoney(value) { return Number(value || 0).toLocaleString("en-US", { style: "currency", currency: "USD" }); }
function formatMinutes(value) {
  const minutes = Number(value || 0);
  const hours = Math.floor(minutes / 60);
  return hours ? `${hours}h ${minutes % 60}m` : `${minutes}m`;
}
function optionList(values = [], selected = "", blank = "Select") {
  return `<option value="">${escapeHTML(blank)}</option>${values.map(value => `<option value="${escapeHTML(value)}" ${value === selected ? "selected" : ""}>${escapeHTML(value)}</option>`).join("")}`;
}
function userOptions(selected = "") {
  return `<option value="">Select owner</option>${state.users.map(user => `<option value="${user.id}" ${String(user.id) === String(selected) ? "selected" : ""}>${escapeHTML(user.name)}</option>`).join("")}`;
}
function statusBadge(status) {
  let tone = "gray";
  if (["Active", "Completed", "Closed", "3 - Meets Expectation"].includes(status)) tone = "green";
  if (["In Progress", "Open", "2 - Partially Meets Expectation"].includes(status)) tone = "orange";
  if (["1 - Does Not Meet Expectation"].includes(status)) tone = "red";
  return `<span class="badge ${tone}">${escapeHTML(status || "Not answered")}</span>`;
}
function notify(message, isError = false) {
  toast.textContent = message;
  toast.className = isError ? "show error" : "show";
  window.setTimeout(() => { toast.className = ""; }, 3200);
}
function showError(error) { notify(error.message || "Something went wrong", true); }

function renderLogin(message = "") {
  appRoot.innerHTML = `
    <main class="login-page">
      <section class="login-panel">
        <div class="login-brand"><img class="brand-mark" src="/brand-logo" alt="E Tech Group"><span class="brand-wordmark">E Tech Group</span></div>
        <h1>Quality Management Review</h1>
        <p>Sign in to review project quality, document evidence, and manage corrective actions.</p>
        <form id="login-form" class="form-grid">
          <div class="form-field full"><label class="required" for="email">Email</label><input class="field" id="email" name="email" type="email" autocomplete="username" required autofocus></div>
          <div class="form-field full"><label class="required" for="password">Password</label><input class="field" id="password" name="password" type="password" autocomplete="current-password" required></div>
          <div class="form-error full">${escapeHTML(message)}</div>
          <button class="button full" type="submit">Sign in</button>
        </form>
      </section>
      <section class="login-art" aria-hidden="true"></section>
    </main>`;
  document.querySelector("#login-form").addEventListener("submit", async event => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      state.user = await api("/api/auth/login", { method: "POST", body: { email: form.get("email"), password: form.get("password") } });
      if (state.user.mustChangePassword) {
        showPasswordChange(true);
      } else {
        await loadReferenceData();
        location.hash = "#/dashboard";
        route();
      }
    } catch (error) { renderLogin(error.message); }
  });
}

function showPasswordChange(required = false) {
  modal.innerHTML = `<form id="password-form" class="modal-content">
    <h2>${required ? "Set a new password" : "Change password"}</h2>
    <p class="hint">Use at least 12 characters. Your temporary password cannot be used again.</p>
    <div class="form-grid">
      <div class="form-field full"><label class="required">Current password</label><input class="field" name="currentPassword" aria-label="Current password" type="password" required></div>
      <div class="form-field full"><label class="required">New password</label><input class="field" name="newPassword" aria-label="New password" type="password" minlength="12" required></div>
      <div class="form-error full"></div>
    </div>
    <div class="modal-actions">${required ? "" : '<button class="button secondary" type="button" data-close-modal>Cancel</button>'}<button class="button" type="submit">Update password</button></div>
  </form>`;
  modal.showModal();
  document.querySelector("#password-form").addEventListener("submit", async event => {
    event.preventDefault();
    const passwordForm = event.currentTarget;
    const submitButton = passwordForm.querySelector('button[type="submit"]');
    const errorMessage = passwordForm.querySelector(".form-error");
    const form = new FormData(passwordForm);
    errorMessage.textContent = "";
    submitButton.disabled = true;
    submitButton.textContent = "Updating...";
    try {
      state.user = await api("/api/auth/change-password", { method: "POST", body: Object.fromEntries(form) });
      modal.close();
      await loadReferenceData();
      location.hash = "#/dashboard";
      route();
      notify("Password updated");
    } catch (error) {
      errorMessage.textContent = error.message;
    } finally {
      submitButton.disabled = false;
      submitButton.textContent = "Update password";
    }
  });
}

async function loadReferenceData() {
  [state.options, state.users] = await Promise.all([api("/api/options"), api("/api/users")]);
}

function shell(active, title, subtitle, content, actions = "", breadcrumbs = "Quality Management Review") {
  const adminLink = state.user.role === "admin" ? `<a href="#/admin" class="${active === "admin" ? "active" : ""}"><span class="nav-icon">${icons.admin}</span><span class="nav-label">Administration</span></a>` : "";
  appRoot.innerHTML = `<div class="shell">
    <aside class="sidebar" id="sidebar">
      <div class="sidebar-brand"><img class="brand-mark" src="/brand-logo" alt="E Tech Group"><span>Quality Review</span></div>
      <nav class="nav" aria-label="Primary">
        <a href="#/dashboard" class="${active === "dashboard" ? "active" : ""}"><span class="nav-icon">${icons.dashboard}</span><span class="nav-label">Dashboard</span></a>
        <a href="#/projects" class="${active === "projects" ? "active" : ""}"><span class="nav-icon">${icons.projects}</span><span class="nav-label">Projects</span></a>
        <a href="#/questions" class="${active === "questions" ? "active" : ""}"><span class="nav-icon">${icons.questions}</span><span class="nav-label">All Questions</span></a>
        <a href="#/actions" class="${active === "actions" ? "active" : ""}"><span class="nav-icon">${icons.actions}</span><span class="nav-label">Action Items</span></a>
        <a href="#/instructions" class="${active === "instructions" ? "active" : ""}"><span class="nav-icon">${icons.instructions}</span><span class="nav-label">Instructions</span></a>
        ${adminLink}
      </nav>
      <div class="sidebar-user"><strong>${escapeHTML(state.user.name)}</strong><small>${escapeHTML(state.user.role)}</small><button class="logout-button" data-logout>Sign out</button></div>
    </aside>
    <main class="main">
      <div class="mobile-bar"><button class="icon-button" data-menu aria-label="Open navigation">&#9776;</button><img class="brand-mark" src="/brand-logo" alt="E Tech Group"><span></span></div>
      <header class="topbar"><div class="breadcrumbs">${breadcrumbs}</div><button class="button secondary small" data-change-password>Change password</button></header>
      <div class="content"><div class="page-head"><div><h1>${escapeHTML(title)}</h1><p>${escapeHTML(subtitle)}</p></div><div class="page-actions">${actions}</div></div>${content}</div>
    </main>
  </div>`;
}

function loading() { appRoot.innerHTML = '<div class="loading">Loading...</div>'; }
function emptyRow(columns, title, detail) { return `<tr><td colspan="${columns}"><div class="empty"><strong>${escapeHTML(title)}</strong>${escapeHTML(detail)}</div></td></tr>`; }

async function renderDashboard() {
  const [metrics, actions] = await Promise.all([api("/api/dashboard"), api("/api/actions")]);
  const activeActions = actions.filter(item => item.status === "Open" || item.status === "In Progress");
  const actionRows = activeActions.slice(0, 6).map(item => `<tr data-link="#/actions/${item.id}"><td><span class="cell-title">${escapeHTML(item.description)}</span><span class="cell-sub">${escapeHTML(item.projectCode)} / ${escapeHTML(item.questionCode)}</span></td><td>${escapeHTML(item.ownerName)}</td><td>${formatDate(item.dueDate)}</td><td>${statusBadge(item.status)}</td></tr>`).join("");
  shell("dashboard", "Quality overview", "Current review health across all projects", `
    <section class="metric-grid">
      <div class="metric"><span class="metric-label">Active projects</span><strong class="metric-value">${metrics.activeProjects}</strong><span class="metric-foot">Projects in review scope</span></div>
      <div class="metric"><span class="metric-label">Conformance rate</span><strong class="metric-value">${formatPercent(metrics.conformanceRate)}</strong><span class="metric-foot">Applicable questions meeting expectations</span></div>
      <div class="metric warning"><span class="metric-label">Open actions</span><strong class="metric-value">${metrics.openActions}</strong><span class="metric-foot">${metrics.overdueActions} overdue</span></div>
      <div class="metric warning"><span class="metric-label">Evidence gap rate</span><strong class="metric-value">${formatPercent(metrics.evidenceGapRate)}</strong><span class="metric-foot">Applicable reviewed questions</span></div>
    </section>
    <section class="panel"><div class="panel-head"><h2>Open action items</h2><a class="button secondary small" href="#/actions">View all</a></div><div class="table-wrap"><table><thead><tr><th>Action</th><th>Owner</th><th>Due</th><th>Status</th></tr></thead><tbody>${actionRows || emptyRow(4, "No open actions", "Open actions will appear here.")}</tbody></table></div></section>`);
}

async function renderProjects() {
  const adminAction = state.user.role === "admin" ? '<button class="button" data-new-project>+ New project</button>' : "";
  shell("projects", "Projects", "Review activity and outstanding actions by project", '<section class="panel"><div class="toolbar"><input class="search" id="project-search" type="search" placeholder="Search code, project, or customer"><select id="project-status"><option value="">All statuses</option>'+optionList(state.options.projectStatuses, "", "All statuses").replace('<option value="">All statuses</option>', '')+'</select></div><div class="table-wrap"><table><thead><tr><th>Project</th><th>Business unit</th><th>Customer</th><th>Status</th><th>BAC Labor Margin</th><th>ACT LAB Margin</th><th>Reviews</th><th>Open actions</th></tr></thead><tbody id="project-rows"></tbody></table></div></section>', adminAction);
  const refresh = async () => {
    const params = new URLSearchParams({ search: document.querySelector("#project-search").value, status: document.querySelector("#project-status").value });
    try {
      const projects = await api(`/api/projects?${params}`);
      document.querySelector("#project-rows").innerHTML = projects.map(item => `<tr data-link="#/projects/${item.id}"><td><span class="code">${escapeHTML(item.projectCode)}</span><span class="cell-sub">${escapeHTML(item.name)}</span></td><td>${escapeHTML(item.businessUnit)}</td><td>${escapeHTML(item.customer || "-")}</td><td>${statusBadge(item.status)}</td><td>${item.bacLaborMargin}</td><td>${item.actLabMargin}</td><td>${item.reviewCount}</td><td>${item.openActions}</td></tr>`).join("") || emptyRow(8, "No projects found", "Adjust the filters or create a project.");
    } catch (error) { showError(error); }
  };
  document.querySelector("#project-search").addEventListener("input", debounce(refresh));
  document.querySelector("#project-status").addEventListener("change", refresh);
  document.querySelector("[data-new-project]")?.addEventListener("click", () => openProjectDrawer());
  await refresh();
}

function projectForm(project = {}) {
  return `<form id="project-form" class="form-grid">
    <div class="form-field"><label class="required">Project code</label><input class="field" name="projectCode" aria-label="Project code" value="${escapeHTML(project.projectCode)}" required></div>
    <div class="form-field"><label class="required">Project name</label><input class="field" name="name" aria-label="Project name" value="${escapeHTML(project.name)}" required></div>
    <div class="form-field"><label class="required">Business unit</label><select name="businessUnit" aria-label="Business unit" required>${optionList(state.options.businessUnits, project.businessUnit)}</select></div>
    <div class="form-field"><label class="required">Status</label><select name="status" aria-label="Project status" required>${optionList(state.options.projectStatuses, project.status || "Active")}</select></div>
    <div class="form-field"><label>Customer</label><input class="field" name="customer" aria-label="Customer" value="${escapeHTML(project.customer)}"></div>
    <div class="form-field"><label>Project manager</label><input class="field" name="projectManager" aria-label="Project manager" value="${escapeHTML(project.projectManager)}"></div>
    <div class="form-field"><label>Start date</label><input class="field" name="startDate" aria-label="Start date" type="date" value="${dateValue(project.startDate)}"></div>
    <div class="form-field"><label>End date</label><input class="field" name="endDate" aria-label="End date" type="date" value="${dateValue(project.endDate)}"></div>
    <div class="section-label">Labor financials</div>
    <div class="form-field"><label>BAC Labor Revenue ($)</label><input class="field" name="bacLaborRevenue" aria-label="BAC Labor Revenue" type="number" min="0" step="0.01" value="${project.bacLaborRevenue ?? 0}"></div>
    <div class="form-field"><label>BAC Labor Cost ($)</label><input class="field" name="bacLaborCost" aria-label="BAC Labor Cost" type="number" min="0" step="0.01" value="${project.bacLaborCost ?? 0}"></div>
    <div class="form-field"><label>BAC Labor Margin (%)</label><output class="calculated-field" data-bac-margin>${project.bacLaborMargin == null ? "-" : formatPercent(project.bacLaborMargin)}</output></div>
    <div class="form-field"><label>ACT LAB Margin (%)</label><input class="field" name="actLabMargin" aria-label="ACT LAB Margin" type="number" step="0.01" value="${project.actLabMargin ?? 0}"></div>
    <div class="form-field"><label>Labor Margin Variance (%)</label><output class="calculated-field" data-margin-variance>${project.laborMarginVariance == null ? "-" : formatPercent(project.laborMarginVariance)}</output></div>
    <div class="form-field full"><label>Notes</label><textarea name="notes" aria-label="Project notes">${escapeHTML(project.notes)}</textarea></div>
    <div class="form-error full"></div>
  </form>`;
}

function openProjectDrawer(project = null) {
  openDrawer(project ? "Edit project" : "New project", project ? project.projectCode : "Add a project to the review program", projectForm(project || {}), `<button class="button secondary" data-close-drawer>Cancel</button><button class="button" type="submit" form="project-form">${project ? "Save changes" : "Create project"}</button>`);
  document.querySelector("#project-form").addEventListener("submit", async event => {
    event.preventDefault();
    const formElement = event.currentTarget;
    const payload = Object.fromEntries(new FormData(formElement));
    ["bacLaborRevenue", "bacLaborCost", "actLabMargin"].forEach(name => { payload[name] = Number(payload[name] || 0); });
    try {
      const result = await api(project ? `/api/projects/${project.id}` : "/api/projects", { method: project ? "PUT" : "POST", body: payload });
      closeDrawer();
      notify(project ? "Project updated" : "Project created");
      if (project) renderProject(project.id); else location.hash = `#/projects/${result.id}`;
    } catch (error) { formElement.querySelector(".form-error").textContent = error.message; }
  });
  const updateMargins = () => {
    const form = document.querySelector("#project-form");
    const revenue = Number(form.elements.bacLaborRevenue.value);
    const cost = Number(form.elements.bacLaborCost.value);
    const actual = Number(form.elements.actLabMargin.value);
    const margin = cost ? 100 * (1 - revenue / cost) : null;
    form.querySelector("[data-bac-margin]").textContent = margin == null ? "-" : formatPercent(margin);
    form.querySelector("[data-margin-variance]").textContent = margin == null ? "-" : formatPercent(actual - margin);
  };
  ["bacLaborRevenue", "bacLaborCost", "actLabMargin"].forEach(name => document.querySelector(`#project-form [name="${name}"]`).addEventListener("input", updateMargins));
}

async function renderProject(id) {
  const data = await api(`/api/projects/${id}`);
  const project = data.project;
  const edit = state.user.role === "admin" ? '<button class="button secondary" data-edit-project>Edit project</button>' : "";
  const reviewRows = data.reviews.map(review => {
    return `<tr data-link="#/reviews/${review.id}"><td><span class="cell-title">${escapeHTML(review.reviewType)}</span><span class="cell-sub">${formatDate(review.reviewDate)}</span></td><td>${escapeHTML(review.reviewerName)}</td><td>${statusBadge(review.status)}</td><td><span class="cell-sub">${review.answered} of ${review.total}</span><progress class="progress" value="${review.answered}" max="${review.total || 1}" aria-label="${review.answered} of ${review.total} questions answered"></progress></td><td>${formatMinutes(review.totalTimeMinutes)}</td></tr>`;
  }).join("");
  shell("projects", project.name, `${project.projectCode} / ${project.customer || "No customer recorded"}`, `
    <section class="project-banner"><div><span class="detail-label">Business unit</span><span class="detail-value">${escapeHTML(project.businessUnit)}</span></div><div><span class="detail-label">Project manager</span><span class="detail-value">${escapeHTML(project.projectManager || "-")}</span></div><div><span class="detail-label">Project dates</span><span class="detail-value">${formatDate(project.startDate)} - ${formatDate(project.endDate)}</span></div><div><span class="detail-label">Status</span>${statusBadge(project.status)}</div></section>
    <section class="project-banner financial-banner"><div><span class="detail-label">BAC Labor Revenue</span><span class="detail-value">${formatMoney(project.bacLaborRevenue)}</span></div><div><span class="detail-label">BAC Labor Cost</span><span class="detail-value">${formatMoney(project.bacLaborCost)}</span></div><div><span class="detail-label">BAC Labor Margin</span><span class="detail-value">${project.bacLaborMargin == null ? "-" : formatPercent(project.bacLaborMargin)}</span></div><div><span class="detail-label">ACT LAB Margin</span><span class="detail-value">${formatPercent(project.actLabMargin)}</span></div><div><span class="detail-label">Labor Margin Variance</span><span class="detail-value">${project.laborMarginVariance == null ? "-" : formatPercent(project.laborMarginVariance)}</span></div></section>
    <section class="panel"><div class="panel-head"><h2>Review history</h2></div><div class="table-wrap"><table><thead><tr><th>Review</th><th>Reviewer</th><th>Status</th><th>Completion</th><th>Total time</th></tr></thead><tbody>${reviewRows || emptyRow(5, "No reviews yet", "Start the first review for this project.")}</tbody></table></div></section>`, `${edit}<button class="button" data-new-review>+ Start review</button>`, `<a href="#/projects">Projects</a> / ${escapeHTML(project.projectCode)}`);
  document.querySelector("[data-edit-project]")?.addEventListener("click", () => openProjectDrawer(project));
  document.querySelector("[data-new-review]").addEventListener("click", () => showNewReview(id));
}

function showNewReview(projectID) {
  modal.innerHTML = `<form id="review-form" class="modal-content"><h2>Start review</h2><div class="form-grid"><div class="form-field full"><label class="required">Review type</label><select name="reviewType" aria-label="Review type" required>${optionList(state.options.reviewTypes)}</select></div><div class="form-field full"><label class="required">Review date</label><input class="field" name="reviewDate" aria-label="Review date" type="date" value="${new Date().toISOString().slice(0,10)}" required></div><div class="form-error full"></div></div><div class="modal-actions"><button class="button secondary" type="button" data-close-modal>Cancel</button><button class="button" type="submit">Start review</button></div></form>`;
  modal.showModal();
  document.querySelector("#review-form").addEventListener("submit", async event => {
    event.preventDefault();
    const formElement = event.currentTarget;
    try {
      const result = await api(`/api/projects/${projectID}/reviews`, { method: "POST", body: Object.fromEntries(new FormData(formElement)) });
      modal.close();
      location.hash = `#/reviews/${result.id}`;
    } catch (error) { formElement.querySelector(".form-error").textContent = error.message; }
  });
}

async function renderReview(id) {
  const data = await api(`/api/reviews/${id}`);
  state.currentReview = data;
  const grouped = Object.groupBy ? Object.groupBy(data.answers, item => item.reviewArea) : data.answers.reduce((groups, item) => ((groups[item.reviewArea] ||= []).push(item), groups), {});
  let rows = "";
  Object.entries(grouped).forEach(([area, answers]) => {
    rows += `<tr><td colspan="5" class="area-heading">${escapeHTML(area)}</td></tr>`;
    rows += answers.map(item => {
      const questionActions = data.actions.filter(action => action.questionId === item.questionId);
      const actionRows = questionActions.map(action => `<tr class="task-row" data-question-group="${item.questionId}" data-action="${action.id}"><td></td><td><span class="task-indent">Task</span><span class="cell-title">${escapeHTML(action.description)}</span><span class="cell-sub">${escapeHTML(action.actionType)} / ${escapeHTML(action.ownerName)}</span></td><td>${statusBadge(action.status)}</td><td>${formatDate(action.dueDate)}</td><td></td></tr>`).join("");
      return `<tr data-question="${item.questionId}" data-question-group="${item.questionId}"><td><span class="code">${escapeHTML(item.questionCode)}</span></td><td><div class="truncate">${escapeHTML(item.questionText)}</div></td><td>${statusBadge(item.result)}</td><td>${escapeHTML(item.evidenceStatus || "-")}</td><td>${item.actionCount}</td></tr>${actionRows}`;
    }).join("");
  });
  const statusSelect = `<select id="review-status">${optionList(state.options.reviewStatuses, data.review.status, "Select status")}</select>`;
  shell("projects", data.projectName, `${data.review.reviewType} / ${formatDate(data.review.reviewDate)}`, `<section class="panel"><div class="toolbar"><input class="search" id="question-search" type="search" placeholder="Search questions"><span>${statusSelect}</span><span class="badge">${data.review.answered} of ${data.review.total} answered</span></div><div class="table-wrap"><table><thead><tr><th>Code</th><th>Question</th><th>Result</th><th>Evidence</th><th>Actions</th></tr></thead><tbody id="review-questions">${rows || emptyRow(5, "No questions configured", "An administrator must add questions before this review can begin.")}</tbody></table></div></section>`, "", `<a href="#/projects">Projects</a> / <a href="#/projects/${data.projectId}">${escapeHTML(data.projectName)}</a> / Review`);
  document.querySelector("#review-status").addEventListener("change", async event => {
    try { await api(`/api/reviews/${id}/status`, { method: "PUT", body: { status: event.target.value } }); notify("Review status updated"); } catch (error) { showError(error); }
  });
  document.querySelector("#question-search").addEventListener("input", event => {
    const term = event.target.value.toLowerCase();
    document.querySelectorAll("[data-question]").forEach(row => {
      const visible = row.textContent.toLowerCase().includes(term);
      document.querySelectorAll(`[data-question-group="${row.dataset.question}"]`).forEach(groupRow => { groupRow.hidden = !visible; });
    });
  });
  document.querySelectorAll("[data-question]").forEach(row => row.addEventListener("click", () => openAnswerDrawer(data.answers.find(item => item.questionId === Number(row.dataset.question)), id)));
  document.querySelectorAll("[data-action]").forEach(row => row.addEventListener("click", () => openActionDrawer(data.actions.find(item => item.id === Number(row.dataset.action)), { returnReviewId: id })));
}

function openAnswerDrawer(item, reviewID) {
  const questionActions = state.currentReview.actions.filter(action => action.questionId === item.questionId);
  const taskRows = questionActions.map(action => `<tr data-drawer-action="${action.id}"><td><span class="cell-title">${escapeHTML(action.description)}</span><span class="cell-sub">${escapeHTML(action.ownerName)} / ${formatDate(action.dueDate)}</span></td><td>${statusBadge(action.status)}</td></tr>`).join("");
  const body = `<form id="answer-form" class="form-grid">
    <div class="form-field full"><label>Question</label><div><strong>${escapeHTML(item.questionText)}</strong>${item.guidance ? `<p class="hint">${escapeHTML(item.guidance)}</p>` : ""}</div></div>
    <div class="form-field"><label>Result</label><select name="result" aria-label="Result">${optionList(state.options.results, item.result)}</select></div>
    <div class="form-field"><label>Evidence status</label><select name="evidenceStatus" aria-label="Evidence status">${optionList(state.options.evidenceStatuses, item.evidenceStatus)}</select></div>
    <div class="form-field full"><label>Evidence notes / location</label><textarea name="evidenceNotes" aria-label="Evidence notes or location">${escapeHTML(item.evidenceNotes)}</textarea></div>
    <div class="form-field full"><label class="check-row"><input name="pmClarificationUsed" type="checkbox" ${item.pmClarificationUsed ? "checked" : ""}> PM Clarification Used</label></div>
    <div class="form-field full" data-pm-summary ${item.pmClarificationUsed ? "" : "hidden"}><label>PM Clarification Summary</label><textarea name="pmClarificationSummary" aria-label="PM Clarification Summary">${escapeHTML(item.pmClarificationSummary)}</textarea></div>
    <div class="form-field"><label>Primary root cause</label><select name="primaryRootCause" aria-label="Primary root cause">${optionList(state.options.rootCauses, item.primaryRootCause)}</select></div>
    <div class="form-field"><label>Quality-system-related?</label><select name="qualitySystemRelated" aria-label="Quality-system-related">${optionList(state.options.systemRelatedValues, item.qualitySystemRelated)}</select></div>
    <div class="form-field"><label>Review time (minutes)</label><input class="field" name="reviewTimeMinutes" aria-label="Review time in minutes" type="number" min="0" value="${item.reviewTimeMinutes || 0}"></div>
    <div class="form-field full"><label>Reviewer notes</label><textarea name="reviewerNotes" aria-label="Reviewer notes">${escapeHTML(item.reviewerNotes)}</textarea></div>
    <div class="form-field full"><label class="check-row"><input name="followUpRequired" type="checkbox" ${item.followUpRequired ? "checked" : ""}> Follow-Up Required</label></div>
    <div class="form-error full"></div>
  </form><section class="drawer-tasks"><h3>Tasks</h3><div class="table-wrap"><table><thead><tr><th>Task</th><th>Status</th></tr></thead><tbody>${taskRows || emptyRow(2, "No tasks", "Add a task for this question when follow-up work is needed.")}</tbody></table></div></section>`;
  openDrawer(`${item.questionCode} / Answer`, item.reviewArea, body, `<button class="button secondary" data-add-action>+ Add action</button><button class="button" type="submit" form="answer-form">Save answer</button>`);
  const formElement = document.querySelector("#answer-form");
  const answerPayload = () => {
    const form = new FormData(formElement);
    const payload = Object.fromEntries(form);
    payload.reviewTimeMinutes = Number(payload.reviewTimeMinutes || 0);
    payload.pmClarificationUsed = form.has("pmClarificationUsed");
    payload.followUpRequired = form.has("followUpRequired");
    return payload;
  };
  const saveAnswer = () => api(`/api/reviews/${reviewID}/answers/${item.questionId}`, { method: "PUT", body: answerPayload() });
  formElement.elements.pmClarificationUsed.addEventListener("change", event => {
    document.querySelector("[data-pm-summary]").hidden = !event.target.checked;
  });
  document.querySelector("#answer-form").addEventListener("submit", async event => {
    event.preventDefault();
    try { await saveAnswer(); closeDrawer(); notify("Answer saved"); renderReview(reviewID); } catch (error) { formElement.querySelector(".form-error").textContent = error.message; }
  });
  document.querySelector("[data-add-action]").addEventListener("click", async () => {
    try {
      await saveAnswer();
      openActionDrawer(null, { reviewId: reviewID, questionId: item.questionId, questionCode: item.questionCode, questionText: item.questionText });
    } catch (error) {
      formElement.querySelector(".form-error").textContent = error.message;
    }
  });
  document.querySelectorAll("[data-drawer-action]").forEach(row => row.addEventListener("click", () => openActionDrawer(questionActions.find(action => action.id === Number(row.dataset.drawerAction)), { returnReviewId: reviewID })));
}

function instructionTable(rows) {
  if (!rows.length) return "";
  const [headers, ...body] = rows;
  const columnCount = Math.max(...rows.map(row => row.length));
  return `<div class="table-wrap"><table class="instruction-table"><thead><tr>${Array.from({ length: columnCount }, (_, index) => `<th>${escapeHTML(headers[index] || "")}</th>`).join("")}</tr></thead><tbody>${body.map(row => `<tr>${Array.from({ length: columnCount }, (_, index) => `<td>${escapeHTML(row[index] || "")}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`;
}

function instructionContent(rows) {
  const leadingRows = [];
  const groups = [];
  let currentGroup = null;
  rows.forEach(row => {
    if (row.length === 1) {
      currentGroup = { title: row[0], rows: [] };
      groups.push(currentGroup);
    } else if (currentGroup) {
      currentGroup.rows.push(row);
    } else {
      leadingRows.push(row);
    }
  });
  return `${instructionTable(leadingRows)}${groups.map(group => `<details class="wiki-subsection"><summary>${escapeHTML(group.title)}</summary>${instructionTable(group.rows)}</details>`).join("")}`;
}

async function renderInstructions() {
  const data = await api("/api/instructions");
  const sections = [
    ["Overview", "Workbook purpose and navigation", data.overview],
    ["Field instructions", "How to complete each review and question field", data.instructions],
    ["Scoring guide", "Question-specific criteria, evidence, and evaluation boundaries", data.scoringGuide],
    ["Source matrix", "Controlling reference material for each question", data.sourceMatrix],
  ];
  const content = sections.map(([title, description, rows], index) => `<details class="wiki-section" ${index === 0 ? "open" : ""}><summary><span>${escapeHTML(title)}</span><small>${escapeHTML(description)}</small></summary><div class="wiki-body">${instructionContent(rows)}</div></details>`).join("");
  shell("instructions", "Instructions", "Workbook guidance, scoring criteria, and source references", `<section class="wiki">${content}</section>`);
}

async function renderQuestions() {
  shell("questions", "All questions", "Question results across every project and review", `<section class="panel"><div class="toolbar"><input class="search" id="result-search" type="search" placeholder="Search projects or questions"><select id="result-area">${optionList(state.options.reviewAreas, "", "All review areas")}</select><select id="result-value">${optionList(state.options.results, "", "All results")}</select><select id="result-status">${optionList(state.options.reviewStatuses, "", "All review statuses")}</select></div><div class="table-wrap"><table><thead><tr><th>Project</th><th>Review</th><th>Question</th><th>Area</th><th>Result</th><th>Evidence</th><th>Actions</th></tr></thead><tbody id="result-rows"></tbody></table></div></section>`);
  const refresh = async () => {
    const params = new URLSearchParams({ search: document.querySelector("#result-search").value, area: document.querySelector("#result-area").value, result: document.querySelector("#result-value").value, status: document.querySelector("#result-status").value });
    try {
      const results = await api(`/api/results?${params}`);
      document.querySelector("#result-rows").innerHTML = results.map(item => `<tr data-link="#/reviews/${item.reviewId}"><td><span class="code">${escapeHTML(item.projectCode)}</span><span class="cell-sub">${escapeHTML(item.projectName)}</span></td><td><span class="cell-title">${escapeHTML(item.reviewType)}</span><span class="cell-sub">${formatDate(item.reviewDate)} / ${escapeHTML(item.reviewStatus)}</span></td><td><span class="code">${escapeHTML(item.questionCode)}</span><span class="cell-sub truncate">${escapeHTML(item.questionText)}</span></td><td>${escapeHTML(item.reviewArea)}</td><td>${statusBadge(item.result)}</td><td>${escapeHTML(item.evidenceStatus || "-")}</td><td>${item.actionCount}</td></tr>`).join("") || emptyRow(7, "No question results", "Adjust the filters or start a project review.");
    } catch (error) { showError(error); }
  };
  ["result-area", "result-value", "result-status"].forEach(id => document.querySelector(`#${id}`).addEventListener("change", refresh));
  document.querySelector("#result-search").addEventListener("input", debounce(refresh));
  await refresh();
}

async function renderActions(openID = null) {
  shell("actions", "Action items", "Corrective and improvement actions across all reviews", `<section class="panel"><div class="toolbar"><input class="search" id="action-search" type="search" placeholder="Search actions, projects, owners"><select id="action-status">${optionList(state.options.actionStatuses, "", "All statuses")}</select><select id="action-owner"><option value="">All owners</option>${state.users.map(user => `<option value="${user.id}">${escapeHTML(user.name)}</option>`).join("")}</select><select id="action-type">${optionList(state.options.actionTypes, "", "All action types")}</select></div><div class="table-wrap"><table><thead><tr><th>Action</th><th>Project / question</th><th>Type</th><th>Owner</th><th>Due date</th><th>Status</th></tr></thead><tbody id="action-rows"></tbody></table></div></section>`);
  let currentActions = [];
  const refresh = async () => {
    const params = new URLSearchParams({ search: document.querySelector("#action-search").value, status: document.querySelector("#action-status").value, owner: document.querySelector("#action-owner").value, type: document.querySelector("#action-type").value });
    try {
      currentActions = await api(`/api/actions?${params}`);
      document.querySelector("#action-rows").innerHTML = currentActions.map(item => `<tr data-action="${item.id}"><td><span class="cell-title">${escapeHTML(item.description)}</span><span class="cell-sub">${escapeHTML(item.notes)}</span></td><td><span class="code">${escapeHTML(item.projectCode)} / ${escapeHTML(item.questionCode)}</span><span class="cell-sub truncate">${escapeHTML(item.questionText)}</span></td><td>${escapeHTML(item.actionType)}</td><td>${escapeHTML(item.ownerName)}</td><td>${formatDate(item.dueDate)}</td><td>${statusBadge(item.status)}</td></tr>`).join("") || emptyRow(6, "No action items", "Adjust the filters or add an action from a review question.");
      document.querySelectorAll("[data-action]").forEach(row => row.addEventListener("click", () => openActionDrawer(currentActions.find(item => item.id === Number(row.dataset.action)))));
      if (openID) { const item = currentActions.find(action => action.id === Number(openID)); if (item) openActionDrawer(item); openID = null; }
    } catch (error) { showError(error); }
  };
  ["action-status", "action-owner", "action-type"].forEach(id => document.querySelector(`#${id}`).addEventListener("change", refresh));
  document.querySelector("#action-search").addEventListener("input", debounce(refresh));
  await refresh();
}

function openActionDrawer(item = null, context = {}) {
  const action = item || { ...context, status: "Open" };
  const body = `<form id="action-form" class="form-grid">
    <div class="form-field full"><label>Question</label><div><strong>${escapeHTML(action.questionCode || "")}</strong><p class="hint">${escapeHTML(action.questionText || "")}</p></div></div>
    <div class="form-field"><label class="required">Action type</label><select name="actionType" aria-label="Action type" required>${optionList(state.options.actionTypes, action.actionType)}</select></div>
    <div class="form-field"><label class="required">Owner</label><select name="ownerId" aria-label="Owner" required>${userOptions(action.ownerId)}</select></div>
    <div class="form-field full"><label class="required">Action / improvement</label><textarea name="description" aria-label="Action or improvement" required>${escapeHTML(action.description)}</textarea></div>
    <div class="form-field"><label>Due date</label><input class="field" name="dueDate" aria-label="Due date" type="date" value="${dateValue(action.dueDate)}"></div>
    ${item ? `<div class="form-field"><label>Status</label><select name="status" aria-label="Action status">${optionList(state.options.actionStatuses, action.status)}</select></div><div class="form-field full"><label>Closure evidence</label><textarea name="closureEvidence" aria-label="Closure evidence">${escapeHTML(action.closureEvidence)}</textarea></div>` : ""}
    <div class="form-field full"><label>Notes</label><textarea name="notes" aria-label="Action notes">${escapeHTML(action.notes)}</textarea></div>
    <div class="form-error full"></div>
  </form>`;
  openDrawer(item ? "Edit action item" : "New action item", item ? `${item.projectCode} / ${item.questionCode}` : "Assign corrective or improvement work", body, `<button class="button secondary" data-close-drawer>Cancel</button><button class="button" type="submit" form="action-form">${item ? "Save changes" : "Create action"}</button>`);
  document.querySelector("#action-form").addEventListener("submit", async event => {
    event.preventDefault();
    const formElement = event.currentTarget;
    const payload = Object.fromEntries(new FormData(formElement));
    payload.ownerId = Number(payload.ownerId);
    payload.reviewId = Number(action.reviewId);
    payload.questionId = Number(action.questionId);
    try {
      await api(item ? `/api/actions/${item.id}` : "/api/actions", { method: item ? "PUT" : "POST", body: payload });
      closeDrawer();
      notify(item ? "Action updated" : "Action created");
      if (context.returnReviewId) renderReview(context.returnReviewId);
      else if (item) renderActions();
      else renderReview(action.reviewId);
    } catch (error) { formElement.querySelector(".form-error").textContent = error.message; }
  });
}

async function renderAdmin() {
  const users = await api("/api/admin/users");
  const userRows = users.map(item => `<tr><td><span class="cell-title">${escapeHTML(item.name)}</span><span class="cell-sub">${escapeHTML(item.email)}</span></td><td>${escapeHTML(item.role)}</td><td>${statusBadge(item.active ? "Active" : "Inactive")}</td><td><button class="button secondary small" data-reset-user="${item.id}" data-user-name="${escapeHTML(item.name)}">Reset password</button></td></tr>`).join("");
  shell("admin", "Administration", "Manage application users", `
    <section class="panel"><div class="panel-head"><h2>Users</h2><button class="button small" data-new-user>+ Add user</button></div><div class="table-wrap"><table><thead><tr><th>User</th><th>Role</th><th>Status</th><th></th></tr></thead><tbody>${userRows}</tbody></table></div></section>`);
  document.querySelector("[data-new-user]").addEventListener("click", showNewUser);
  document.querySelectorAll("[data-reset-user]").forEach(button => button.addEventListener("click", () => resetUser(button.dataset.resetUser, button.dataset.userName)));
}

function showNewUser() {
  modal.innerHTML = `<form id="user-form" class="modal-content"><h2>Add user</h2><div class="form-grid"><div class="form-field full"><label class="required">Name</label><input class="field" name="name" aria-label="Name" required></div><div class="form-field full"><label class="required">Email</label><input class="field" name="email" aria-label="Email" type="email" required></div><div class="form-field full"><label class="required">Role</label><select name="role" aria-label="Role"><option value="reviewer">Reviewer</option><option value="admin">Administrator</option></select></div><div class="form-error full"></div></div><div class="modal-actions"><button class="button secondary" type="button" data-close-modal>Cancel</button><button class="button" type="submit">Create user</button></div></form>`;
  modal.showModal();
  document.querySelector("#user-form").addEventListener("submit", async event => {
    event.preventDefault();
    const formElement = event.currentTarget;
    try { const result = await api("/api/admin/users", { method: "POST", body: Object.fromEntries(new FormData(formElement)) }); showTemporaryPassword(result.temporaryPassword, "User created"); } catch (error) { formElement.querySelector(".form-error").textContent = error.message; }
  });
}

async function resetUser(id, name) {
  try { const result = await api(`/api/admin/users/${id}/reset-password`, { method: "POST" }); showTemporaryPassword(result.temporaryPassword, `${name} password reset`); } catch (error) { showError(error); }
}
function showTemporaryPassword(password, heading) {
  modal.innerHTML = `<div class="modal-content"><h2>${escapeHTML(heading)}</h2><p class="hint">This password is shown only once. The user must change it at the next sign-in.</p><div class="password-value">${escapeHTML(password)}</div><div class="modal-actions"><button class="button" type="button" data-finish-temp>Done</button></div></div>`;
  document.querySelector("[data-finish-temp]").addEventListener("click", () => { modal.close(); renderAdmin(); });
  modal.showModal();
}

function openDrawer(title, subtitle, body, footer) {
  drawerRoot.innerHTML = `<div class="drawer-backdrop" data-close-drawer></div><aside class="drawer" role="dialog" aria-modal="true" aria-label="${escapeHTML(title)}"><header class="drawer-head"><div><h2>${escapeHTML(title)}</h2><p>${escapeHTML(subtitle)}</p></div><button class="icon-button" data-close-drawer aria-label="Close">&times;</button></header><div class="drawer-body">${body}</div><footer class="drawer-foot">${footer}</footer></aside>`;
  document.body.style.overflow = "hidden";
}
function closeDrawer() { drawerRoot.innerHTML = ""; document.body.style.overflow = ""; }
function debounce(callback, wait = 250) { let timeout; return (...args) => { clearTimeout(timeout); timeout = setTimeout(() => callback(...args), wait); }; }

async function route() {
  if (!state.user) return;
  closeDrawer();
  const parts = location.hash.replace(/^#\/?/, "").split("/").filter(Boolean);
  const section = parts[0] || "dashboard";
  loading();
  try {
    if (section === "dashboard") await renderDashboard();
    else if (section === "projects" && parts[1]) await renderProject(parts[1]);
    else if (section === "projects") await renderProjects();
    else if (section === "reviews" && parts[1]) await renderReview(parts[1]);
    else if (section === "questions") await renderQuestions();
    else if (section === "actions") await renderActions(parts[1]);
    else if (section === "instructions") await renderInstructions();
    else if (section === "admin" && state.user.role === "admin") await renderAdmin();
    else location.hash = "#/dashboard";
  } catch (error) {
    if (error.status === 401) { state.user = null; renderLogin("Your session expired. Please sign in again."); }
    else { showError(error); appRoot.innerHTML = `<div class="loading">${escapeHTML(error.message)}</div>`; }
  }
}

document.addEventListener("click", async event => {
  const linkRow = event.target.closest("[data-link]");
  if (linkRow) location.hash = linkRow.dataset.link;
  if (event.target.closest("[data-close-drawer]")) closeDrawer();
  if (event.target.closest("[data-close-modal]")) modal.close();
  if (event.target.closest("[data-change-password]")) showPasswordChange(false);
  if (event.target.closest("[data-menu]")) document.querySelector("#sidebar")?.classList.toggle("open");
  if (event.target.closest("[data-logout]")) {
    try { await api("/api/auth/logout", { method: "POST" }); } catch { /* Clear local state regardless. */ }
    state.user = null;
    location.hash = "";
    renderLogin();
  }
});
window.addEventListener("hashchange", route);

(async function initialize() {
  try {
    state.user = await api("/api/auth/me");
    if (state.user.mustChangePassword) showPasswordChange(true);
    else { await loadReferenceData(); if (!location.hash) location.hash = "#/dashboard"; await route(); }
  } catch { renderLogin(); }
})();