package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type apiError struct {
	Error string `json:"error"`
}

func readJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, apiError{Error: message})
}

func handleDBError(response http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(response, http.StatusNotFound, "record not found")
		return
	}
	writeError(response, http.StatusInternalServerError, "database operation failed")
}

func pathID(request *http.Request, name string) (int64, error) {
	return strconv.ParseInt(request.PathValue(name), 10, 64)
}

func required(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}
