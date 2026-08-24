package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
	shared "fleetmonitoring/backend/internal/shared/domain"

	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

// Debt / depguard note (SPEC-003 Step2, ADR-0002):
// BFF POST /api/chat must validate via assistant/domain.ChatRequest.Validate
// per spec/plan, but fleet BC is forbidden to import assistant BC
// (backend/.golangci.yml: fleet-bc deny assistant, cmd/api deny assistant).
// Plan §12 Step2 says cmd/api will eventually import assistant/domain for Validate
// and assistant/adapters/http/client.go. That requires a depguard exception
// or moving AgentClient to shared/fleet adapter. For tests we keep the BFF
// handler in fleet/adapters/http and define a consumer-side AgentClient
// interface locally (string message, no assistant import) to keep depguard
// green. Validate still must happen via assistant/domain in production; if
// fleet imports assistant/domain directly, CI will need an allowlist.
// Tests assert behaviour (400 without LLM, 1..4000, 16KB) without importing
// assistant/domain, so they stay valid regardless of which layer does Validate.
// When production moves Validate into shared or adds exception, tests remain green.

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type chatResponse struct {
	Reply     string `json:"reply"`
	Citations []struct {
		Tool  string `json:"tool"`
		Count int    `json:"count"`
	} `json:"citations"`
	RequestID string `json:"request_id"`
}

type mockAgentClient struct {
	mu            sync.Mutex
	calls         int
	lastMessage   string
	lastRequestID string
	fn            func(ctx context.Context, message string) (chatResponse, error)
}

func (m *mockAgentClient) Chat(ctx context.Context, message string) (chatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastMessage = message
	if v := ctx.Value(shared.RequestIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			m.lastRequestID = s
		}
	} else if v := ctx.Value(requestIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			m.lastRequestID = s
		}
	}
	if m.fn != nil {
		return m.fn(ctx, message)
	}
	return chatResponse{Reply: "hola", Citations: []struct {
		Tool  string `json:"tool"`
		Count int    `json:"count"`
	}{}, RequestID: m.lastRequestID}, nil
}

func (m *mockAgentClient) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// requestIDKey is the expected context key for X-Request-ID propagation.
// Production must use same key or store request_id in ctx; tests check via this key.
// If production uses different key/mechanism, adapt test and production together.
type requestIDKey struct{}

func newChatHandler(t *testing.T, client *mockAgentClient) http.Handler {
	t.Helper()
	// Expected production symbol: fleethttp.NewChatHandler(AgentClient) http.Handler
	// This will fail RED until backend/internal/fleet/adapters/http/chat.go exists.
	// AgentClient is consumer-side interface defined in fleet adapter (no assistant import).
	// Production must export NewChatHandler that:
	//  - http.MaxBytesReader 16KB, json.Decode, ChatRequest.Validate (1..4000)
	//  - Content-Type must be application/json -> 400
	//  - rate.Limiter 10/min/IP via x/time/rate + sync.Map
	//  - breaker.Execute around client.Chat
	//  - context.WithTimeout 15s, X-Request-ID UUID, slog, map 400/429/503 with Retry-After, X-Accel-Buffering: no
	return fleethttp.NewChatHandler(clientAdapter{client})
}

// clientAdapter adapts local mockAgentClient to the production AgentClient interface.
// Production AgentClient is expected to be interface{ Chat(ctx context.Context, msg string) (any, error) }
// or with fleethttp.ChatRequest/ChatResponse. This adapter bridges via type assertion.
// If production uses assistant/domain.ChatRequest, adapter must be updated and depguard exception added.
type clientAdapter struct{ m *mockAgentClient }

func (a clientAdapter) Chat(ctx context.Context, msg string) (fleethttp.ChatResponse, error) {
	// This adapter expects production to define fleethttp.ChatRequest/ChatResponse.
	// Until chat.go exists this will not compile (RED). When production exists,
	// either keep string-based Chat or change adapter to use ChatRequest struct.
	resp, err := a.m.Chat(ctx, msg)
	if err != nil {
		return fleethttp.ChatResponse{}, err
	}
	return fleethttp.ChatResponse{
		Reply:     resp.Reply,
		Citations: nil,
		RequestID: resp.RequestID,
	}, nil
}

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %v: %v", v, err)
	}
	return bytes.NewReader(b)
}

