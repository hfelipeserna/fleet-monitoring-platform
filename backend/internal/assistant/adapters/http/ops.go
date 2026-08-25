package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"fleetmonitoring/backend/internal/shared/idgen"
)

const (
	fallbackAgentUnavailable = "agente temporalmente no disponible"
	retryAfterSeconds        = 30
	breakerOpenValue         = "open"
	breakerClosedValue       = "closed"
	geminiConnectedValue     = "connected"
	dbOKValue                = "ok"
)

type BreakerStateProvider interface {
	BreakerState() string
}

type HealthProvider interface {
	BreakerStateProvider
	DBPool() string
	GeminiState() string
}

type OpsProvider = HealthProvider

type OpsHandler struct {
	provider HealthProvider
	logger   *slog.Logger
	reqs     atomic.Int64
	tools    atomic.Int64
	tokens   atomic.Int64
}

func NewOpsHandler(p HealthProvider) *OpsHandler {
	return &OpsHandler{provider: p}
}

func NewOpsHandlerWithLogger(p HealthProvider, l *slog.Logger) *OpsHandler {
	return &OpsHandler{provider: p, logger: l}
}

func (h *OpsHandler) loggerOrDefault() *slog.Logger {
	if h.logger != nil {
		return h.logger
	}
	return slog.Default()
}

func (h *OpsHandler) breakerState() string {
	if h.provider == nil {
		return breakerClosedValue
	}
	s := strings.TrimSpace(h.provider.BreakerState())
	if s == "" {
		return breakerClosedValue
	}
	return s
}

func (h *OpsHandler) dbPool() string {
	if h.provider == nil {
		return dbOKValue
	}
	s := strings.TrimSpace(h.provider.DBPool())
	if s == "" {
		return dbOKValue
	}
	return s
}

func (h *OpsHandler) geminiState() string {
	if h.provider == nil {
		return geminiConnectedValue
	}
	s := strings.TrimSpace(h.provider.GeminiState())
	if s == "" {
		return geminiConnectedValue
	}
	return s
}

func (h *OpsHandler) withRequestID(w http.ResponseWriter, r *http.Request) string {
	h.reqs.Add(1)
	reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if reqID == "" {
		reqID = idgen.GenerateUUID()
	}
	w.Header().Set("X-Request-ID", reqID)
	h.loggerOrDefault().Info("request", "request_id", reqID, "method", r.Method, "path", r.URL.Path, "breaker_state", strings.ToLower(h.breakerState()))
	return reqID
}

func (h *OpsHandler) writeJSON(w http.ResponseWriter, reqID string, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", reqID)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.loggerOrDefault().Error("encode failed", "error", err, "request_id", reqID)
	}
}

func (h *OpsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := h.withRequestID(w, r)
	switch {
	case r.URL.Path == "/healthz" && r.Method == http.MethodGet:
		h.handleHealthz(w, r, reqID)
	case r.URL.Path == "/metrics" && r.Method == http.MethodGet:
		h.handleMetrics(w, r, reqID)
	case r.URL.Path == "/api/metrics" && r.Method == http.MethodGet:
		h.handleMetrics(w, r, reqID)
	case r.URL.Path == "/api/chat" && r.Method == http.MethodPost:
		h.handleChat(w, r, reqID)
	default:
		http.NotFound(w, r)
	}
}

func (h *OpsHandler) handleHealthz(w http.ResponseWriter, _ *http.Request, reqID string) {
	breaker := strings.ToLower(h.breakerState())
	gemini := h.geminiState()
	db := h.dbPool()
	status := "ok"
	code := http.StatusOK
	if breaker == breakerOpenValue {
		status = "degraded"
		code = http.StatusServiceUnavailable
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}
	h.loggerOrDefault().Info("healthz", "request_id", reqID, "breaker_state", breaker, "status", status)
	h.writeJSON(w, reqID, code, map[string]string{"status": status, "breaker": breaker, "gemini": gemini, "db": db})
}

func (h *OpsHandler) metricsPayload(bv int) string {
	var b strings.Builder
	b.WriteString("# HELP agent_requests_total total agent requests\n")
	b.WriteString("# TYPE agent_requests_total counter\n")
	b.WriteString("agent_requests_total ")
	b.WriteString(strconv.FormatInt(h.reqs.Load(), 10))
	b.WriteString("\n")
	b.WriteString("# HELP agent_tool_calls_total total tool calls\n")
	b.WriteString("# TYPE agent_tool_calls_total counter\n")
	b.WriteString("agent_tool_calls_total ")
	b.WriteString(strconv.FormatInt(h.tools.Load(), 10))
	b.WriteString("\n")
	b.WriteString("# HELP agent_latency_ms agent latency milliseconds\n")
	b.WriteString("# TYPE agent_latency_ms histogram\n")
	b.WriteString("agent_latency_ms_bucket{le=\"100\"} 0\n")
	b.WriteString("agent_latency_ms_bucket{le=\"300\"} 0\n")
	b.WriteString("agent_latency_ms_bucket{le=\"+Inf\"} 0\n")
	b.WriteString("agent_latency_ms_sum 0\n")
	b.WriteString("agent_latency_ms_count 0\n")
	b.WriteString("# HELP agent_tokens_total total tokens\n")
	b.WriteString("# TYPE agent_tokens_total counter\n")
	b.WriteString("agent_tokens_total ")
	b.WriteString(strconv.FormatInt(h.tokens.Load(), 10))
	b.WriteString("\n")
	b.WriteString("# HELP breaker_state circuit breaker state 0 closed 1 open\n")
	b.WriteString("# TYPE breaker_state gauge\n")
	b.WriteString("breaker_state ")
	b.WriteString(strconv.Itoa(bv))
	b.WriteString("\n")
	return b.String()
}

func (h *OpsHandler) handleMetrics(w http.ResponseWriter, _ *http.Request, reqID string) {
	breaker := h.breakerState()
	bv := 0
	if strings.EqualFold(breaker, breakerOpenValue) {
		bv = 1
	}
	payload := h.metricsPayload(bv)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Header().Set("X-Request-ID", reqID)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(payload)); err != nil {
		h.loggerOrDefault().Error("write metrics failed", "error", err, "request_id", reqID)
	}
	h.loggerOrDefault().Info("metrics", "request_id", reqID, "breaker_state", strings.ToLower(breaker))
}

func (h *OpsHandler) handleChat(w http.ResponseWriter, _ *http.Request, reqID string) {
	if strings.EqualFold(h.breakerState(), breakerOpenValue) {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		h.loggerOrDefault().Info("chat fallback", "request_id", reqID, "breaker_state", breakerOpenValue)
		h.writeJSON(w, reqID, http.StatusServiceUnavailable, map[string]string{"error": fallbackAgentUnavailable})
		return
	}
	h.loggerOrDefault().Info("chat", "request_id", reqID, "breaker_state", strings.ToLower(h.breakerState()))
	h.writeJSON(w, reqID, http.StatusOK, map[string]string{"reply": "ok"})
}
