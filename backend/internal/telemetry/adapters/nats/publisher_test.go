package nats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
	"fleetmonitoring/backend/internal/telemetry/domain"
	"github.com/nats-io/nats.go"
)

type fakeBreakerRec struct {
	successes int
	failures  int
}

func (f *fakeBreakerRec) RecordSuccess() { f.successes++ }
func (f *fakeBreakerRec) RecordFailure() { f.failures++ }

type fakeFuture struct {
	okCh  chan *nats.PubAck
	errCh chan error
	msg   *nats.Msg
}

func (f *fakeFuture) Ok() <-chan *nats.PubAck { return f.okCh }
func (f *fakeFuture) Err() <-chan error       { return f.errCh }
func (f *fakeFuture) Msg() *nats.Msg          { return f.msg }

func newSuccessFuture() *fakeFuture {
	ok := make(chan *nats.PubAck, 1)
	ok <- &nats.PubAck{Stream: "TELEMETRY", Sequence: 1}
	errCh := make(chan error, 1)
	return &fakeFuture{okCh: ok, errCh: errCh}
}

func newNilAckFuture() *fakeFuture {
	ok := make(chan *nats.PubAck, 1)
	ok <- nil
	errCh := make(chan error, 1)
	return &fakeFuture{okCh: ok, errCh: errCh}
}

func newErrFuture(err error) *fakeFuture {
	ok := make(chan *nats.PubAck, 1)
	errCh := make(chan error, 1)
	errCh <- err
	return &fakeFuture{okCh: ok, errCh: errCh}
}

func newPendingFuture() *fakeFuture {
	// never sends, blocks until timeout
	return &fakeFuture{okCh: make(chan *nats.PubAck), errCh: make(chan error)}
}

type fakeJS struct {
	publishAsyncErr error
	publishAsyncFn func(subject string, data []byte, opts ...nats.PubOpt) (nats.PubAckFuture, error)
	completeCh chan struct{}
	capturedSubject string
	capturedData []byte
	capturedMsgId string
	publishAsyncCalls int
}

func (f *fakeJS) PublishAsync(subject string, data []byte, opts ...nats.PubOpt) (nats.PubAckFuture, error) {
	f.publishAsyncCalls++
	f.capturedSubject = subject
	f.capturedData = data
	// extract MsgId from opts by using a Msg and applying opts
	if len(opts) > 0 {
		msg := nats.NewMsg(subject)
		msg.Data = data
		for _, o := range opts {
			_ = o
			// PubOpt is interface configurePublish; easiest check data contains client_event_id later
			// We capture raw header via building msg header manually not needed for test assertion beyond data
		}
	}
	if f.publishAsyncFn != nil {
		return f.publishAsyncFn(subject, data, opts...)
	}
	if f.publishAsyncErr != nil {
		return nil, f.publishAsyncErr
	}
	return newSuccessFuture(), nil
}
func (f *fakeJS) PublishAsyncComplete() <-chan struct{} {
	if f.completeCh != nil {
		return f.completeCh
	}
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (f *fakeJS) Publish(string, []byte, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeJS) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeJS) PublishMsgAsync(*nats.Msg, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeJS) PublishAsyncPending() int { return 0 }
func (f *fakeJS) CleanupPublisher() {}
func (f *fakeJS) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) SubscribeSync(string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) ChanSubscribe(string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) ChanQueueSubscribe(string, string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) QueueSubscribe(string, string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) QueueSubscribeSync(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) PullSubscribe(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJS) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeJS) UpdateStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeJS) DeleteStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeJS) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeJS) PurgeStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeJS) StreamsInfo(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeJS) Streams(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeJS) StreamNames(...nats.JSOpt) <-chan string { return nil }
func (f *fakeJS) GetMsg(string, uint64, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeJS) GetLastMsg(string, string, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeJS) DeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeJS) SecureDeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeJS) AddConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeJS) UpdateConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeJS) DeleteConsumer(string, string, ...nats.JSOpt) error { return nil }
func (f *fakeJS) ConsumerInfo(string, string, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeJS) ConsumersInfo(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeJS) Consumers(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeJS) ConsumerNames(string, ...nats.JSOpt) <-chan string { return nil }
func (f *fakeJS) AccountInfo(...nats.JSOpt) (*nats.AccountInfo, error) { return nil, nil }
func (f *fakeJS) StreamNameBySubject(string, ...nats.JSOpt) (string, error) { return "", nil }
func (f *fakeJS) KeyValue(string) (nats.KeyValue, error) { return nil, nil }
func (f *fakeJS) CreateKeyValue(*nats.KeyValueConfig) (nats.KeyValue, error) { return nil, nil }
func (f *fakeJS) DeleteKeyValue(string) error { return nil }
func (f *fakeJS) KeyValueStoreNames() <-chan string { return nil }
func (f *fakeJS) KeyValueStores() <-chan nats.KeyValueStatus { return nil }
func (f *fakeJS) ObjectStore(string) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeJS) CreateObjectStore(*nats.ObjectStoreConfig) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeJS) DeleteObjectStore(string) error { return nil }
func (f *fakeJS) ObjectStoreNames(...nats.ObjectOpt) <-chan string { return nil }
func (f *fakeJS) ObjectStores(...nats.ObjectOpt) <-chan nats.ObjectStoreStatus { return nil }