func TestChatBFF(t *testing.T) {
	// Covers [SPEC-003: FR-001, BR-005/008, AC-004/009]
	t.Run("POST valid -> 200 proxy", func(t *testing.T) {
		// Covers [SPEC-003: AC-004, FR-001]
		// Arrange
		mock := &mockAgentClient{
			fn: func(ctx context.Context, message string) (chatResponse, error) {
				if message != "hola" {
					t.Errorf("expected message hola got %q", message)
				}
				return chatResponse{Reply: "hola", Citations: []struct {
					Tool  string `json:"tool"`
					Count int    `json:"count"`
				}{}}, nil
			},
		}
		h := newChatHandler(t, mock)
		body := jsonBody(t, map[string]string{"message": "hola"})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 got %d body %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("expected Content-Type application/json got %q", ct)
		}
		if v := rec.Header().Get("X-Accel-Buffering"); v != "no" {
			t.Fatalf("expected X-Accel-Buffering: no got %q", v)
		}
		var resp chatResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected json reply got %q err %v", rec.Body.String(), err)
		}
		if resp.Reply != "hola" {
			t.Fatalf("expected reply hola got %q body %s", resp.Reply, rec.Body.String())
		}
		if mock.Calls() != 1 {
			t.Fatalf("expected AgentClient called once got %d", mock.Calls())
		}
	})

	t.Run("message empty -> 400 without LLM", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009, BR-005]
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)
		body := jsonBody(t, map[string]string{"message": ""})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty message got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "validation") {
			t.Fatalf("expected validation error body got %s", rec.Body.String())
		}
		if mock.Calls() != 0 {
			t.Fatalf("expected 0 LLM calls for validation failure got %d", mock.Calls())
		}
	})

	t.Run("message 4001 chars -> 400 without LLM", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009, BR-005]
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)
		long := strings.Repeat("a", 4001)
		body := jsonBody(t, map[string]string{"message": long})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for 4001 chars got %d body %s", rec.Code, rec.Body.String())
		}
		if mock.Calls() != 0 {
			t.Fatalf("expected 0 LLM calls for 4001 chars got %d", mock.Calls())
		}
	})

	t.Run("body 17KB MaxBytes -> 400", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-005, FR-001]
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)
		// 17KB payload > 16KB limit (1<<14). Use message ~17KB.
		big := strings.Repeat("x", 17*1024)
		body := jsonBody(t, map[string]string{"message": big})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for 17KB body got %d body %s", rec.Code, rec.Body.String())
		}
		if mock.Calls() != 0 {
			t.Fatalf("expected 0 LLM calls for oversized body got %d", mock.Calls())
		}
	})

	t.Run("11 req per min same IP -> 429 Retry-After:6", func(t *testing.T) {
		// Covers [SPEC-003: AC-004, BR-005, FR-001]
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)
		ip := "198.51.100.9:1234"
		var lastRec *httptest.ResponseRecorder
		// Act: 11 sequential POSTs from same IP (<60s window, limiter 10/min)
		for i := 0; i < 11; i++ {
			body := jsonBody(t, map[string]string{"message": "hola"})
			req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = ip
			// Also send X-Forwarded-For to test LB extraction per plan
			req.Header.Set("X-Forwarded-For", "198.51.100.9")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			lastRec = rec
			if i < 10 && rec.Code == http.StatusTooManyRequests {
				t.Fatalf("unexpected 429 on request %d body %s", i+1, rec.Body.String())
			}
		}

		// Assert
		if lastRec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 on 11th request got %d body %s", lastRec.Code, lastRec.Body.String())
		}
		if got := lastRec.Header().Get("Retry-After"); got != "6" {
			t.Fatalf("expected Retry-After:6 got %q body %s", got, lastRec.Body.String())
		}
		if ct := lastRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("expected json Content-Type on 429 got %q", ct)
		}
		// Ensure no LLM call for the 429'd request (allow 10 calls, 11th blocked)
		if mock.Calls() != 10 {
			t.Fatalf("expected 10 LLM calls before rate limit got %d", mock.Calls())
		}
	})

	t.Run("Content-Type invalid -> 400", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, FR-001, BR-005]
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)
		body := jsonBody(t, map[string]string{"message": "hola"})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid Content-Type got %d body %s", rec.Code, rec.Body.String())
		}
		if mock.Calls() != 0 {
			t.Fatalf("expected 0 LLM calls for bad Content-Type got %d", mock.Calls())
		}
	})

	t.Run("X-Request-ID propagated and UUID", func(t *testing.T) {
		// Covers [SPEC-003: FR-001, BR-005]
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)

		// sub-case: client supplies X-Request-ID
		t.Run("propagates supplied UUID", func(t *testing.T) {
			// Arrange
			supplied := "123e4567-e89b-12d3-a456-426614174000"
			body := jsonBody(t, map[string]string{"message": "hola"})
			req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Request-ID", supplied)
			rec := httptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 got %d body %s", rec.Code, rec.Body.String())
			}
			got := rec.Header().Get("X-Request-ID")
			if got != supplied {
				t.Fatalf("expected X-Request-ID echo %q got %q", supplied, got)
			}
			if !uuidRe.MatchString(got) {
				t.Fatalf("expected UUID X-Request-ID got %q", got)
			}
			// also ensure propagated to AgentClient via context or captured field
			// mock captures via context; if production stores in ChatRequest.RequestID, check there
			// For RED we allow either; assert that mock saw same ID if it captures
			if mock.lastRequestID != "" && mock.lastRequestID != supplied {
				t.Fatalf("expected AgentClient request_id %q got %q", supplied, mock.lastRequestID)
			}
		})

		t.Run("generates UUID when missing", func(t *testing.T) {
			// Arrange
			mock2 := &mockAgentClient{}
			h2 := newChatHandler(t, mock2)
			body := jsonBody(t, map[string]string{"message": "hola"})
			req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
			req.Header.Set("Content-Type", "application/json")
			// no X-Request-ID header
			rec := httptest.NewRecorder()

			// Act
			h2.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 got %d body %s", rec.Code, rec.Body.String())
			}
			got := rec.Header().Get("X-Request-ID")
			if got == "" {
				t.Fatalf("expected generated X-Request-ID got empty")
			}
			if !uuidRe.MatchString(got) {
				t.Fatalf("expected generated UUID got %q", got)
			}
		})

		t.Run("invalid X-Request-ID -> 400 or regenerate UUID", func(t *testing.T) {
			// Covers BR-009 zoneId UUID strictness analog; spec says X-Request-ID UUID validation
			// Policy: if header present but not UUID, either 400 or ignore and generate new UUID.
			// This test documents expectation: handler must not propagate invalid UUID.
			// We accept either 400 or 200 with new UUID, but must not echo invalid value.
			// Arrange
			mock3 := &mockAgentClient{}
			h3 := newChatHandler(t, mock3)
			body := jsonBody(t, map[string]string{"message": "hola"})
			req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Request-ID", "not-a-uuid")
			rec := httptest.NewRecorder()

			// Act
			h3.ServeHTTP(rec, req)

			// Assert
			got := rec.Header().Get("X-Request-ID")
			if got == "not-a-uuid" {
				t.Fatalf("must not echo invalid X-Request-ID %q", got)
			}
			if rec.Code == http.StatusBadRequest {
				if !strings.Contains(strings.ToLower(rec.Body.String()), "request") {
					t.Fatalf("expected request_id validation message got %s", rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 or 400 for invalid X-Request-ID got %d body %s", rec.Code, rec.Body.String())
			}
			if !uuidRe.MatchString(got) {
				t.Fatalf("expected regenerated UUID got %q", got)
			}
		})
	})

	t.Run("breaker open -> 503 Retry-After:30 without LLM", func(t *testing.T) {
		// Covers [SPEC-003: AC-004, BR-005, FR-001, FR-006]
		// Arrange
		// We test two paths: if handler exposes WithBreaker, use pre-opened breaker;
		// else trip internal breaker via repeated failures.
		mock := &mockAgentClient{
			fn: func(ctx context.Context, message string) (chatResponse, error) {
				return chatResponse{}, fmt.Errorf("upstream fail: %w", fmt.Errorf("gemini unavailable"))
			},
		}
		// Try injected breaker path first: create gobreaker that is already open.
		// gobreaker.StateOpen after consecutive failures.
		breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "test-chat-breaker",
			MaxRequests: 1,
			Interval:    60 * time.Second,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(c gobreaker.Counts) bool {
				return c.ConsecutiveFailures >= 3
			},
		})
		// Pre-trip breaker via direct Execute failures
		for i := 0; i < 5; i++ {
			_, _ = breaker.Execute(func() (any, error) { return nil, fmt.Errorf("fail") })
		}
		if breaker.State() != gobreaker.StateOpen {
			t.Fatalf("setup: expected breaker open after pre-trip got %v", breaker.State())
		}
		// Use handler with injected open breaker if available, else use default handler and trip via handler loop.
		var h http.Handler
		if hh, ok := tryNewChatHandlerWithBreaker(mock, breaker); ok {
			h = hh
			// reset mock to success to verify no call when open
			mock.fn = func(ctx context.Context, message string) (chatResponse, error) {
				return chatResponse{Reply: "should not be called"}, nil
			}
			mock.mu.Lock()
			mock.calls = 0
			mock.mu.Unlock()
		} else {
			// Fallback: use default handler and trip it via 10 failing requests
			h = newChatHandler(t, mock)
			for i := 0; i < 10; i++ {
				body := jsonBody(t, map[string]string{"message": "hola"})
				req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
				req.Header.Set("Content-Type", "application/json")
				req.RemoteAddr = fmt.Sprintf("10.0.0.%d:1234", i%5+1) // vary IP to avoid rate limit
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
			}
			// Now breaker should be open; swap mock to success
			mock.fn = func(ctx context.Context, message string) (chatResponse, error) {
				return chatResponse{Reply: "should not be called"}, nil
			}
			mock.mu.Lock()
			mock.calls = 0
			mock.mu.Unlock()
		}

		body := jsonBody(t, map[string]string{"message": "hola"})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.1:1234"
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 breaker open got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Fatalf("expected Retry-After:30 got %q body %s", got, rec.Body.String())
		}
		if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
			t.Fatalf("expected X-Accel-Buffering: no on 503 got %q", got)
		}
		if mock.Calls() != 0 {
			t.Fatalf("expected 0 LLM calls when breaker open got %d", mock.Calls())
		}
	})
}

