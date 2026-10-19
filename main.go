package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

//go:embed Web/*
var assets embed.FS

type app struct {
	db           *pgxpool.Pool
	secureCookie bool
	instructions instructionData
	questions    []controlledQuestion
}

func main() {
	ctx := context.Background()
	databaseURL := env("DATABASE_URL", "postgres://qmr:qmr@localhost:5432/qmr?sslmode=disable")
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if _, err := db.Exec(ctx, schemaSQL); err != nil {
		log.Fatalf("apply schema: %v", err)
	}

	instructions, err := loadInstructions(assets)
	if err != nil {
		log.Fatalf("load instructions: %v", err)
	}
	questions, err := loadQuestions(assets)
	if err != nil {
		log.Fatalf("load questions: %v", err)
	}
	application := &app{db: db, secureCookie: strings.EqualFold(env("COOKIE_SECURE", "false"), "true"), instructions: instructions, questions: questions}
	if err := application.syncQuestionDefinitions(ctx); err != nil {
		log.Fatalf("synchronize questions: %v", err)
	}
	if err := application.ensureBootstrapAdmin(ctx); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	mux := http.NewServeMux()
	application.routes(mux)

	webAssets, err := fs.Sub(assets, "Web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(webAssets))))
	mux.HandleFunc("GET /brand-logo", application.serveLogo)
	mux.HandleFunc("GET /", func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") {
			http.NotFound(response, request)
			return
		}
		content, readErr := fs.ReadFile(webAssets, "index.html")
		if readErr != nil {
			http.Error(response, "application unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write(content)
	})

	server := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("QMR listening on %s", server.Addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (application *app) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", application.health)
	mux.HandleFunc("POST /api/auth/login", application.login)
	mux.HandleFunc("POST /api/auth/logout", application.auth(application.logout))
	mux.HandleFunc("GET /api/auth/me", application.auth(application.me))
	mux.HandleFunc("POST /api/auth/change-password", application.auth(application.changePassword))

	mux.HandleFunc("GET /api/options", application.auth(application.options))
	mux.HandleFunc("GET /api/instructions", application.auth(application.getInstructions))
	mux.HandleFunc("GET /api/users", application.auth(application.listActiveUsers))
	mux.HandleFunc("GET /api/dashboard", application.auth(application.dashboard))
	mux.HandleFunc("GET /api/projects", application.auth(application.listProjects))
	mux.HandleFunc("POST /api/projects", application.admin(application.createProject))
	mux.HandleFunc("GET /api/projects/{id}", application.auth(application.getProject))
	mux.HandleFunc("PUT /api/projects/{id}", application.admin(application.updateProject))
	mux.HandleFunc("POST /api/projects/{id}/reviews", application.auth(application.createReview))
	mux.HandleFunc("GET /api/reviews/{id}", application.auth(application.getReview))
	mux.HandleFunc("PUT /api/reviews/{reviewID}/answers/{questionID}", application.auth(application.saveAnswer))
	mux.HandleFunc("PUT /api/reviews/{id}/status", application.auth(application.updateReviewStatus))

	mux.HandleFunc("GET /api/results", application.auth(application.listResults))
	mux.HandleFunc("GET /api/actions", application.auth(application.listActions))
	mux.HandleFunc("POST /api/actions", application.auth(application.createAction))
	mux.HandleFunc("PUT /api/actions/{id}", application.auth(application.updateAction))

	mux.HandleFunc("GET /api/admin/users", application.admin(application.listUsers))
	mux.HandleFunc("POST /api/admin/users", application.admin(application.createUser))
	mux.HandleFunc("POST /api/admin/users/{id}/reset-password", application.admin(application.resetPassword))
}

func (application *app) health(response http.ResponseWriter, request *http.Request) {
	if err := application.db.Ping(request.Context()); err != nil {
		writeError(response, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (application *app) serveLogo(response http.ResponseWriter, request *http.Request) {
	content, err := assets.ReadFile("Web/E Tech Group - Mark - Full Color - Background.png")
	if err != nil {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Type", "image/png")
	response.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = response.Write(content)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Referrer-Policy", "same-origin")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'")
		next.ServeHTTP(response, request)
	})
}

func parseDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q", value)
	}
	return &parsed, nil
}
