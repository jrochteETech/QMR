package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "qmr_session"

type user struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	Active             bool   `json:"active"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type userContextKey struct{}

func (application *app) ensureBootstrapAdmin(ctx context.Context) error {
	var count int
	if err := application.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil || count > 0 {
		return err
	}
	password := env("ADMIN_PASSWORD", "ChangeMe!123")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = application.db.Exec(ctx,
		"INSERT INTO users (name, email, password_hash, role, must_change_password) VALUES ($1, $2, $3, 'admin', TRUE)",
		env("ADMIN_NAME", "QMR Administrator"), strings.ToLower(env("ADMIN_EMAIL", "admin@example.com")), string(hash))
	return err
}

func (application *app) login(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if readJSON(request, &input) != nil || !required(input.Email, input.Password) {
		writeError(response, http.StatusBadRequest, "email and password are required")
		return
	}
	var current user
	var passwordHash string
	err := application.db.QueryRow(request.Context(),
		"SELECT id, name, email, role, active, must_change_password, password_hash FROM users WHERE email = $1",
		strings.ToLower(strings.TrimSpace(input.Email))).Scan(&current.ID, &current.Name, &current.Email, &current.Role, &current.Active, &current.MustChangePassword, &passwordHash)
	if err != nil || !current.Active || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		writeError(response, http.StatusUnauthorized, "invalid email or password")
		return
	}
	token, tokenHash, err := newToken()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "could not start session")
		return
	}
	expires := time.Now().Add(12 * time.Hour)
	if _, err := application.db.Exec(request.Context(), "INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)", tokenHash, current.ID, expires); err != nil {
		handleDBError(response, err)
		return
	}
	http.SetCookie(response, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: application.secureCookie, SameSite: http.SameSiteStrictMode, Expires: expires})
	writeJSON(response, http.StatusOK, current)
}

func (application *app) logout(response http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(sessionCookie); err == nil {
		_, _ = application.db.Exec(request.Context(), "DELETE FROM sessions WHERE token_hash = $1", hashToken(cookie.Value))
	}
	http.SetCookie(response, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: application.secureCookie, SameSite: http.SameSiteStrictMode})
	response.WriteHeader(http.StatusNoContent)
}

func (application *app) me(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, currentUser(request))
}

func (application *app) changePassword(response http.ResponseWriter, request *http.Request) {
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if readJSON(request, &input) != nil || len(input.NewPassword) < 12 {
		writeError(response, http.StatusBadRequest, "new password must be at least 12 characters")
		return
	}
	current := currentUser(request)
	var passwordHash string
	if err := application.db.QueryRow(request.Context(), "SELECT password_hash FROM users WHERE id = $1", current.ID).Scan(&passwordHash); err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.CurrentPassword)) != nil {
		writeError(response, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	newHash, _ := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	_, err := application.db.Exec(request.Context(), "UPDATE users SET password_hash = $1, must_change_password = FALSE, updated_at = NOW() WHERE id = $2", string(newHash), current.ID)
	if err != nil {
		handleDBError(response, err)
		return
	}
	current.MustChangePassword = false
	writeJSON(response, http.StatusOK, current)
}

func (application *app) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie(sessionCookie)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "authentication required")
			return
		}
		var current user
		err = application.db.QueryRow(request.Context(), `
			SELECT u.id, u.name, u.email, u.role, u.active, u.must_change_password
			FROM sessions s JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1 AND s.expires_at > NOW()`, hashToken(cookie.Value)).
			Scan(&current.ID, &current.Name, &current.Email, &current.Role, &current.Active, &current.MustChangePassword)
		if err != nil || !current.Active {
			writeError(response, http.StatusUnauthorized, "session expired")
			return
		}
		if current.MustChangePassword && request.URL.Path != "/api/auth/change-password" && request.URL.Path != "/api/auth/logout" && request.URL.Path != "/api/auth/me" {
			writeError(response, http.StatusForbidden, "password change required")
			return
		}
		next(response, request.WithContext(context.WithValue(request.Context(), userContextKey{}, current)))
	}
}

func (application *app) admin(next http.HandlerFunc) http.HandlerFunc {
	return application.auth(func(response http.ResponseWriter, request *http.Request) {
		if currentUser(request).Role != "admin" {
			writeError(response, http.StatusForbidden, "administrator access required")
			return
		}
		next(response, request)
	})
}

func currentUser(request *http.Request) user {
	return request.Context().Value(userContextKey{}).(user)
}

func newToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func temporaryPassword() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "Qmr!" + base64.RawURLEncoding.EncodeToString(buffer), nil
}
