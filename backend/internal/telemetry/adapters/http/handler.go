package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
)

type Publisher = application.Publisher
type RateLimiter = application.RateLimiter
type Breaker = application.Breaker
type JetStreamInfo = application.JetStreamInfo

const (
	maxBodyBytes = 1 << 20
	retryAfter   = "5"
)

type handler struct {
	svc      *application.IngestService
	breaker  Breaker
	js       JetStreamInfo
	inflight atomic.Int64
	totalAll atomic.Int64
	mu       sync.RWMutex
	perPlate map[string]*atomic.Int64
}

func NewHandler(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo) http.Handler {
	svc := application.NewIngestService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
	return NewHandlerWithService(svc, breaker, js)
}

func NewHandlerWithService(svc *application.IngestService, breaker Breaker, js JetStreamInfo) http.Handler {
	return &handler{svc: svc, breaker: breaker, js: js, perPlate: make(map[string]*atomic.Int64)}
}

func (h *handler) recordIngest(plate string, n int) {
	h.totalAll.Add(int64(n))
	h.mu.RLock()
	ctr, ok := h.perPlate[plate]
	h.mu.RUnlock()
	if ok {
		ctr.Add(int64(n))
		return
	}
	h.mu.Lock()
	ctr, ok = h.perPlate[plate]
	if !ok {
		ctr = &atomic.Int64{}
		h.perPlate[plate] = ctr
	}
	h.mu.Unlock()
	ctr.Add(int64(n))
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\"", `\"`)
	return s
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
	body, err := h.readBody(r, w)
	if err != nil {
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
		} else {
			writeValidation(w)
		}
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
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
		} else {
			writeValidation(w)
		}
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

func decodeSingleEvent(m map[string]json.RawMessage) (application.RawEvent, error) {
	plate, speedInt, err := decodeRequiredFields(m)
	if err != nil {
		return application.RawEvent{}, err
	}
	latPtr, err := parseOptionalFloat(m, "lat")
	if err != nil {
		return application.RawEvent{}, err
	}
	lonPtr, err := parseOptionalFloat(m, "lon")
	if err != nil {
		return application.RawEvent{}, err
	}
	cid, err := parseOptionalString(m, "client_event_id")
	if err != nil {
		return application.RawEvent{}, err
	}
	occ, err := parseOccurredAt(m)
	if err != nil {
		return application.RawEvent{}, err
	}
	return application.RawEvent{
		Plate:         plate,
		Speed:         &speedInt,
		Lat:           latPtr,
		Lon:           lonPtr,
		ClientEventID: cid,
		OccurredAt:    occ,
	}, nil
}

func decodeRequiredFields(m map[string]json.RawMessage) (string, int, error) {
	plateRaw, err := getRequiredRaw(m, "plate")
	if err != nil {
		return "", 0, err
	}
	var plate string
	if err := json.Unmarshal(plateRaw, &plate); err != nil {
		return "", 0, fmt.Errorf("invalid plate: %w", err)
	}
	speedRaw, err := getRequiredRaw(m, "speed")
	if err != nil {
		return "", 0, err
	}
	speedInt, err := parseSpeedInt(speedRaw)
	if err != nil {
		return "", 0, err
	}
	return plate, speedInt, nil
}

func getRequiredRaw(m map[string]json.RawMessage, key string) (json.RawMessage, error) {
	raw, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("missing %s: %w", key, application.ErrValidation)
	}
	return raw, nil
}

func parseOptionalFloat(m map[string]json.RawMessage, key string) (*float64, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "null" {
		return nil, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	return &f, nil
}

func parseOptionalString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", nil
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "null" || trim == "" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("invalid %s: %w", key, err)
	}
	return s, nil
}

func parseOccurredAt(m map[string]json.RawMessage) (*time.Time, error) {
	raw, ok := m["occurred_at"]
	if !ok {
		return nil, nil
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "null" || trim == "" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("invalid occurred_at: %w", err)
	}
	tm, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		tm2, err2 := time.Parse(time.RFC3339, s)
		if err2 != nil {
			return nil, fmt.Errorf("invalid occurred_at format: %w", err)
		}
		tm = tm2
	}
	return &tm, nil
}

func decodeBatch(body []byte) ([]application.RawEvent, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	eventsRaw, ok := top["events"]
	if !ok {
		return nil, fmt.Errorf("missing events: %w", application.ErrValidation)
	}
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(eventsRaw, &rawMessages); err != nil {
		return nil, fmt.Errorf("invalid events: %w", err)
	}
	if len(rawMessages) == 0 || len(rawMessages) > application.MaxBatchSize {
		return nil, fmt.Errorf("invalid batch size %d: %w", len(rawMessages), application.ErrValidation)
	}
	raws := make([]application.RawEvent, 0, len(rawMessages))
	for i, eb := range rawMessages {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(eb, &m); err != nil {
			return nil, fmt.Errorf("invalid event %d: %w", i, err)
		}
		raw, err := decodeSingleEvent(m)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", i, err)
		}
		raws = append(raws, raw)
	}
	return raws, nil
}

func (h *handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	state := "closed"
	if h.breaker != nil {
		state = h.breaker.State()
		if h.breaker.IsOpen() {
			state = "open"
		}
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
	if h.breaker != nil && h.breaker.IsOpen() {
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
	fmt.Fprintf(w, "# HELP telemetry_ingest_total total telemetry events ingested via 202\n")
	fmt.Fprintf(w, "# TYPE telemetry_ingest_total counter\n")
	h.mu.RLock()
	plates := make([]string, 0, len(h.perPlate))
	snapshot := make(map[string]int64, len(h.perPlate))
	for plate, ctr := range h.perPlate {
		plates = append(plates, plate)
		snapshot[plate] = ctr.Load()
	}
	h.mu.RUnlock()
	sort.Strings(plates)
	for _, plate := range plates {
		fmt.Fprintf(w, "telemetry_ingest_total{plate=\"%s\"} %d\n", escapeLabel(plate), snapshot[plate])
	}
	fmt.Fprintf(w, "# HELP telemetry_ingest_total_sum total telemetry events ingested via 202\n")
	fmt.Fprintf(w, "# TYPE telemetry_ingest_total_sum counter\n")
	fmt.Fprintf(w, "telemetry_ingest_total_sum %d\n", h.totalAll.Load())
}

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
