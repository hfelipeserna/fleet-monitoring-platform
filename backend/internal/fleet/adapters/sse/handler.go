package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

const sseBufferSize = 256 // channel buffer for SSE streams, fits 16GB host (64 was drop-prone)

type AlertSubscriber interface {
	SubscribeAlerts(ctx context.Context, lastSeq uint64) (<-chan AlertMsg, func(), error)
}

type TelemetrySubscriber interface {
	SubscribePositions(ctx context.Context, plate *string, lastSeq uint64) (<-chan PosMsg, func(), error)
}

type Handler struct {
	alerts       AlertSubscriber
	positions    TelemetrySubscriber
	pingInterval time.Duration
}

func NewHandler(alerts AlertSubscriber, positions TelemetrySubscriber, opts ...Option) http.Handler {
	cfg := handlerConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	interval := cfg.pingInterval
	if interval == 0 {
		interval = 15 * time.Second
	}
	return &Handler{alerts: alerts, positions: positions, pingInterval: interval}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/alerts":
		h.handleAlerts(w, r)
	case "/api/fleet/positions/stream":
		h.handleFleet(w, r)
	default:
		http.NotFound(w, r)
	}
}

func validateAccept(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

func parseLastSeq(r *http.Request) (uint64, error) {
	v := r.Header.Get("Last-Event-ID")
	if v == "" {
		return 0, nil
	}
	v = strings.TrimSpace(v)
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Last-Event-ID %q: %w", v, err)
	}
	if n == math.MaxUint64 {
		return 0, fmt.Errorf("overflow Last-Event-ID %q: %w", v, shared.ErrValidation)
	}
	return n + 1, nil
}

func setupSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

func writeUnavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "5")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprint(w, "retry: 5000\n")
}

func writeSSEMessage(w http.ResponseWriter, flusher http.Flusher, id uint64, event string, data []byte) {
	fmt.Fprintf(w, "id: %d\n", id)
	fmt.Fprintf(w, "event: %s\n", event)
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		fmt.Fprintf(w, "data: %s\n", l)
	}
	fmt.Fprint(w, "\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func serveStream[T any](h *Handler, w http.ResponseWriter, r *http.Request, subscribe func(context.Context, uint64) (<-chan T, func(), error), encode func(T) (uint64, string, []byte)) {
	if !validateAccept(r) {
		http.Error(w, "accept must be text/event-stream", http.StatusBadRequest)
		return
	}
	lastSeq, err := parseLastSeq(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ch, unsub, err := subscribe(r.Context(), lastSeq)
	if err != nil {
		writeUnavailable(w)
		return
	}
	if unsub != nil {
		defer unsub()
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Warn("flusher not supported")
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	setupSSE(w)
	fmt.Fprintf(w, "retry: 5000\n\n")
	flusher.Flush()
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			id, event, data := encode(msg)
			writeSSEMessage(w, flusher, id, event, data)
		case <-ticker.C:
			fmt.Fprintf(w, ":ping\n\n")
			flusher.Flush()
		}
	}
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if h.alerts == nil {
		writeUnavailable(w)
		return
	}
	serveStream(h, w, r, h.alerts.SubscribeAlerts, func(m AlertMsg) (uint64, string, []byte) {
		return m.Seq, "alert:critical", m.Data
	})
}

func encodeFleet(m PosMsg) (uint64, string, []byte) {
	if len(m.Data) > 0 {
		var tmp struct {
			Plate      string    `json:"plate"`
			Lat        *float64  `json:"lat"`
			Lon        *float64  `json:"lon"`
			Speed      int       `json:"speed"`
			ReceivedAt time.Time `json:"received_at"`
		}
		if err := json.Unmarshal(m.Data, &tmp); err == nil {
			if tmp.Lat != nil {
				v := shared.Round6(*tmp.Lat)
				tmp.Lat = &v
			}
			if tmp.Lon != nil {
				v := shared.Round6(*tmp.Lon)
				tmp.Lon = &v
			}
			if data, err := json.Marshal(tmp); err == nil {
				return m.Seq, "fleet:position", data
			}
		}
		return m.Seq, "fleet:position", m.Data
	}
	if m.Lat != nil {
		v := shared.Round6(*m.Lat)
		m.Lat = &v
	}
	if m.Lon != nil {
		v := shared.Round6(*m.Lon)
		m.Lon = &v
	}
	payload := struct {
		Plate      string    `json:"plate"`
		Lat        *float64  `json:"lat"`
		Lon        *float64  `json:"lon"`
		Speed      int       `json:"speed"`
		ReceivedAt time.Time `json:"received_at"`
	}{
		Plate:      m.Plate,
		Lat:        m.Lat,
		Lon:        m.Lon,
		Speed:      m.Speed,
		ReceivedAt: m.ReceivedAt,
	}
	data, _ := json.Marshal(payload)
	return m.Seq, "fleet:position", data
}

func (h *Handler) handleFleet(w http.ResponseWriter, r *http.Request) {
	plateStr := r.URL.Query().Get("plate")
	var plate *string
	if plateStr != "" {
		if _, err := shared.ParsePlate(plateStr); err != nil {
			http.Error(w, fmt.Sprintf("invalid plate: %v", err), http.StatusBadRequest)
			return
		}
		plate = &plateStr
	}
	if h.positions == nil {
		writeUnavailable(w)
		return
	}
	serveStream(h, w, r, func(ctx context.Context, lastSeq uint64) (<-chan PosMsg, func(), error) {
		return h.positions.SubscribePositions(ctx, plate, lastSeq)
	}, encodeFleet)
}
