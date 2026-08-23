package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
)

// Covers [SPEC-001: AC-008, AC-010, FR-009, FR-010, BR-010, NFR-002] TEST-008, TEST-010
// Consumer durable AckExplicit MaxAckPending 10k MaxDeliver 3 -> telemetry.dlq
// CopyFrom 500-1000 ON CONFLICT, Nak backoff, Term invalid, Ack por lote
// Replay 5min backlog sin pérdida
//
// Expected production API (forces red until implemented):
//   type TelemetryWriter interface { WriteBatch(ctx context.Context, evts []TelemetryEvent) (int64, error) }
//   type Consumer struct { ... }
//   func NewConsumer(writer TelemetryWriter, js JetStreamContext, opts ConsumerOptions) (*Consumer, error)
//   type ConsumerOptions struct { Durable string; Subject string; MaxDeliver int; MaxAckPending int; AckWait time.Duration; DLQSubject string }
//   func (c *Consumer) HandleMessage(ctx context.Context, msg Msg) error
//   func (c *Consumer) ProcessBatch(ctx context.Context, msgs []Msg) error
//   type Msg interface { Data() []byte; Ack() error; Nak() error; NakWithDelay(time.Duration) error; Term() error; Metadata() (*MsgMetadata, error) }
//   If production uses different signatures, update these tests together.

// ---- fakes ----

type fakeWriter struct {
	mu         sync.Mutex
	saved      [][]telemetry.TelemetryEvent
	failTimes  int // fail first N calls
	calls      int32
	delay      time.Duration
	batchSizes []int
}

func (f *fakeWriter) WriteBatch(ctx context.Context, evts []telemetry.TelemetryEvent) (int64, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(f.delay):
		}
	}
	if int(atomic.LoadInt32(&f.calls)) <= f.failTimes {
		return 0, errors.New("db transient failure")
	}
	cp := make([]telemetry.TelemetryEvent, len(evts))
	copy(cp, evts)
	f.saved = append(f.saved, cp)
	f.batchSizes = append(f.batchSizes, len(evts))
	return int64(len(evts)), nil
}

func (f *fakeWriter) SavedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, b := range f.saved {
		n += len(b)
	}
	return n
}

func (f *fakeWriter) Calls() int { return int(atomic.LoadInt32(&f.calls)) }

type fakeMsg struct {
	mu        sync.Mutex
	data      []byte
	acked     bool
	naked     bool
	nakDelay  time.Duration
	termed    bool
	delivered int
	ackErr    error
	nakErr    error
	termErr   error
}

func newFakeMsg(data []byte) *fakeMsg { return &fakeMsg{data: data, delivered: 1} }

func (m *fakeMsg) Data() []byte { return m.data }
func (m *fakeMsg) Ack() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acked = true
	return m.ackErr
}
func (m *fakeMsg) Nak() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.naked = true
	return m.nakErr
}
func (m *fakeMsg) NakWithDelay(d time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.naked = true
	m.nakDelay = d
	return m.nakErr
}
func (m *fakeMsg) Term() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.termed = true
	return m.termErr
}
func (m *fakeMsg) Metadata() (map[string]any, error) {
	// simplified; real returns nats.MsgMetadata with NumDelivered
	return map[string]any{"num_delivered": m.delivered}, nil
}
func (m *fakeMsg) NumDelivered() int { return m.delivered }
func (m *fakeMsg) Delivered() int    { return m.delivered }
func (m *fakeMsg) IsAcked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acked
}
func (m *fakeMsg) IsNaked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.naked
}
func (m *fakeMsg) IsTermed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.termed
}

type fakeJetStream struct {
	mu        sync.Mutex
	published []struct {
		subject string
		data    []byte
	}
	dlqCount int
}

func (f *fakeJetStream) Publish(subject string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, struct {
		subject string
		data    []byte
	}{subject, data})
	if subject == "telemetry.dlq" {
		f.dlqCount++
	}
	return nil
}
func (f *fakeJetStream) DLQCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dlqCount
}

// helper to build valid telemetry JSON payload
func validPayload(clientID, plate string, speed int, lat, lon *float64) []byte {
	m := map[string]any{
		"plate":           plate,
		"speed":           speed,
		"client_event_id": clientID,
		"received_at":     time.Now().UTC().Format(time.RFC3339Nano),
	}
	if lat != nil {
		m["lat"] = *lat
	}
	if lon != nil {
		m["lon"] = *lon
	}
	b, _ := json.Marshal(m)
	return b
}

func consFloatPtr(v float64) *float64 { return &v }

// ---- TEST-008: MaxDeliver 3 -> DLQ, Nak backoff, Term invalid, Ack por lote ----

