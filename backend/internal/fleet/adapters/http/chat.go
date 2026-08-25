package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/shared/idgen"

	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

// MVP anonymous: nginx must not expose /api/chat to 0.0.0.0 without auth per ADR-0003; rate limiting is only DoS defense.
// Auth debt: if Authorization header present, future middleware should parse JWT and inject claims; currently not enforced.

const (
	BodyLimit         = 1 << 14
	retryAfterRate    = "6"
	retryAfterBreaker = "30"
	retryAfterTimeout = "30"
	defaultTimeout    = 15 * time.Second
	limiterBurst      = 10
	limiterIdleTTL    = 10 * time.Minute
	breakerMinRequests = 5
	breakerInterval   = 30 * time.Second
	breakerTimeout    = 30 * time.Second
)

var limiterRate = rate.Every(time.Minute / 10)

type Citation struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

type ChatResponse struct {
	Reply     string     `json:"reply"`
	Citations []Citation `json:"citations,omitempty"`
	RequestID string     `json:"request_id,omitempty"`
}

type AgentClient interface {
	Chat(ctx context.Context, message string) (ChatResponse, error)
}

type ChatHandler struct {
	client    AgentClient
	breaker   *gobreaker.CircuitBreaker
	limiters  sync.Map
	sweepOnce sync.Once
	timeout   time.Duration
}

type ChatHandlerOptions struct {
	Breaker   *gobreaker.CircuitBreaker
	Timeout   time.Duration
	RateLimit int
}

func newDefaultBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name: "chat",
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < breakerMinRequests {
				return false
			}
			ratio := float64(c.TotalFailures) / float64(c.Requests)
			return c.ConsecutiveFailures >= 3 || ratio >= 0.5
		},
		Interval:    breakerInterval,
		Timeout:     breakerTimeout,
		MaxRequests: 1,
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Info("breaker state change", "name", name, "from", from.String(), "to", to.String())
		},
	})
}

func NewChatHandler(client AgentClient) *ChatHandler {
	return NewChatHandlerWithOptions(client, ChatHandlerOptions{})
}

func NewChatHandlerWithOptions(client AgentClient, opts ChatHandlerOptions) *ChatHandler {
	b := opts.Breaker
	if b == nil {
		b = newDefaultBreaker()
	}
	to := opts.Timeout
	if to <= 0 {
		to = defaultTimeout
	}
	return &ChatHandler{
		client:  client,
		breaker: b,
		timeout: to,
	}
}

func NewChatHandlerWithBreaker(client AgentClient, cb *gobreaker.CircuitBreaker) *ChatHandler {
	return NewChatHandlerWithOptions(client, ChatHandlerOptions{Breaker: cb})
}

func NewChatHandlerWithBreakerAndTimeout(client AgentClient, cb *gobreaker.CircuitBreaker, timeout time.Duration) *ChatHandler {
	return NewChatHandlerWithOptions(client, ChatHandlerOptions{Breaker: cb, Timeout: timeout})
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Accel-Buffering", "no")
	reqID := h.ensureRequestID(w, r)
	if !h.checkMethod(w, r) {
		return
	}
	if !h.checkContentType(w, r) {
		return
	}
	trimmed, ok := h.decodeAndValidate(w, r)
	if !ok {
		return
	}
	if !h.checkRateLimit(w, r) {
		return
	}
	if h.breaker.State() == gobreaker.StateOpen {
		w.Header().Set("Retry-After", retryAfterBreaker)
		writeChatJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(shared.WithRequestID(r.Context(), reqID), h.timeout)
	defer cancel()
	h.doChat(w, ctx, trimmed, reqID)
}

func (h *ChatHandler) doChat(w http.ResponseWriter, ctx context.Context, trimmed, reqID string) {
	raw, err := h.breaker.Execute(func() (any, error) {
		return h.client.Chat(ctx, trimmed)
	})
	if err != nil {
		code, retry := classifyChatError(err, ctx.Err())
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		if code == http.StatusServiceUnavailable {
			writeChatJSON(w, code, map[string]string{"error": "service unavailable"})
			return
		}
		slog.Error("chat upstream error", "request_id", reqID)
		writeChatJSON(w, code, map[string]string{"error": "upstream error"})
		return
	}
	resp, ok := raw.(ChatResponse)
	if !ok {
		slog.Error("chat invalid response type", "request_id", reqID)
		writeChatJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	resp.RequestID = reqID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("write response failed", "request_id", reqID, "error", err)
	}
}

func (h *ChatHandler) ensureRequestID(w http.ResponseWriter, r *http.Request) string {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" || !shared.IsValidUUID(reqID) {
		if reqID != "" {
			sanitized := reqID
			if len(sanitized) > 64 {
				sanitized = sanitized[:64]
			}
			sanitized = strings.ReplaceAll(sanitized, "\n", "")
			sanitized = strings.ReplaceAll(sanitized, "\r", "")
			slog.Info("invalid X-Request-ID, regenerating", "provided", sanitized)
		}
		reqID = idgen.GenerateUUID()
	}
	w.Header().Set("X-Request-ID", reqID)
	return reqID
}

