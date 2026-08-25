package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"fleetmonitoring/backend/internal/telemetry/application"
)

func mapServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, application.ErrValidation) {
		writeValidation(w)
		return
	}
	if errors.Is(err, application.ErrRateLimited) {
		w.Header().Set("Retry-After", retryAfter)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}
	if errors.Is(err, application.ErrBackpressure) {
		w.Header().Set("Retry-After", retryAfter)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backpressure"})
		return
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "validation") {
		writeValidation(w)
		return
	}
	if strings.Contains(msg, "rate") {
		w.Header().Set("Retry-After", retryAfter)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}
	w.Header().Set("Retry-After", retryAfter)
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backpressure"})
}

func writeValidation(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func isMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return true
	}
	return strings.Contains(err.Error(), "request body too large")
}
