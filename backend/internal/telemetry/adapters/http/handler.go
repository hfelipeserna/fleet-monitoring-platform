package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
)

type Publisher = application.Publisher
type RateLimiter = application.RateLimiter
type Breaker = application.Breaker
type JetStreamInfo = application.JetStreamInfo

type handler struct {
	svc     *application.IngestService
	breaker Breaker
	js      JetStreamInfo
	inflight atomic.Int64
}

func NewHandler(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo) http.Handler {
	svc := application.NewIngestService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
	h := &handler{svc: svc, breaker: breaker, js: js}
	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/telemetry":
		h.handleSingle(w, r)
	case "/v1/telemetry/batch":
		h.handleBatch(w, r)
	case "/healthz":
		h.handleHealthz(w, r)
	case "/metrics":
		h.handleMetrics(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) handleSingle(w http.ResponseWriter, r *http.Request) {
	h.inflight.Add(1)
	defer h.inflight.Add(-1)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	if len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	plateRaw, ok := m["plate"]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	speedRaw, ok := m["speed"]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	var plate string
	if err := json.Unmarshal(plateRaw, &plate); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	speedInt, err := parseSpeedInt(speedRaw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	var latPtr *float64
	if v, ok := m["lat"]; ok {
		trim := strings.TrimSpace(string(v))
		if trim == "null" {
			latPtr = nil
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
				return
			}
			latPtr = &f
		}
	}
	var lonPtr *float64
	if v, ok := m["lon"]; ok {
		trim := strings.TrimSpace(string(v))
		if trim == "null" {
			lonPtr = nil
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
				return
			}
			lonPtr = &f
		}
	}
	var cid string
	if v, ok := m["client_event_id"]; ok {
		trim := strings.TrimSpace(string(v))
		if trim != "null" && trim != "" {
			if err := json.Unmarshal(v, &cid); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
				return
			}
		}
	}
	var occ *time.Time
	if v, ok := m["occurred_at"]; ok {
		trim := strings.TrimSpace(string(v))
		if trim != "null" && trim != "" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
				return
			}
			tm, err := time.Parse(time.RFC3339Nano, s)
			if err != nil {
				tm2, err2 := time.Parse(time.RFC3339, s)
				if err2 != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
					return
				}
				tm = tm2
			}
			occ = &tm
		}
	}
	raw := application.RawEvent{
		Plate:         plate,
		Speed:         &speedInt,
		Lat:           latPtr,
		Lon:           lonPtr,
		ClientEventID: cid,
		OccurredAt:    occ,
	}
	evt, err := h.svc.IngestSingle(r.Context(), raw)
	if err != nil {
		mapServiceError(w, err)
		return
	}
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	if len(body) > 1<<20 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	eventsRaw, ok := top["events"]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(eventsRaw, &rawMessages); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	if len(rawMessages) == 0 || len(rawMessages) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	raws := make([]application.RawEvent, 0, len(rawMessages))
	for _, eb := range rawMessages {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(eb, &m); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		plateRaw, ok := m["plate"]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		speedRaw, ok := m["speed"]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		var plate string
		if err := json.Unmarshal(plateRaw, &plate); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		speedInt, err := parseSpeedInt(speedRaw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
			return
		}
		var latPtr *float64
		if v, ok := m["lat"]; ok {
			trim := strings.TrimSpace(string(v))
			if trim == "null" {
				latPtr = nil
			} else {
				var f float64
				if err := json.Unmarshal(v, &f); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
					return
				}
				latPtr = &f
			}
		}
		var lonPtr *float64
		if v, ok := m["lon"]; ok {
			trim := strings.TrimSpace(string(v))
			if trim == "null" {
				lonPtr = nil
			} else {
				var f float64
				if err := json.Unmarshal(v, &f); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
					return
				}
				lonPtr = &f
			}
		}
		var cid string
		if v, ok := m["client_event_id"]; ok {
			trim := strings.TrimSpace(string(v))
			if trim != "null" && trim != "" {
				if err := json.Unmarshal(v, &cid); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
					return
				}
			}
		}
		var occ *time.Time
		if v, ok := m["occurred_at"]; ok {
			trim := strings.TrimSpace(string(v))
			if trim != "null" && trim != "" {
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
					return
				}
				tm, err := time.Parse(time.RFC3339Nano, s)
				if err != nil {
					tm2, err2 := time.Parse(time.RFC3339, s)
					if err2 != nil {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
						return
					}
					tm = tm2
				}
				occ = &tm
			}
		}
		raws = append(raws, application.RawEvent{
			Plate:         plate,
			Speed:         &speedInt,
			Lat:           latPtr,
			Lon:           lonPtr,
			ClientEventID: cid,
			OccurredAt:    occ,
		})
	}
	evts, err := h.svc.IngestBatch(r.Context(), raws)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	if len(evts) > 0 {
		slog.Info("ingest batch", "plate", evts[0].Plate, "count", len(evts))
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"accepted": len(evts)})
}

func (h *handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	state := "closed"
	if h.breaker != nil {
		state = h.breaker.State()
	}
	var used, max uint64
	jet := "connected"
	if h.js != nil {
		u, m := h.js.Bytes()
		used, max = u, m
		jet = fmt.Sprintf("%d/%d", used, max)
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"breaker":   state,
		"jetstream": jet,
	})
}

func (h *handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	inflight := h.inflight.Load()
	var used, max uint64
	if h.js != nil {
		used, max = h.js.Bytes()
	}
	state := 0
	if h.breaker != nil && strings.EqualFold(h.breaker.State(), "open") {
		state = 1
	}
	fmt.Fprintf(w, "# HELP ingest_inflight current ingest requests\n")
	fmt.Fprintf(w, "# TYPE ingest_inflight gauge\n")
	fmt.Fprintf(w, "ingest_inflight %d\n", inflight)
	fmt.Fprintf(w, "# HELP nats_pending pending publish async\n")
	fmt.Fprintf(w, "# TYPE nats_pending gauge\n")
	fmt.Fprintf(w, "nats_pending 0\n")
	fmt.Fprintf(w, "# HELP jetstream_bytes used bytes\n")
	fmt.Fprintf(w, "# TYPE jetstream_bytes gauge\n")
	fmt.Fprintf(w, "jetstream_bytes %d\n", used)
	fmt.Fprintf(w, "# HELP jetstream_max_bytes max bytes\n")
	fmt.Fprintf(w, "# TYPE jetstream_max_bytes gauge\n")
	fmt.Fprintf(w, "jetstream_max_bytes %d\n", max)
	fmt.Fprintf(w, "# HELP breaker_state breaker open state\n")
	fmt.Fprintf(w, "# TYPE breaker_state gauge\n")
	fmt.Fprintf(w, "breaker_state %d\n", state)
}

func mapServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, application.ErrValidation) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	if errors.Is(err, application.ErrRateLimited) {
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}
	if errors.Is(err, application.ErrBackpressure) {
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backpressure"})
		return
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "validation") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation"})
		return
	}
	if strings.Contains(msg, "rate") {
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}
	w.Header().Set("Retry-After", "5")
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backpressure"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func parseSpeedInt(raw json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, fmt.Errorf("missing speed")
	}
	if strings.ContainsAny(s, ".eE") {
		return 0, fmt.Errorf("speed must be integer")
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("speed must be integer: %w", err)
	}
	return v, nil
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
