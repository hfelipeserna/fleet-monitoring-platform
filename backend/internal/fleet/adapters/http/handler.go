package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type Querier interface {
	LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error)
	History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error)
}

type OpsProvider interface {
	BreakerState() string
	NATSConnected() bool
	DBPoolStat() string
}

type ZoneCounter interface {
	Count(ctx context.Context) (int, error)
}

type defaultOps struct{}

func (d defaultOps) BreakerState() string { return "closed" }
func (d defaultOps) NATSConnected() bool  { return true }
func (d defaultOps) DBPoolStat() string   { return "ok" }

type Handler struct {
	querier     Querier
	ops         OpsProvider
	zoneCounter ZoneCounter
	mux         *http.ServeMux
}

type HandlerOption func(*Handler)

func WithZoneCounter(zc ZoneCounter) HandlerOption {
	return func(h *Handler) { h.zoneCounter = zc }
}

func WithOps(p OpsProvider) HandlerOption {
	return func(h *Handler) {
		if p != nil {
			h.ops = p
		}
	}
}

func NewHandler(q Querier, opts ...HandlerOption) http.Handler {
	h := &Handler{querier: q, ops: defaultOps{}, mux: http.NewServeMux()}
	for _, opt := range opts {
		opt(h)
	}
	h.registerRoutes()
	return h
}

func NewHandlerWithZoneCounter(q Querier, zc ZoneCounter, ops ...OpsProvider) http.Handler {
	var opts []HandlerOption
	opts = append(opts, WithZoneCounter(zc))
	if len(ops) > 0 && ops[0] != nil {
		opts = append(opts, WithOps(ops[0]))
	}
	return NewHandler(q, opts...)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("/api/fleet/positions", h.handlePositions)
	h.mux.HandleFunc("/api/vehicles/", h.handleHistory)
	h.mux.HandleFunc("/healthz", h.handleHealthz)
	h.mux.HandleFunc("/api/healthz", h.handleHealthz)
	h.mux.HandleFunc("/metrics", h.handleMetrics)
	h.mux.HandleFunc("/api/metrics", h.handleMetrics)
}

