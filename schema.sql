CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'reviewer')),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id BIGSERIAL PRIMARY KEY,
    project_code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    business_unit TEXT NOT NULL CHECK (business_unit IN ('Central', 'East', 'GL', 'MCT', 'West')),
    status TEXT NOT NULL CHECK (status IN ('Active', 'Completed')),
    customer TEXT NOT NULL DEFAULT '',
    project_manager TEXT NOT NULL DEFAULT '',
    start_date DATE,
    end_date DATE,
    notes TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE projects ADD COLUMN IF NOT EXISTS bac_labor_revenue NUMERIC(14,2) NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS bac_labor_cost NUMERIC(14,2) NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS act_lab_margin NUMERIC(8,2) NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS question_definitions (
    id BIGSERIAL PRIMARY KEY,
    question_code TEXT NOT NULL UNIQUE,
    review_area TEXT NOT NULL,
    question_text TEXT NOT NULL,
    guidance TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reviews (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    review_type TEXT NOT NULL CHECK (review_type IN ('Initial Review', 'Follow-Up Review')),
    status TEXT NOT NULL DEFAULT 'Not Started' CHECK (status IN ('Not Started', 'In Progress', 'Completed')),
    review_date DATE NOT NULL DEFAULT CURRENT_DATE,
    reviewer_id BIGINT NOT NULL REFERENCES users(id),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS review_questions (
    review_id BIGINT NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES question_definitions(id),
    question_code TEXT NOT NULL,
    review_area TEXT NOT NULL,
    question_text TEXT NOT NULL,
    guidance TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (review_id, question_id)
);

INSERT INTO review_questions (review_id, question_id, question_code, review_area, question_text, guidance, sort_order)
SELECT r.id, q.id, q.question_code, q.review_area, q.question_text, q.guidance, q.sort_order
FROM reviews r CROSS JOIN question_definitions q
WHERE q.active
ON CONFLICT (review_id, question_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS review_answers (
    id BIGSERIAL PRIMARY KEY,
    review_id BIGINT NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES question_definitions(id),
    result TEXT NOT NULL DEFAULT '',
    evidence_status TEXT NOT NULL DEFAULT '',
    evidence_notes TEXT NOT NULL DEFAULT '',
    primary_root_cause TEXT NOT NULL DEFAULT '',
    quality_system_related TEXT NOT NULL DEFAULT '',
    reviewer_notes TEXT NOT NULL DEFAULT '',
    review_time_minutes INTEGER NOT NULL DEFAULT 0 CHECK (review_time_minutes >= 0),
    updated_by BIGINT NOT NULL REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (review_id, question_id)
);

ALTER TABLE review_answers ADD COLUMN IF NOT EXISTS pm_clarification_used BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE review_answers ADD COLUMN IF NOT EXISTS pm_clarification_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE review_answers ADD COLUMN IF NOT EXISTS follow_up_required BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS actions (
    id BIGSERIAL PRIMARY KEY,
    review_id BIGINT NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES question_definitions(id),
    action_type TEXT NOT NULL,
    description TEXT NOT NULL,
    owner_id BIGINT NOT NULL REFERENCES users(id),
    due_date DATE,
    status TEXT NOT NULL DEFAULT 'Open' CHECK (status IN ('Open', 'In Progress', 'Closed', 'Deferred')),
    closure_evidence TEXT NOT NULL DEFAULT '',
    closed_date DATE,
    notes TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reviews_project ON reviews(project_id);
CREATE INDEX IF NOT EXISTS idx_review_questions_question ON review_questions(question_id);
CREATE INDEX IF NOT EXISTS idx_answers_review ON review_answers(review_id);
CREATE INDEX IF NOT EXISTS idx_actions_review ON actions(review_id);
CREATE INDEX IF NOT EXISTS idx_actions_owner ON actions(owner_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);