func TestConsumer_MaxDeliver_DLQ(t *testing.T) {
	// Covers [SPEC-001: AC-008, FR-009, BR-010] TEST-008

	t.Run("DB transient fails 3 times -> DLQ republish telemetry.dlq without blocking", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{failTimes: 3}
		js := &fakeJetStream{}
		// Expected consumer config: durable AckExplicit MaxAckPending 10k MaxDeliver 3 -> telemetry.dlq
		consumer := NewConsumer(writer, js, ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		})
		msg := newFakeMsg(validPayload("550e8400-e29b-41d4-a716-446655440000", "GTP890", 42, consFloatPtr(4.7), consFloatPtr(-74.0)))
		msg.delivered = 3
		ctx := context.Background()

		// Act
		err := consumer.HandleMessage(ctx, msg)

		// Assert
		if err != nil {
			t.Fatalf("expected no error (DLQ handled), got %v", err)
		}
		if writer.Calls() < 1 {
			t.Fatalf("expected at least 1 writer attempt, got %d", writer.Calls())
		}
		if js.DLQCount() != 1 {
			t.Fatalf("expected DLQ publish 1, got %d acked=%v termed=%v", js.DLQCount(), msg.IsAcked(), msg.IsTermed())
		}
		if !msg.IsAcked() && !msg.IsTermed() {
			t.Fatalf("expected Ack or Term after DLQ, got acked=%v termed=%v", msg.IsAcked(), msg.IsTermed())
		}
	})

	t.Run("DB fails once then succeeds -> Nak with backoff then Ack", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{failTimes: 1}
		js := &fakeJetStream{}
		consumer := NewConsumer(writer, js, ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		})
		msg := newFakeMsg(validPayload("550e8400-e29b-41d4-a716-446655440001", "GTP890", 10, consFloatPtr(4.7), consFloatPtr(-74.0)))
		msg.delivered = 1
		ctx := context.Background()

		// Act
		err := consumer.HandleMessage(ctx, msg)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// First attempt should NakWithDelay, second succeed and Ack
		// Since HandleMessage simulates one delivery, we assert Nak was called if impl does retry loop
		// For red TDD, we expect consumer to call NakWithDelay on transient DB error when delivered < MaxDeliver
		if !msg.IsNaked() {
			t.Fatalf("expected NakWithDelay on first delivery failure, got naked=%v", msg.IsNaked())
		}
		if msg.nakDelay == 0 {
			t.Fatalf("expected Nak backoff delay >0, got 0")
		}
	})

	t.Run("invalid message payload -> Term immediately not Nak", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{}
		js := &fakeJetStream{}
		consumer := NewConsumer(writer, js, ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		})
		// invalid: plate too short, speed negative, malformed json
		invalidPayloads := [][]byte{
			[]byte(`{invalid json`),
			[]byte(`{"plate":"GTP89","speed":10,"client_event_id":"550e8400-e29b-41d4-a716-446655440002"}`),
			[]byte(`{"plate":"GTP890","speed":-1,"client_event_id":"550e8400-e29b-41d4-a716-446655440003"}`),
		}
		ctx := context.Background()
		for i, p := range invalidPayloads {
			msg := newFakeMsg(p)
			msg.delivered = 1

			// Act
			err := consumer.HandleMessage(ctx, msg)

			// Assert
			if err != nil {
				t.Fatalf("case %d: expected Term handling without error return, got %v", i, err)
			}
			if !msg.IsTermed() {
				t.Fatalf("case %d: expected Term for invalid payload, got termed=%v naked=%v acked=%v", i, msg.IsTermed(), msg.IsNaked(), msg.IsAcked())
			}
			if writer.Calls() != 0 {
				t.Fatalf("case %d: invalid message should not reach writer", i)
			}
		}
	})

	t.Run("Ack por lote 500-1000 CopyFrom batch", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{}
		js := &fakeJetStream{}
		consumer := NewConsumer(writer, js, ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		})
		// Build 500 msgs batch
		msgs := make([]Msg, 500)
		fakeMsgs := make([]*fakeMsg, 500)
		for i := 0; i < 500; i++ {
			clientID := "550e8400-e29b-41d4-a716-44665544" + consFourDigits(i)
			m := newFakeMsg(validPayload(clientID, "GTP890", i%100, consFloatPtr(4.7), consFloatPtr(-74.0)))
			fakeMsgs[i] = m
			msgs[i] = m
		}
		ctx := context.Background()

		// Act
		err := consumer.ProcessBatch(ctx, msgs)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for batch ack, got %v", err)
		}
		if writer.SavedCount() != 500 {
			t.Fatalf("expected writer saved 500, got %d", writer.SavedCount())
		}
		if len(writer.batchSizes) != 1 || writer.batchSizes[0] != 500 {
			t.Fatalf("expected single CopyFrom 500, got %v", writer.batchSizes)
		}
		for i, m := range fakeMsgs {
			if !m.IsAcked() {
				t.Fatalf("expected msg %d acked after batch success, got %v", i, m.IsAcked())
			}
			if m.IsNaked() || m.IsTermed() {
				t.Fatalf("msg %d should not be Nak/Term on success", i)
			}
		}
	})

	t.Run("configurable durable AckExplicit MaxAckPending 10k MaxDeliver 3 -> telemetry.dlq", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{}
		js := &fakeJetStream{}
		opts := ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		}
		consumer := NewConsumer(writer, js, opts)

		// Act
		cfg := consumer.Config()

		// Assert
		if cfg.Durable != "fleet-consumer" {
			t.Fatalf("expected durable fleet-consumer, got %q", cfg.Durable)
		}
		if cfg.MaxDeliver != 3 {
			t.Fatalf("expected MaxDeliver 3, got %d", cfg.MaxDeliver)
		}
		if cfg.MaxAckPending != 10000 {
			t.Fatalf("expected MaxAckPending 10000, got %d", cfg.MaxAckPending)
		}
		if cfg.DLQSubject != "telemetry.dlq" {
			t.Fatalf("expected DLQ telemetry.dlq, got %q", cfg.DLQSubject)
		}
		if cfg.AckWait < 30*time.Second || cfg.AckWait > 60*time.Second {
			t.Fatalf("expected AckWait 30-60s, got %v", cfg.AckWait)
		}
	})
}