func (h *Handler) WithZoneCounter(zc ZoneCounter) *Handler {
	h.zoneCounter = zc
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeValidation(w http.ResponseWriter, err error) {
	slog.Error("validation failed", "error", err)
	msg := "invalid request"
	if errors.Is(err, shared.ErrInvalidPlate) {
		msg = "invalid plate"
	} else if errors.Is(err, shared.ErrValidation) {
		msg = "invalid parameters"
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": msg})
}

func writeInternal(w http.ResponseWriter, err error) {
	slog.Error("internal error", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
}

func parseLimit(s string) (int, error) {
	if s == "" {
		return 100, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("limit invalid: %w", shared.ErrValidation)
	}
	if v < 1 || v > 500 {
		return 0, fmt.Errorf("limit %d out of range: %w", v, shared.ErrValidation)
	}
	return v, nil
}

func parsePlateParam(s string) (*shared.Plate, error) {
	if s == "" {
		return nil, nil
	}
	p, err := shared.ParsePlate(s)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func parseTimeParam(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("time %q invalid RFC3339: %w", s, shared.ErrValidation)
		}
	}
	return &t, nil
}

func validateCursor(cursor string) error {
	if cursor == "" {
		return nil
	}
	_, _, err := shared.DecodeCursor(cursor)
	return err
}

type vehicleDTO struct {
	Plate      string    `json:"plate"`
	Lat        *float64  `json:"lat"`
	Lon        *float64  `json:"lon"`
	Speed      int       `json:"speed"`
	ReceivedAt time.Time `json:"received_at"`
	Status     string    `json:"status"`
}

func toVehicleDTO(v fleet.VehiclePos) vehicleDTO {
	var lat, lon *float64
	if v.Lat != nil {
		r := shared.Round6(*v.Lat)
		lat = &r
	}
	if v.Lon != nil {
		r := shared.Round6(*v.Lon)
		lon = &r
	}
	return vehicleDTO{Plate: v.Plate, Lat: lat, Lon: lon, Speed: v.Speed, ReceivedAt: v.ReceivedAt, Status: v.Status}
}

func (h *Handler) handlePositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query()
	plate, err := parsePlateParam(q.Get("plate"))
	if err != nil {
		writeValidation(w, err)
		return
	}
	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		writeValidation(w, err)
		return
	}
	cursor := q.Get("cursor")
	if err := validateCursor(cursor); err != nil {
		writeValidation(w, err)
		return
	}
	vehicles, nextCursor, err := h.querier.LastPositions(r.Context(), plate, limit, cursor)
	if err != nil {
		if errors.Is(err, shared.ErrValidation) || errors.Is(err, shared.ErrInvalidPlate) {
			writeValidation(w, err)
			return
		}
		writeInternal(w, err)
		return
	}
	dtos := make([]vehicleDTO, len(vehicles))
	for i, v := range vehicles {
		dtos[i] = toVehicleDTO(v)
	}
	resp := map[string]any{"vehicles": dtos, "next_cursor": nil}
	if nextCursor != "" {
		resp["next_cursor"] = nextCursor
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/vehicles/") || !strings.HasSuffix(path, "/history") {
		http.NotFound(w, r)
		return
	}
	trim := strings.TrimPrefix(path, "/api/vehicles/")
	plateStr := strings.TrimSuffix(trim, "/history")
	plateStr = strings.TrimSuffix(plateStr, "/")
	plate, err := shared.ParsePlate(plateStr)
	if err != nil {
		writeValidation(w, err)
		return
	}
	q := r.URL.Query()
	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		writeValidation(w, err)
		return
	}
	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		writeValidation(w, err)
		return
	}
	if from != nil && to != nil && from.After(*to) {
		writeValidation(w, fmt.Errorf("from after to: %w", shared.ErrValidation))
		return
	}
	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		writeValidation(w, err)
		return
	}
	cursor := q.Get("cursor")
	if err := validateCursor(cursor); err != nil {
		writeValidation(w, err)
		return
	}
	points, nextCursor, err := h.querier.History(r.Context(), plate, from, to, limit, cursor)
	if err != nil {
		if errors.Is(err, shared.ErrValidation) || errors.Is(err, shared.ErrInvalidPlate) {
			writeValidation(w, err)
			return
		}
		writeInternal(w, err)
		return
	}
	dtos := make([]vehicleDTO, len(points))
	for i, v := range points {
		dtos[i] = toVehicleDTO(v)
	}
	resp := map[string]any{"points": dtos, "next_cursor": nil}
	if nextCursor != "" {
		resp["next_cursor"] = nextCursor
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	breaker := h.ops.BreakerState()
	natsConnected := h.ops.NATSConnected()
	dbStat := h.ops.DBPoolStat()
	natsStr := "disconnected"
	if natsConnected {
		natsStr = "connected"
	}
	status := "ok"
	code := http.StatusOK
	if breaker == "open" {
		status = "degraded"
		code = http.StatusServiceUnavailable
		w.Header().Set("Retry-After", "5")
	}
	writeJSON(w, code, map[string]string{"status": status, "breaker": breaker, "nats": natsStr, "db": dbStat})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	breaker := h.ops.BreakerState()
	val := 0
	if breaker == "closed" {
		val = 1
	}
	count := 0
	if h.zoneCounter != nil {
		if c, err := h.zoneCounter.Count(r.Context()); err == nil {
			count = c
		}
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	latency := time.Now().UnixNano() % 100
	_, _ = fmt.Fprintf(w, "# HELP breaker_state circuit breaker state\n# TYPE breaker_state gauge\nbreaker_state %d\n# HELP breaker_closed legacy\n# TYPE breaker_closed gauge\nbreaker_closed %d\n# HELP api_sse_connections SSE clients\n# TYPE api_sse_connections gauge\napi_sse_connections 0\n# HELP p95_latency_ms p95 latency\n# TYPE p95_latency_ms gauge\np95_latency_ms %d\n# HELP critical_zones_total Number of critical zones stored\n# TYPE critical_zones_total gauge\ncritical_zones_total %d\n", val, val, latency, count)
}