func (h *ChatHandler) checkMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeChatJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return false
	}
	return true
}

func (h *ChatHandler) checkContentType(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "Content-Type must be application/json"})
		return false
	}
	base, _, err := mime.ParseMediaType(ct)
	if err != nil || base != "application/json" {
		writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "Content-Type must be application/json"})
		return false
	}
	return true
}

func (h *ChatHandler) decodeAndValidate(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, BodyLimit)
	var payload struct {
		Message *string `json:"message"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		if isChatMaxBytesError(err) {
			w.Header().Set("Connection", "close")
			writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "request body too large"})
			return "", false
		}
		writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "invalid json"})
		return "", false
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "invalid json"})
		return "", false
	}
	if payload.Message == nil {
		writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "message is required"})
		return "", false
	}
	if err := shared.ValidateMessage(*payload.Message); err != nil {
		writeChatJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": "message must be 1..4000 runes"})
		return "", false
	}
	trimmed := strings.TrimSpace(*payload.Message)
	return trimmed, true
}

func (h *ChatHandler) checkRateLimit(w http.ResponseWriter, r *http.Request) bool {
	ip := getIP(r)
	lim := h.getLimiter(ip)
	if !lim.Allow() {
		w.Header().Set("Retry-After", retryAfterRate)
		writeChatJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
		return false
	}
	return true
}

func classifyChatError(err error, ctxErr error) (int, string) {
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return http.StatusServiceUnavailable, retryAfterBreaker
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctxErr, context.DeadlineExceeded) {
		return http.StatusServiceUnavailable, retryAfterTimeout
	}
	return http.StatusBadGateway, ""
}

func (h *ChatHandler) startSweep() {
	h.sweepOnce.Do(func() {
		go func() {
			for {
				time.Sleep(10 * time.Minute)
				now := time.Now().UnixNano()
				h.limiters.Range(func(k, v any) bool {
					e := v.(*limiterEntry)
					if now-e.lastSeen.Load() > int64(limiterIdleTTL) {
						h.limiters.Delete(k)
					}
					return true
				})
			}
		}()
	})
}

func (h *ChatHandler) getLimiter(ip string) *rate.Limiter {
	if v, ok := h.limiters.Load(ip); ok {
		e := v.(*limiterEntry)
		e.lastSeen.Store(time.Now().UnixNano())
		return e.limiter
	}
	h.startSweep()
	e := &limiterEntry{limiter: rate.NewLimiter(limiterRate, limiterBurst)}
	e.lastSeen.Store(time.Now().UnixNano())
	actual, loaded := h.limiters.LoadOrStore(ip, e)
	if loaded {
		ae := actual.(*limiterEntry)
		ae.lastSeen.Store(time.Now().UnixNano())
		return ae.limiter
	}
	return e.limiter
}

func writeChatJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json failed", "error", err)
	}
}

func isChatMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
