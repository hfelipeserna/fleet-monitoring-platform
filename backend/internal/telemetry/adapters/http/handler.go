package http

import (
	"net/http"
	"sync/atomic"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
	"fleetmonitoring/backend/internal/telemetry/infra/metrics"
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
	metrics  metrics.CounterVec
}

func NewHandler(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo) http.Handler {
	svc := application.NewIngestService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
	return NewHandlerWithService(svc, breaker, js)
}

func NewHandlerWithService(svc *application.IngestService, breaker Breaker, js JetStreamInfo) http.Handler {
	return &handler{svc: svc, breaker: breaker, js: js}
}

func (h *handler) recordIngest(plate string, n int) {
	h.metrics.Add(plate, int64(n))
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