func validTelemetryEvent() domain.TelemetryEvent {
	now := time.Now().UTC()
	lat := 4.711
	lon := -74.072
	return domain.TelemetryEvent{
		ClientEventID: "550e8400-e29b-41d4-a716-446655440000",
		Plate:         "ABC123",
		Speed:         42,
		Lat:           &lat,
		Lon:           &lon,
		ReceivedAt:    now,
	}
}

func TestPublisher_Publish(t *testing.T) {
	// Covers [SPEC-001: AC-001, FR-001, BR-006]
	t.Run("success records success", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{}
		brk := &fakeBreakerRec{}
		pub := NewPublisherWithBreaker(fake, 3*time.Second, brk)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if brk.successes != 1 {
			t.Fatalf("expected 1 success, got %d", brk.successes)
		}
		if brk.failures != 0 {
			t.Fatalf("expected 0 failures")
		}
		if !strings.Contains(fake.capturedSubject, "telemetry.raw.ABC123") {
			t.Fatalf("expected subject telemetry.raw.ABC123 got %q", fake.capturedSubject)
		}
	})

	t.Run("PublishAsync backpressure string max_pending wraps ErrBackpressure", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncErr: errors.New("max_pending exceeded")}
		brk := &fakeBreakerRec{}
		pub := NewPublisherWithBreaker(fake, 3*time.Second, brk)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected ErrBackpressure wrapped, got %v", err)
		}
		if brk.failures != 1 {
			t.Fatalf("expected failure recorded")
		}
	})

	t.Run("isBackpressure case insensitive BACKPRESSURE", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncErr: errors.New("BACKPRESSURE detected")}
		pub := NewPublisher(fake, 3*time.Second)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure case insensitive, got %v", err)
		}
	})

	t.Run("timeout returns ErrBackpressure", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncFn: func(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) {
			return newPendingFuture(), nil
		}}
		pub := NewPublisher(fake, 30*time.Millisecond)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if err == nil {
			t.Fatalf("expected timeout error")
		}
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected ErrBackpressure, got %v", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "timeout") {
			t.Fatalf("expected timeout message, got %v", err)
		}
	})

	t.Run("ctx canceled returns ErrBackpressure", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncFn: func(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) {
			return newPendingFuture(), nil
		}}
		pub := NewPublisher(fake, 3*time.Second)
		evt := validTelemetryEvent()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Act
		err := pub.Publish(ctx, evt)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure, got %v", err)
		}
	})

	t.Run("ack nil returns backpressure", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncFn: func(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) {
			return newNilAckFuture(), nil
		}}
		pub := NewPublisher(fake, 3*time.Second)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if err == nil {
			t.Fatalf("expected error for nil ack")
		}
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure, got %v", err)
		}
	})

	t.Run("ack error backpressure wraps", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncFn: func(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) {
			return newErrFuture(errors.New("too many requests")), nil
		}}
		pub := NewPublisher(fake, 3*time.Second)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure, got %v", err)
		}
	})

	t.Run("ack error non-backpressure returns plain error", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncFn: func(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) {
			return newErrFuture(errors.New("some nats error")), nil
		}}
		pub := NewPublisher(fake, 3*time.Second)
		evt := validTelemetryEvent()

		// Act
		err := pub.Publish(context.Background(), evt)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected not backpressure")
		}
	})

	t.Run("NewPublisher defaults timeout to 3s", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{}
		// Act
		pub := NewPublisher(fake, 0)
		// Assert
		if pub.timeout != 3*time.Second {
			t.Fatalf("expected 3s got %v", pub.timeout)
		}
	})
}