func TestChatBFF_Timeout(t *testing.T) {
	// Covers [SPEC-003: AC-004, BR-005, FR-001, FR-006]
	// Arrange
	// Use short timeout for fast test if handler supports injection; otherwise use 15s real.
	mock := &mockAgentClient{
		fn: func(ctx context.Context, message string) (chatResponse, error) {
			// Respect ctx cancellation; simulate 16s work with select
			select {
			case <-time.After(16 * time.Second):
				return chatResponse{Reply: "late"}, nil
			case <-ctx.Done():
				return chatResponse{}, ctx.Err()
			}
		},
	}
	var h http.Handler
	var timeout time.Duration
	if hh, d, ok := tryNewChatHandlerWithTimeout(mock, 50*time.Millisecond); ok {
		h = hh
		timeout = d
		// Override fn to respect short timeout (200ms sleep > 50ms)
		mock.fn = func(ctx context.Context, message string) (chatResponse, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return chatResponse{Reply: "late"}, nil
			case <-ctx.Done():
				return chatResponse{}, ctx.Err()
			}
		}
	} else {
		// Fallback to default 15s handler (slow path). Use a faster mock that checks deadline instead of sleeping full 16s.
		// To keep test under 16s, we check ctx deadline and return timeout error if deadline ~15s.
		// This still validates handler sets 15s timeout without sleeping 16s.
		h = newChatHandler(t, mock)
		timeout = 15 * time.Second
		mock.fn = func(ctx context.Context, message string) (chatResponse, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Errorf("expected deadline ~15s got none")
				select {
				case <-time.After(16 * time.Second):
					return chatResponse{Reply: "late"}, nil
				case <-ctx.Done():
					return chatResponse{}, ctx.Err()
				}
			}
			remaining := time.Until(deadline)
			// Handler should set ~15s; allow 12-16s window
			if remaining < 12*time.Second || remaining > 16*time.Second {
				t.Errorf("expected context timeout ~15s got %v", remaining)
			}
			// Simulate work that exceeds timeout
			select {
			case <-time.After(remaining + 1*time.Second):
				return chatResponse{Reply: "late"}, nil
			case <-ctx.Done():
				return chatResponse{}, ctx.Err()
			}
		}
	}

	body := jsonBody(t, map[string]string{"message": "hola"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	start := time.Now()

	// Act
	h.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	// Assert
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 timeout got %d body %s elapsed %v timeout %v", rec.Code, rec.Body.String(), elapsed, timeout)
	}
	if got := rec.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("expected Retry-After:30 got %q body %s", got, rec.Body.String())
	}
	if elapsed > timeout+2*time.Second {
		t.Fatalf("expected timeout ~%v got elapsed %v", timeout, elapsed)
	}
	// Ensure handler did not return 200 with late reply
	if strings.Contains(rec.Body.String(), "late") {
		t.Fatalf("unexpected late reply propagated %s", rec.Body.String())
	}
}

