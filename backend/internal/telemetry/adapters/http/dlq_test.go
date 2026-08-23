package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Covers [SPEC-001: AC-008, FR-009, BR-010] TEST-008
// POST /internal/dlq/republish -> republish manual desde telemetry.dlq a TELEMETRY subject
//
// Expected production API (forces red until implemented):
//   func NewDLQHandler(js JetStreamContext, opts DLQOptions) http.Handler
//   or func NewHandlerWithDLQ(...) http.Handler with route POST /internal/dlq/republish
//   JetStreamContext expected to have methods: Pull DLQ, Fetch, Publish
//   Handler should: fetch messages from telemetry.dlq, republish to TELEMETRY stream subject telemetry.raw.{plate},
//   and Ack/Term DLQ messages. Returns JSON {republished: N, errors: 0}
//
// If production uses different name (e.g., dlq.Handler), update tests together.

// ---- fakes ----

type fakeDLQJetStream struct {
	dlqMessages [][]byte // messages currently in DLQ
	republished []struct {
		subject string
		data    []byte
	}
	fetchErr error
	pubErr   error
}

type fakeDLQMsg struct {
	data []byte
	f    *fakeDLQJetStream
}

func (m *fakeDLQMsg) Data() []byte { return m.data }
func (m *fakeDLQMsg) Ack() error {
	for i, d := range m.f.dlqMessages {
		if string(d) == string(m.data) {
			m.f.dlqMessages = append(m.f.dlqMessages[:i], m.f.dlqMessages[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeDLQJetStream) FetchDLQ(n int) ([]DLQMsg, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	if n > len(f.dlqMessages) {
		n = len(f.dlqMessages)
	}
	out := make([]DLQMsg, n)
	for i := 0; i < n; i++ {
		cp := make([]byte, len(f.dlqMessages[i]))
		copy(cp, f.dlqMessages[i])
		out[i] = &fakeDLQMsg{data: cp, f: f}
	}
	return out, nil
}

func (f *fakeDLQJetStream) RepublishToTelemetry(data []byte, plate string) error {
	if f.pubErr != nil {
		return f.pubErr
	}
	subject := "telemetry.raw." + plate
	f.republished = append(f.republished, struct {
		subject string
		data    []byte
	}{subject, data})
	return nil
}

func (f *fakeDLQJetStream) RepublishRaw(subject string, data []byte) error {
	if f.pubErr != nil {
		return f.pubErr
	}
	f.republished = append(f.republished, struct {
		subject string
		data    []byte
	}{subject, data})
	return nil
}

func (f *fakeDLQJetStream) AckDLQ(count int) error {
	if count > len(f.dlqMessages) {
		count = len(f.dlqMessages)
	}
	f.dlqMessages = f.dlqMessages[count:]
	return nil
}

func validDLQPayload(plate, clientID string) []byte {
	m := map[string]any{
		"plate":           plate,
		"speed":           42,
		"lat":             4.711,
		"lon":             -74.072,
		"client_event_id": clientID,
		"received_at":     "2026-08-23T10:00:00Z",
	}
	b, _ := json.Marshal(m)
	return b
}

// helper to build DLQ handler — will fail compile until production implements it
func buildDLQHandler(js *fakeDLQJetStream) http.Handler {
	// Expected: func NewDLQHandler(js DLQJetStream) http.Handler
	// or func NewRepublishHandler(js DLQJetStream) http.Handler
	return NewDLQHandler(js)
}

// ---------------------------------------------------------------------------
// TEST-008 DLQ republish
// ---------------------------------------------------------------------------

func TestDLQ_Republish_Manual(t *testing.T) {
	// Covers [SPEC-001: AC-008, FR-009, BR-010] TEST-008

	t.Run("POST /internal/dlq/republish republishes 2 msgs from DLQ to TELEMETRY and returns count", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{
			dlqMessages: [][]byte{
				validDLQPayload("GTP890", "550e8400-e29b-41d4-a716-446655440010"),
				validDLQPayload("TTY423", "550e8400-e29b-41d4-a716-446655440011"),
			},
		}
		h := buildDLQHandler(js)
		body := bytes.NewReader([]byte(`{"limit": 10}`))
		req := httptest.NewRequest(http.MethodPost, "/internal/dlq/republish", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected json response, got %q err %v", rec.Body.String(), err)
		}
		if int(resp["republished"].(float64)) != 2 {
			t.Fatalf("expected republished 2, got %v body %s", resp["republished"], rec.Body.String())
		}
		if len(js.republished) != 2 {
			t.Fatalf("expected 2 republished via JetStream, got %d", len(js.republished))
		}
		if len(js.dlqMessages) != 0 {
			t.Fatalf("expected DLQ drained after republish, remaining %d", len(js.dlqMessages))
		}
	})

	t.Run("POST /internal/dlq/republish empty DLQ returns 0 without error", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{
			dlqMessages: [][]byte{},
		}
		h := buildDLQHandler(js)
		req := httptest.NewRequest(http.MethodPost, "/internal/dlq/republish", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for empty DLQ, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid json %q", rec.Body.String())
		}
		if int(resp["republished"].(float64)) != 0 {
			t.Fatalf("expected 0 republished, got %v", resp["republished"])
		}
		if len(js.republished) != 0 {
			t.Fatalf("expected 0 republished")
		}
	})

	t.Run("POST /internal/dlq/republish with limit 1 republishes only 1", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{
			dlqMessages: [][]byte{
				validDLQPayload("GTP890", "550e8400-e29b-41d4-a716-446655440020"),
				validDLQPayload("GTP890", "550e8400-e29b-41d4-a716-446655440021"),
				validDLQPayload("GTP890", "550e8400-e29b-41d4-a716-446655440022"),
			},
		}
		h := buildDLQHandler(js)
		body := bytes.NewReader([]byte(`{"limit":1}`))
		req := httptest.NewRequest(http.MethodPost, "/internal/dlq/republish", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if int(resp["republished"].(float64)) != 1 {
			t.Fatalf("expected 1, got %v", resp["republished"])
		}
		if len(js.republished) != 1 {
			t.Fatalf("expected 1 republished")
		}
		if len(js.dlqMessages) != 2 {
			t.Fatalf("expected 2 remaining in DLQ, got %d", len(js.dlqMessages))
		}
	})

	t.Run("GET /internal/dlq/republish -> 405 method not allowed", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{}
		h := buildDLQHandler(js)
		req := httptest.NewRequest(http.MethodGet, "/internal/dlq/republish", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(js.republished) != 0 {
			t.Fatalf("expected no republish on wrong method")
		}
	})

	t.Run("republish preserves original client_event_id and plate subject routing", func(t *testing.T) {
		// Arrange
		payload := validDLQPayload("GTP890", "550e8400-e29b-41d4-a716-446655440030")
		js := &fakeDLQJetStream{
			dlqMessages: [][]byte{payload},
		}
		h := buildDLQHandler(js)
		req := httptest.NewRequest(http.MethodPost, "/internal/dlq/republish", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if len(js.republished) != 1 {
			t.Fatalf("expected 1 republished")
		}
		if js.republished[0].subject != "telemetry.raw.GTP890" {
			t.Fatalf("expected subject telemetry.raw.GTP890, got %q", js.republished[0].subject)
		}
		var m map[string]any
		if err := json.Unmarshal(js.republished[0].data, &m); err != nil {
			t.Fatalf("republished data not json: %v", err)
		}
		if m["client_event_id"] != "550e8400-e29b-41d4-a716-446655440030" {
			t.Fatalf("expected client_event_id preserved, got %v", m["client_event_id"])
		}
		if m["plate"] != "GTP890" {
			t.Fatalf("expected plate preserved, got %v", m["plate"])
		}
	})

	t.Run("republish handler does not expose DLQ to public API without auth (internal only)", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{}
		h := buildDLQHandler(js)
		// Unknown path should 404, not expose DLQ via GET /dlq
		req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for unknown DLQ path, got %d", rec.Code)
		}
	})
}

func TestDLQ_Republish_Errors(t *testing.T) {
	// Covers [SPEC-001: AC-008, FR-009] TEST-008 resilience

	t.Run("JetStream republish error -> 500 without acking DLQ", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{
			dlqMessages: [][]byte{validDLQPayload("GTP890", "550e8400-e29b-41d4-a716-446655440040")},
			pubErr:      bytes.ErrTooLarge, // simulate publish failure
		}
		// Override to force error via injected pubErr; handler should surface 500 and not Ack DLQ
		h := buildDLQHandler(js)
		req := httptest.NewRequest(http.MethodPost, "/internal/dlq/republish", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on JetStream publish failure, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(js.dlqMessages) != 1 {
			t.Fatalf("DLQ should not be acked on publish failure, remaining %d", len(js.dlqMessages))
		}
	})

	t.Run("invalid JSON body -> 400", func(t *testing.T) {
		// Arrange
		js := &fakeDLQJetStream{}
		h := buildDLQHandler(js)
		req := httptest.NewRequest(http.MethodPost, "/internal/dlq/republish", bytes.NewReader([]byte(`{invalid`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid json, got %d", rec.Code)
		}
	})
}
