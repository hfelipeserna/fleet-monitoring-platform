package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"fleetmonitoring/backend/internal/telemetry/application"
	infranats "fleetmonitoring/backend/internal/telemetry/infra/nats"
)

const dlqMaxBodyBytes = 1 << 20

type DLQMsg = infranats.DLQMsg

type DLQJetStream interface {
	FetchDLQ(n int) ([]DLQMsg, error)
	RepublishRaw(subject string, data []byte) error
}

type dlqHandler struct {
	js DLQJetStream
}

func NewDLQHandler(js DLQJetStream) http.Handler {
	return &dlqHandler{js: js}
}

func (h *dlqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/internal/dlq/republish" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, dlqMaxBodyBytes)
	defer func() { _ = r.Body.Close() }()
	limit, err := parseRepublishLimit(r)
	if err != nil {
		if isMaxBytesError(err) {
			http.Error(w, `{"error":"payload_too_large"}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"error":"validation"}`, http.StatusBadRequest)
		return
	}
	msgs, err := h.fetchDLQLimited(limit)
	if err != nil {
		slog.Error("dlq fetch failed", "error", err)
		http.Error(w, `{"error":"dlq_fetch_failed"}`, http.StatusInternalServerError)
		return
	}
	republished, err := h.republishAll(msgs)
	if err != nil {
		slog.Error("dlq republish failed", "error", err)
		http.Error(w, `{"error":"dlq_republish_failed"}`, http.StatusInternalServerError)
		return
	}
	writeRepublished(w, republished)
}

func parseRepublishLimit(r *http.Request) (int, error) {
	limit := parseQueryLimit(r)
	n, has, err := parseBodyLimit(r)
	if err != nil {
		return 0, err
	}
	if has {
		limit = sanitizeDLQLimit(n)
	}
	return limit, nil
}

func parseQueryLimit(r *http.Request) int {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return infranats.DefaultDLQLimit
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return infranats.DefaultDLQLimit
	}
	return sanitizeDLQLimit(n)
}

func parseBodyLimit(r *http.Request) (int, bool, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return 0, false, fmt.Errorf("read body failed: %w", err)
	}
	if len(body) == 0 {
		return 0, false, nil
	}
	return extractLimitFromBody(body)
}

func extractLimitFromBody(body []byte) (int, bool, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, false, fmt.Errorf("validation: %w", errors.Join(application.ErrValidation, err))
	}
	v, ok := m["limit"]
	if !ok {
		return 0, false, nil
	}
	var n int
	if err := json.Unmarshal(v, &n); err != nil {
		return 0, false, fmt.Errorf("validation: %w", errors.Join(application.ErrValidation, err))
	}
	if n < 0 {
		return 0, false, nil
	}
	return n, true, nil
}

func sanitizeDLQLimit(n int) int {
	return infranats.SanitizeDLQLimit(n)
}

func (h *dlqHandler) fetchDLQLimited(limit int) ([]DLQMsg, error) {
	limit = sanitizeDLQLimit(limit)
	msgs, err := h.js.FetchDLQ(limit)
	if err != nil {
		return nil, err
	}
	if len(msgs) > limit {
		msgs = msgs[:limit]
	}
	return msgs, nil
}

func (h *dlqHandler) republishAll(msgs []DLQMsg) (int, error) {
	republished := 0
	var republishErr error
	for _, msg := range msgs {
		data := msg.Data()
		subject := resolveSubject(extractPlate(data))
		if err := h.js.RepublishRaw(subject, data); err != nil {
			republishErr = err
			continue
		}
		if err := msg.Ack(); err != nil {
			republishErr = fmt.Errorf("ack dlq failed: %w", err)
			continue
		}
		republished++
	}
	if republishErr != nil {
		return republished, republishErr
	}
	return republished, nil
}

func resolveSubject(plate string) string {
	return infranats.ResolveSubject(plate)
}

func writeRepublished(w http.ResponseWriter, n int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"republished": n})
}

func extractPlate(data []byte) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if v, ok := m["plate"]; ok {
		var plate string
		if err := json.Unmarshal(v, &plate); err == nil {
			return plate
		}
	}
	return ""
}