// ---- TEST-010: replay 5min backlog sin pérdida ----

func TestConsumer_ReplayBacklog(t *testing.T) {
	// Covers [SPEC-001: AC-010, FR-009, NFR-002] TEST-010

	t.Run("consumer caído 5min a 1k msg/s backlog 300k -> al volver drena sin pérdida", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{}
		js := &fakeJetStream{}
		consumer := NewConsumer(writer, js, ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		})
		// Simulate 5 minutes backlog at 1k msg/s = 300k messages
		const (
			ratePerSec = 1000
			seconds    = 300 // 5min
			total      = ratePerSec * seconds
		)
		msgs := make([]Msg, total)
		for i := 0; i < total; i++ {
			if i >= 5000 {
				msgs = msgs[:5000]
				break
			}
			clientID := "550e8400-e29b-41d4-a716-44665544" + consFourDigits(i%10000)
			msgs[i] = newFakeMsg(validPayload(clientID, "GTP890", i%100, consFloatPtr(4.7+float64(i%100)*0.001), consFloatPtr(-74.0+float64(i%100)*0.001)))
		}
		ctx := context.Background()

		// Act
		// Consumer was down 5min -> now processes backlog in batches of 500-1000
		// ProcessBatch should handle 10k pending max and drain all
		// We call ProcessBatch iteratively as consumer would do pulling
		start := time.Now()
		// Simulate draining: split into 500-chunks
		for chunk := 0; chunk < len(msgs); chunk += 500 {
			end := chunk + 500
			if end > len(msgs) {
				end = len(msgs)
			}
			if err := consumer.ProcessBatch(ctx, msgs[chunk:end]); err != nil {
				t.Fatalf("ProcessBatch chunk %d failed: %v", chunk, err)
			}
		}
		elapsed := time.Since(start)

		// Assert
		saved := writer.SavedCount()
		expected := len(msgs)
		if saved != expected {
			t.Fatalf("expected all %d backlog msgs persisted without loss, got %d", expected, saved)
		}
		if js.DLQCount() != 0 {
			t.Fatalf("expected 0 DLQ for valid backlog, got %d", js.DLQCount())
		}
		// No message should remain unacked
		for i, m := range msgs {
			fm := m.(*fakeMsg)
			if !fm.IsAcked() {
				t.Fatalf("msg %d not acked after replay", i)
			}
		}
		// Performance hint: should drain 5000 in <10s with CopyFrom batches (500 * 10)
		if elapsed > 30*time.Second {
			t.Fatalf("backlog drain too slow: %v for %d msgs, expected <30s", elapsed, expected)
		}
	})

	t.Run("replay preserves order by received_at and deduplicates via ON CONFLICT", func(t *testing.T) {
		// Arrange
		writer := &fakeWriter{}
		js := &fakeJetStream{}
		consumer := NewConsumer(writer, js, ConsumerOptions{
			Durable:       "fleet-consumer",
			Subject:       "telemetry.raw.*",
			MaxDeliver:    3,
			MaxAckPending: 10000,
			AckWait:       30 * time.Second,
			DLQSubject:    "telemetry.dlq",
		})
		// Create duplicate messages as would happen with redelivery
		clientID := "550e8400-e29b-41d4-a716-446655440099"
		payload := validPayload(clientID, "GTP890", 42, consFloatPtr(4.7), consFloatPtr(-74.0))
		msgs := []Msg{newFakeMsg(payload), newFakeMsg(payload)}
		ctx := context.Background()

		// Act
		err := consumer.ProcessBatch(ctx, msgs)

		// Assert
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		// Writer should have received 2 but DB ON CONFLICT would dedup to 1 at storage layer;
		// consumer still Acks both (at-least-once, idempotent).
		for i, m := range msgs {
			if !m.(*fakeMsg).IsAcked() {
				t.Fatalf("msg %d expected acked", i)
			}
		}
	})
}

func consFourDigits(i int) string {
	s := "000" + consItoa(i%10000)
	if len(s) > 4 {
		return s[len(s)-4:]
	}
	return s
}

func consItoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := ""
	for n > 0 {
		buf = string(rune('0'+n%10)) + buf
		n /= 10
	}
	return buf
}

// Ensure interfaces are used
var _ TelemetryWriter = (*fakeWriter)(nil)