func TestPublisher_PublishBatch(t *testing.T) {
	// Covers [SPEC-001: AC-002, FR-002]
	t.Run("empty returns ErrValidation", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{}
		pub := NewPublisher(fake, 3*time.Second)

		// Act
		err := pub.PublishBatch(context.Background(), []domain.TelemetryEvent{})

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, application.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("success batch records success", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{}
		brk := &fakeBreakerRec{}
		pub := NewPublisherWithBreaker(fake, 3*time.Second, brk)
		evts := []domain.TelemetryEvent{validTelemetryEvent(), validTelemetryEvent()}

		// Act
		err := pub.PublishBatch(context.Background(), evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if brk.successes != 1 {
			t.Fatalf("expected success recorded, got %d", brk.successes)
		}
	})

	t.Run("batch PublishAsync backpressure wraps", func(t *testing.T) {
		// Arrange
		fake := &fakeJS{publishAsyncErr: errors.New("max_pending limit")}
		pub := NewPublisher(fake, 3*time.Second)
		evts := []domain.TelemetryEvent{validTelemetryEvent()}

		// Act
		err := pub.PublishBatch(context.Background(), evts)

		// Assert
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure, got %v", err)
		}
	})

	t.Run("batch timeout returns backpressure", func(t *testing.T) {
		// Arrange
		ch := make(chan struct{}) // never closed
		fake := &fakeJS{
			publishAsyncFn: func(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) {
				return newSuccessFuture(), nil
			},
			completeCh: ch,
		}
		pub := NewPublisher(fake, 30*time.Millisecond)
		evts := []domain.TelemetryEvent{validTelemetryEvent()}

		// Act
		err := pub.PublishBatch(context.Background(), evts)

		// Assert
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure timeout, got %v", err)
		}
	})

	t.Run("batch ctx canceled returns backpressure", func(t *testing.T) {
		// Arrange
		ch := make(chan struct{})
		fake := &fakeJS{completeCh: ch}
		pub := NewPublisher(fake, 3*time.Second)
		evts := []domain.TelemetryEvent{validTelemetryEvent()}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Act
		err := pub.PublishBatch(ctx, evts)

		// Assert
		if !errors.Is(err, application.ErrBackpressure) {
			t.Fatalf("expected backpressure, got %v", err)
		}
	})
}

func TestMarshalEvent(t *testing.T) {
	// Covers [SPEC-001: FR-003, FR-005]
	t.Run("produces json with required fields", func(t *testing.T) {
		// Arrange
		evt := validTelemetryEvent()

		// Act
		data, err := marshalEvent(evt)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if m["plate"] != "ABC123" {
			t.Fatalf("expected plate ABC123 got %v", m["plate"])
		}
		if m["client_event_id"] != evt.ClientEventID {
			t.Fatalf("expected client_event_id")
		}
		if m["speed"] == nil {
			t.Fatalf("expected speed")
		}
		if m["lat"] == nil || m["lon"] == nil {
			t.Fatalf("expected lat/lon")
		}
		if m["received_at"] == nil {
			t.Fatalf("expected received_at")
		}
	})

	t.Run("includes occurred_at when present", func(t *testing.T) {
		// Arrange
		evt := validTelemetryEvent()
		now := time.Now().UTC()
		evt.OccurredAt = &now

		// Act
		data, err := marshalEvent(evt)

		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		if m["occurred_at"] == nil {
			t.Fatalf("expected occurred_at")
		}
	})

	t.Run("omits occurred_at when nil", func(t *testing.T) {
		// Arrange
		evt := validTelemetryEvent()
		evt.OccurredAt = nil

		// Act
		data, err := marshalEvent(evt)

		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		if m["occurred_at"] != nil {
			t.Fatalf("expected no occurred_at")
		}
	})
}

func TestIsBackpressure(t *testing.T) {
	// Covers [SPEC-001: BR-006]
	cases := []struct {
		name string
		err  string
		want bool
	}{
		{"backpressure lower", "backpressure", true},
		{"BACKPRESSURE upper", "BACKPRESSURE", true},
		{"BackPressure mixed", "BackPressure", true},
		{"max_pending", "max_pending exceeded", true},
		{"MAX_PENDING upper", "MAX_PENDING", true},
		{"max pending space", "max pending", true},
		{"pending exceeded", "pending exceeded", true},
		{"too many", "too many requests", true},
		{"TOO MANY upper", "TOO MANY", true},
		{"other error", "some other error", false},
		{"nil", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			var err error
			if c.err != "" {
				err = errors.New(c.err)
			}
			// Act
			got := isBackpressure(err)
			// Assert
			if got != c.want {
				t.Fatalf("expected %v for %q got %v", c.want, c.err, got)
			}
		})
	}

	t.Run("nil returns false", func(t *testing.T) {
		// Arrange
		// Act
		got := isBackpressure(nil)
		// Assert
		if got {
			t.Fatalf("expected false")
		}
	})
}
