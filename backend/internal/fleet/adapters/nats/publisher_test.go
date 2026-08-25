package nats

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sony/gobreaker"
)

type fakeFleetBreaker struct {
	state gobreaker.State
	execErr error
	executed bool
}

func (f *fakeFleetBreaker) State() gobreaker.State { return f.state }
func (f *fakeFleetBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	f.executed = true
	if f.execErr != nil {
		return nil, f.execErr
	}
	return fn()
}

type fakeFleetJS struct {
	publishErr error
	capturedSubject string
	capturedPayload []byte
	publishCalled int
}

func (f *fakeFleetJS) Conn() *nats.Conn { return nil }
func (f *fakeFleetJS) Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	f.publishCalled++
	f.capturedSubject = subject
	f.capturedPayload = payload
	return &jetstream.PubAck{}, f.publishErr
}
func (f *fakeFleetJS) PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) { return nil, nil }
func (f *fakeFleetJS) PublishAsync(subject string, payload []byte, opts ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) { return nil, nil }
func (f *fakeFleetJS) PublishMsgAsync(msg *nats.Msg, opts ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) { return nil, nil }
func (f *fakeFleetJS) PublishAsyncPending() int { return 0 }
func (f *fakeFleetJS) PublishAsyncComplete() <-chan struct{} { ch := make(chan struct{}); close(ch); return ch }
func (f *fakeFleetJS) CleanupPublisher() {}
func (f *fakeFleetJS) AccountInfo(ctx context.Context) (*jetstream.AccountInfo, error) { return nil, nil }
func (f *fakeFleetJS) CreateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetJS) UpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetJS) CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetJS) Stream(ctx context.Context, stream string) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetJS) StreamNameBySubject(ctx context.Context, subject string) (string, error) { return "", nil }
func (f *fakeFleetJS) DeleteStream(ctx context.Context, stream string) error { return nil }
func (f *fakeFleetJS) ListStreams(ctx context.Context, opts ...jetstream.StreamListOpt) jetstream.StreamInfoLister { return nil }
func (f *fakeFleetJS) StreamNames(ctx context.Context, opts ...jetstream.StreamListOpt) jetstream.StreamNameLister { return nil }
func (f *fakeFleetJS) CreateOrUpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetJS) CreateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetJS) UpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetJS) OrderedConsumer(ctx context.Context, stream string, cfg jetstream.OrderedConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetJS) Consumer(ctx context.Context, stream string, consumer string) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetJS) DeleteConsumer(ctx context.Context, stream string, consumer string) error { return nil }
func (f *fakeFleetJS) KeyValue(ctx context.Context, bucket string) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetJS) CreateKeyValue(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetJS) UpdateKeyValue(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetJS) CreateOrUpdateKeyValue(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetJS) DeleteKeyValue(ctx context.Context, bucket string) error { return nil }
func (f *fakeFleetJS) KeyValueStoreNames(ctx context.Context) jetstream.KeyValueNamesLister { return nil }
func (f *fakeFleetJS) KeyValueStores(ctx context.Context) jetstream.KeyValueLister { return nil }
func (f *fakeFleetJS) ObjectStore(ctx context.Context, bucket string) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetJS) CreateObjectStore(ctx context.Context, cfg jetstream.ObjectStoreConfig) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetJS) UpdateObjectStore(ctx context.Context, cfg jetstream.ObjectStoreConfig) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetJS) CreateOrUpdateObjectStore(ctx context.Context, cfg jetstream.ObjectStoreConfig) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetJS) DeleteObjectStore(ctx context.Context, bucket string) error { return nil }
func (f *fakeFleetJS) ObjectStoreNames(ctx context.Context) jetstream.ObjectStoreNamesLister { return nil }
func (f *fakeFleetJS) ObjectStores(ctx context.Context) jetstream.ObjectStoresLister { return nil }

func validAlert() fleet.Alert {
	return fleet.Alert{
		EventID:   "550e8400-e29b-41d4-a716-446655440000",
		Plate:     "ABC123",
		AlertType: "speeding_on",
		Speed:     120,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestAlertPublisher_Publish(t *testing.T) {
	// Covers [SPEC-002: AC-003, BR-004]
	t.Run("success publishes to alerts.critical with MsgID", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetJS{}
		pub := NewAlertPublisher(fake)
		alert := validAlert()

		// Act
		err := pub.Publish(context.Background(), alert)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.publishCalled != 1 {
			t.Fatalf("expected 1 publish, got %d", fake.publishCalled)
		}
		if fake.capturedSubject != "alerts.critical" {
			t.Fatalf("expected alerts.critical got %q", fake.capturedSubject)
		}
		var got fleet.Alert
		if err := json.Unmarshal(fake.capturedPayload, &got); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if got.Plate != alert.Plate {
			t.Fatalf("expected plate %q got %q", alert.Plate, got.Plate)
		}
		expectedMsgID := alert.MsgID()
		if expectedMsgID == "" {
			t.Fatalf("expected MsgID not empty")
		}
	})

	t.Run("breaker open returns ErrOpenState", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetJS{}
		brk := &fakeFleetBreaker{state: gobreaker.StateOpen}
		pub := NewAlertPublisherWithBreaker(fake, brk, 3*time.Second)
		alert := validAlert()

		// Act
		err := pub.Publish(context.Background(), alert)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected ErrOpenState, got %v", err)
		}
		if fake.publishCalled != 0 {
			t.Fatalf("expected no publish when breaker open")
		}
	})

	t.Run("breaker Execute error wraps publish error", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetJS{publishErr: errors.New("nats down")}
		brk := &fakeFleetBreaker{state: gobreaker.StateClosed}
		pub := NewAlertPublisherWithBreaker(fake, brk, 3*time.Second)
		alert := validAlert()

		// Act
		err := pub.Publish(context.Background(), alert)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if fake.publishCalled != 1 {
			t.Fatalf("expected publish called")
		}
		if !brk.executed {
			t.Fatalf("expected breaker Execute called")
		}
	})

	t.Run("without breaker direct publish success", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetJS{}
		pub := NewAlertPublisherWithBreaker(fake, nil, 3*time.Second)
		alert := validAlert()

		// Act
		err := pub.Publish(context.Background(), alert)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("timeout zero defaults to 3s via NewAlertPublisherWithBreaker", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetJS{}
		pub := NewAlertPublisherWithBreaker(fake, nil, 0)

		// Act
		// Assert
		if pub.timeout != 3*time.Second {
			t.Fatalf("expected 3s got %v", pub.timeout)
		}
	})

	t.Run("zone alert MsgID includes zone bucket", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetJS{}
		pub := NewAlertPublisher(fake)
		zid := "11111111-1111-1111-1111-111111111111"
		alert := fleet.Alert{
			EventID:   "550e8400-e29b-41d4-a716-446655440001",
			Plate:     "XYZ789",
			AlertType: "zone_enter",
			ZoneID:    &zid,
			CreatedAt: time.Now().UTC(),
		}

		// Act
		err := pub.Publish(context.Background(), alert)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		expected := alert.MsgID()
		if expected == "" || len(expected) == 0 {
			t.Fatalf("expected MsgID")
		}
		// ensure publish payload contains zoneID
		var got fleet.Alert
		_ = json.Unmarshal(fake.capturedPayload, &got)
		if got.ZoneID == nil || *got.ZoneID != zid {
			t.Fatalf("expected zoneID preserved")
		}
	})
}
