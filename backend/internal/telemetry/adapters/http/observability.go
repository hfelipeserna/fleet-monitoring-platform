package http

import (
	"fmt"
	"net/http"
	"strings"
)

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\"", `\"`)
	return s
}

func (h *handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	state := breakerState(h.breaker)
	jet := jetstreamStatus(h.js)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"breaker":   state,
		"jetstream": jet,
	})
}

func breakerState(b Breaker) string {
	if b == nil {
		return "closed"
	}
	state := b.State()
	if b.IsOpen() {
		return "open"
	}
	return state
}

func jetstreamStatus(js JetStreamInfo) string {
	if js == nil {
		return "connected"
	}
	used, max := js.Bytes()
	return fmt.Sprintf("%d/%d", used, max)
}

func (h *handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	writeMetricsGauges(w, h)
	writeTelemetryCounters(w, h)
}

func writeMetricsGauges(w http.ResponseWriter, h *handler) {
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
}

func writeTelemetryCounters(w http.ResponseWriter, h *handler) {
	fmt.Fprintf(w, "# HELP telemetry_ingest_total total telemetry events ingested via 202\n")
	fmt.Fprintf(w, "# TYPE telemetry_ingest_total counter\n")
	h.metrics.Range(func(plate string, val int64) bool {
		fmt.Fprintf(w, "telemetry_ingest_total{plate=\"%s\"} %d\n", escapeLabel(plate), val)
		return true
	})
	fmt.Fprintf(w, "# HELP telemetry_ingest_total_sum total telemetry events ingested via 202\n")
	fmt.Fprintf(w, "# TYPE telemetry_ingest_total_sum counter\n")
	fmt.Fprintf(w, "telemetry_ingest_total_sum %d\n", h.metrics.Total())
}
