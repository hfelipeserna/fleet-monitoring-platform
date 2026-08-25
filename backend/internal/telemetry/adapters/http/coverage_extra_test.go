package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fleetmonitoring/backend/internal/telemetry/application"
)

// Covers [SPEC-001: AC-004, BR-005, BR-006, FR-004]

func TestCoverage_TelemetryHelpers(t *testing.T) {
	t.Run("mapServiceError validation rate backpressure strings", func(t *testing.T) {
		// Arrange
		// Act validation via ErrValidation
		rec := httptest.NewRecorder()
		mapServiceError(rec, application.ErrValidation)
		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 validation, got %d", rec.Code)
		}
		// rate limited via ErrRateLimited
		rec2 := httptest.NewRecorder()
		mapServiceError(rec2, application.ErrRateLimited)
		if rec2.Code != http.StatusTooManyRequests || rec2.Header().Get("Retry-After") != "5" {
			t.Fatalf("expected 429 rate, got %d header %q", rec2.Code, rec2.Header().Get("Retry-After"))
		}
		// backpressure via ErrBackpressure
		rec3 := httptest.NewRecorder()
		mapServiceError(rec3, application.ErrBackpressure)
		if rec3.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 backpressure, got %d", rec3.Code)
		}
		// validation via string contains
		rec4 := httptest.NewRecorder()
		mapServiceError(rec4, errors.New("some validation failed"))
		if rec4.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 via string validation, got %d", rec4.Code)
		}
		// rate via string
		rec5 := httptest.NewRecorder()
		mapServiceError(rec5, errors.New("rate limit exceeded"))
		if rec5.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 via string rate, got %d", rec5.Code)
		}
		// backpressure default
		rec6 := httptest.NewRecorder()
		mapServiceError(rec6, errors.New("other error"))
		if rec6.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 default, got %d", rec6.Code)
		}
	})

	t.Run("isMaxBytesError true for MaxBytesError and string", func(t *testing.T) {
		// Arrange
		mbe := &http.MaxBytesError{Limit: 1024}
		// Act
		a := isMaxBytesError(mbe)
		b := isMaxBytesError(errors.New("request body too large"))
		c := isMaxBytesError(errors.New("other"))
		d := isMaxBytesError(nil)
		// Assert
		if !a || !b {
			t.Fatalf("expected true, got %v %v", a, b)
		}
		if c || d {
			t.Fatalf("expected false, got %v %v", c, d)
		}
	})

	t.Run("handleBodyError 413 vs 400", func(t *testing.T) {
		// Arrange
		h := &handler{}
		rec := httptest.NewRecorder()
		// Act 413
		h.handleBodyError(rec, &http.MaxBytesError{Limit: 1024})
		// Assert
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d", rec.Code)
		}
		rec2 := httptest.NewRecorder()
		h.handleBodyError(rec2, errors.New("other"))
		if rec2.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec2.Code)
		}
	})

	t.Run("escapeLabel replaces", func(t *testing.T) {
		// Arrange
		// Act
		a := escapeLabel(`a\b"c
d`)
		// Assert
		if !strings.Contains(a, `\\`) || !strings.Contains(a, `\"`) || !strings.Contains(a, `\n`) {
			t.Fatalf("expected escaped, got %q", a)
		}
	})

	t.Run("breakerState nil closed open", func(t *testing.T) {
		// Arrange
		bNil := breakerState(nil)
		bClosed := breakerState(&fakeBreakerTel{state: "closed", open: false})
		bOpen := breakerState(&fakeBreakerTel{state: "open", open: true})
		// Act
		// Assert
		if bNil != "closed" || bClosed != "closed" || bOpen != "open" {
			t.Fatalf("expected closed closed open, got %q %q %q", bNil, bClosed, bOpen)
		}
	})

	t.Run("jetstreamStatus nil and with js", func(t *testing.T) {
		// Arrange
		a := jetstreamStatus(nil)
		b := jetstreamStatus(&fakeJSTel{used: 10, max: 100})
		// Act
		// Assert
		if a != "connected" {
			t.Fatalf("expected connected nil, got %q", a)
		}
		if b != "10/100" {
			t.Fatalf("expected 10/100, got %q", b)
		}
	})

	t.Run("handleHealthz and handleMetrics method not allowed", func(t *testing.T) {
		// Arrange
		h := &handler{breaker: &fakeBreakerTel{state: "closed"}, js: &fakeJSTel{}}
		req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		rec := httptest.NewRecorder()
		// Act
		h.handleHealthz(rec, req)
		// Assert
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 healthz POST, got %d", rec.Code)
		}
		req2 := httptest.NewRequest(http.MethodPost, "/metrics", nil)
		rec2 := httptest.NewRecorder()
		h.handleMetrics(rec2, req2)
		if rec2.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 metrics POST, got %d", rec2.Code)
		}
		// success GET
		req3 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec3 := httptest.NewRecorder()
		h.handleHealthz(rec3, req3)
		if rec3.Code != http.StatusOK {
			t.Fatalf("expected 200 healthz GET, got %d", rec3.Code)
		}
		if !strings.Contains(rec3.Body.String(), "ok") {
			t.Fatalf("expected ok body, got %q", rec3.Body.String())
		}
		req4 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec4 := httptest.NewRecorder()
		h.handleMetrics(rec4, req4)
		if rec4.Code != http.StatusOK {
			t.Fatalf("expected 200 metrics GET, got %d", rec4.Code)
		}
		if !strings.Contains(rec4.Body.String(), "breaker_state") {
			t.Fatalf("expected breaker_state, got %q", rec4.Body.String())
		}
	})

	t.Run("readBody empty -> validation and maxBytes", func(t *testing.T) {
		// Arrange
		h := &handler{}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.Body = http.NoBody
		rec := httptest.NewRecorder()
		// Act
		_, err := h.readBody(req, rec)
		// Assert
		if err == nil || !strings.Contains(err.Error(), "empty body") {
			t.Fatalf("expected empty body error, got %v", err)
		}
	})
}

type fakeBreakerTel struct {
	state string
	open  bool
	err   error
}
func (f *fakeBreakerTel) State() string { return f.state }
func (f *fakeBreakerTel) IsOpen() bool  { return f.open }
func (f *fakeBreakerTel) Allow() error  { return f.err }

type fakeJSTel struct {
	used uint64
	max  uint64
}
func (f *fakeJSTel) Bytes() (uint64, uint64) { return f.used, f.max }
