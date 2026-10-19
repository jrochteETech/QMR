# Quality Management Review

A project quality review system with a Go API, vanilla HTML/CSS/JavaScript frontend, and PostgreSQL persistence.

## Features

- Project register with repeated Initial and Follow-Up review instances
- Workbook-derived result, evidence, root-cause, system-impact, and action fields
- Question answer drawer with linked corrective or improvement actions
- Searchable cross-project question and action views
- Admin-managed question catalog, projects, and users
- One-time temporary passwords with mandatory password change after creation or reset
- Built-in expandable instructions, scoring guidance, and source matrix
- Project labor financials with calculated BAC margin and margin variance
- Question-level PM clarification, follow-up flags, inline tasks, and review time totals
- Dashboard metrics for conformance, evidence gaps, reviews, and overdue actions

## Run with Docker Compose

1. Copy `.env.example` to `.env` and replace every placeholder secret.
2. For an HTTPS deployment, set `COOKIE_SECURE=true`.
3. Start the stack:

   ```sh
   docker compose up --build -d
   ```

4. Open `http://localhost:8080` or the configured `QMR_PORT`.

The bootstrap admin is created only when the user table is empty. Its configured password must be changed on first sign-in.

## Komodo

Deploy this repository as a Compose stack using `compose.yaml`. Store all `.env` values as Komodo-managed secrets or environment variables. Preserve the `qmr-data` named volume across redeployments. Put the app behind an HTTPS reverse proxy, set `COOKIE_SECURE=true`, and expose only the app service; PostgreSQL has no host port.

## Local Go development

Run PostgreSQL, set `DATABASE_URL`, `ADMIN_EMAIL`, and `ADMIN_PASSWORD`, then run:

```sh
go run .
```

The schema is applied idempotently at startup. Instructions and controlled lists are packaged with the application, and question definitions are managed in the Administration view.