func TestChatBFF_TableValidation(t *testing.T) {
	// Covers [SPEC-003: AC-009, BR-009]
	cases := []struct {
		name        string
		contentType string
		payload     any
		rawBody     string
	}{
		{"empty message", "application/json", map[string]string{"message": ""}, ""},
		{"spaces only", "application/json", map[string]string{"message": "   "}, ""},
		{"missing message field", "application/json", map[string]any{}, ""},
		{"message non-string", "application/json", map[string]any{"message": 123}, ""},
		{"extra fields ignored but message empty still 400", "application/json", map[string]any{"message": "", "role": "system"}, ""},
		{"invalid json", "application/json", nil, `{"message":`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			mock := &mockAgentClient{}
			h := newChatHandler(t, mock)
			var body *bytes.Reader
			if tc.rawBody != "" {
				body = bytes.NewReader([]byte(tc.rawBody))
			} else {
				body = jsonBody(t, tc.payload)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %q got %d body %s", tc.name, rec.Code, rec.Body.String())
			}
			if mock.Calls() != 0 {
				t.Fatalf("expected 0 LLM calls for %q got %d", tc.name, mock.Calls())
			}
		})
	}
}

func tryNewChatHandlerWithBreaker(client *mockAgentClient, breaker *gobreaker.CircuitBreaker) (http.Handler, bool) {
	return fleethttp.NewChatHandlerWithBreaker(clientAdapter{client}, breaker), true
}

