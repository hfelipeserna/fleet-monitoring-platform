package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"fleetmonitoring/backend/internal/telemetry/application"
)

func (h *handler) handleSingle(w http.ResponseWriter, r *http.Request) {
	h.inflight.Add(1)
	defer h.inflight.Add(-1)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	body, err := h.readBody(r, w)
	if err != nil {
		h.handleBodyError(w, err)
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		writeValidation(w)
		return
	}
	raw, err := decodeSingleEvent(m)
	if err != nil {
		writeValidation(w)
		return
	}
	evt, err := h.svc.IngestSingle(r.Context(), raw)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	h.recordIngest(evt.Plate, 1)
	slog.Info("ingest single", "plate", evt.Plate, "client_event_id", evt.ClientEventID)
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (h *handler) handleBatch(w http.ResponseWriter, r *http.Request) {
	h.inflight.Add(1)
	defer h.inflight.Add(-1)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	body, err := h.readBody(r, w)
	if err != nil {
		h.handleBodyError(w, err)
		return
	}
	raws, err := decodeBatch(body)
	if err != nil {
		writeValidation(w)
		return
	}
	evts, err := h.svc.IngestBatch(r.Context(), raws)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	for _, e := range evts {
		h.recordIngest(e.Plate, 1)
	}
	if len(evts) > 0 {
		slog.Info("ingest batch", "plate", evts[0].Plate, "count", len(evts))
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"accepted": len(evts)})
}

func (h *handler) handleBodyError(w http.ResponseWriter, err error) {
	if isMaxBytesError(err) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
		return
	}
	writeValidation(w)
}

func (h *handler) readBody(r *http.Request, w http.ResponseWriter) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body: %w", application.ErrValidation)
	}
	return body, nil
}
