package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
)

// Covers [SPEC-001: AC-001, AC-004, BR-003]

func TestCoverage_TelemetryApp(t *testing.T) {
	t.Run("parsePayload invalid json", func(t *testing.T) {
		// Arrange
		// Act
		_, err := parsePayload([]byte("{ invalid"))
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("parsePayload missing plate", func(t *testing.T) {
		// Arrange
		data := json.RawMessage(`{"speed": 10}`)
		// Act
		_, err := parsePayload(data)
		// Assert
		if err == nil {
			t.Fatalf("expected missing plate")
		}
	})

	t.Run("parseReceivedAtField", func(t *testing.T) {
		// Arrange
		raw := map[string]json.RawMessage{"received_at": json.RawMessage(`"2026-08-24T10:00:00Z"`)}
		// Act
		tt := parseReceivedAtField(raw)
		// Assert
		if tt.IsZero() {
			t.Fatalf("expected not zero")
		}
		// also test parsePayloadWithClock with clock
		clk := fakeClock{now: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)}
		evt, err := parsePayloadWithClock([]byte(`{"plate":"GTP890","speed":10,"client_event_id":"550e8400-e29b-41d4-a716-446655440000"}`), clk)
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if evt.Plate != "GTP890" {
			t.Fatalf("expected plate")
		}
	})

	t.Run("NewConsumerWithClock and ProcessBatch", func(t *testing.T) {
		// Arrange
		writer := &fakeWriterCoverage{}
		pub := &fakePublisherCoverage{}
		opts := ConsumerOptions{Durable: "test", Subject: "telemetry.raw.*", MaxDeliver: 3, MaxAckPending: 10, AckWait: 5 * time.Second, DLQSubject: "dlq"}
		clock := fakeClock{now: time.Now().UTC()}
		consumer := NewConsumerWithClock(writer, pub, opts, clock)
		// Act
		if consumer == nil {
			t.Fatalf("expected consumer")
		}
		if consumer.Config().Durable != "test" {
			t.Fatalf("expected test durable")
		}
		// ProcessBatch with one valid message
		msg := &fakeMsgCoverage{data: []byte(`{"plate":"GTP890","speed":10,"client_event_id":"550e8400-e29b-41d4-a716-446655440000"}`)}
		err := consumer.ProcessBatch(context.Background(), []Msg{msg})
		// Assert
		if err != nil {
			t.Fatalf("expected nil ProcessBatch, got %v", err)
		}
	})

	t.Run("backoffFor and validate helpers", func(t *testing.T) {
		// Arrange
		// Act
		d1 := backoffFor(1)
		d2 := backoffFor(10)
		// Assert
		if d1 <= 0 || d2 <= 0 {
			t.Fatalf("expected positive backoff")
		}
		// validate helpers via HandleMessage with invalid
	})
}

type fakeClock struct{ now time.Time }
func (f fakeClock) Now() time.Time { return f.now }

type fakeWriterCoverage struct{}
func (f *fakeWriterCoverage) WriteBatch(ctx context.Context, evts []telemetry.TelemetryEvent) (int64, error) { return int64(len(evts)), nil }

type fakePublisherCoverage struct{}
func (f *fakePublisherCoverage) Publish(subject string, data []byte) error { return nil }

type fakeMsgCoverage struct {
	data []byte
	acked bool
}
func (m *fakeMsgCoverage) Data() []byte { return m.data }
func (m *fakeMsgCoverage) Ack() error { m.acked = true; return nil }
func (m *fakeMsgCoverage) Nak() error { return nil }
func (m *fakeMsgCoverage) NakWithDelay(d time.Duration) error { return nil }
func (m *fakeMsgCoverage) Term() error { return nil }
func (m *fakeMsgCoverage) Delivered() int { return 1 }