func tryNewChatHandlerWithTimeout(client *mockAgentClient, d time.Duration) (http.Handler, time.Duration, bool) {
	h := fleethttp.NewChatHandlerWithBreakerAndTimeout(clientAdapter{client}, nil, d)
	return h, d, true
}

// Keep imports used for RED compilation check.
var _ = gobreaker.StateClosed
var _ = rate.Limit(10)

func TestChatBFF_RateLimiterUsesXForwardedFor(t *testing.T) {
	// Covers [SPEC-003: BR-005, FR-001]
	// Arrange: already covered in TestChatBFF 11 req test via X-Forwarded-For,
	// this test isolates IP extraction logic.
	t.Run("different IPs have independent buckets", func(t *testing.T) {
		// Arrange
		mock := &mockAgentClient{}
		h := newChatHandler(t, mock)
		// Act: 10 requests from IP A should not rate limit IP B
		for i := range 10 {
			body := jsonBody(t, map[string]string{"message": "hola"})
			req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "192.0.2.10:1234"
			req.Header.Set("X-Forwarded-For", "192.0.2.10")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 for IP A request %d got %d body %s", i+1, rec.Code, rec.Body.String())
			}
		}
		// 1 request from different IP should still be 200
		body := jsonBody(t, map[string]string{"message": "hola"})
		req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.20:1234"
		req.Header.Set("X-Forwarded-For", "192.0.2.20")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for different IP got %d body %s", rec.Code, rec.Body.String())
		}
	})
}
