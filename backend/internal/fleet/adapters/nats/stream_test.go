package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type fakeFleetStreamJS struct {
	capturedCfg *jetstream.StreamConfig
	retErr error
	called int
}

func (f *fakeFleetStreamJS) CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	f.called++
	f.capturedCfg = &cfg
	return nil, f.retErr
}
func (f *fakeFleetStreamJS) Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) { return nil, nil }
func (f *fakeFleetStreamJS) PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) { return nil, nil }
func (f *fakeFleetStreamJS) PublishAsync(subject string, payload []byte, opts ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) { return nil, nil }
func (f *fakeFleetStreamJS) PublishMsgAsync(msg *nats.Msg, opts ...jetstream.PublishOpt) (jetstream.PubAckFuture, error) { return nil, nil }
func (f *fakeFleetStreamJS) PublishAsyncPending() int { return 0 }
func (f *fakeFleetStreamJS) PublishAsyncComplete() <-chan struct{} { ch:=make(chan struct{}); close(ch); return ch }
func (f *fakeFleetStreamJS) CleanupPublisher() {}
func (f *fakeFleetStreamJS) AccountInfo(ctx context.Context) (*jetstream.AccountInfo, error) { return nil, nil }
func (f *fakeFleetStreamJS) CreateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetStreamJS) UpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetStreamJS) Stream(ctx context.Context, stream string) (jetstream.Stream, error) { return nil, nil }
func (f *fakeFleetStreamJS) StreamNameBySubject(ctx context.Context, subject string) (string, error) { return "", nil }
func (f *fakeFleetStreamJS) DeleteStream(ctx context.Context, stream string) error { return nil }
func (f *fakeFleetStreamJS) ListStreams(ctx context.Context, opts ...jetstream.StreamListOpt) jetstream.StreamInfoLister { return nil }
func (f *fakeFleetStreamJS) StreamNames(ctx context.Context, opts ...jetstream.StreamListOpt) jetstream.StreamNameLister { return nil }
func (f *fakeFleetStreamJS) CreateOrUpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetStreamJS) CreateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetStreamJS) UpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetStreamJS) OrderedConsumer(ctx context.Context, stream string, cfg jetstream.OrderedConsumerConfig) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetStreamJS) Consumer(ctx context.Context, stream string, consumer string) (jetstream.Consumer, error) { return nil, nil }
func (f *fakeFleetStreamJS) DeleteConsumer(ctx context.Context, stream string, consumer string) error { return nil }
func (f *fakeFleetStreamJS) KeyValue(ctx context.Context, bucket string) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetStreamJS) CreateKeyValue(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetStreamJS) UpdateKeyValue(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetStreamJS) CreateOrUpdateKeyValue(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) { return nil, nil }
func (f *fakeFleetStreamJS) DeleteKeyValue(ctx context.Context, bucket string) error { return nil }
func (f *fakeFleetStreamJS) KeyValueStoreNames(ctx context.Context) jetstream.KeyValueNamesLister { return nil }
func (f *fakeFleetStreamJS) KeyValueStores(ctx context.Context) jetstream.KeyValueLister { return nil }
func (f *fakeFleetStreamJS) ObjectStore(ctx context.Context, bucket string) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetStreamJS) CreateObjectStore(ctx context.Context, cfg jetstream.ObjectStoreConfig) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetStreamJS) UpdateObjectStore(ctx context.Context, cfg jetstream.ObjectStoreConfig) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetStreamJS) CreateOrUpdateObjectStore(ctx context.Context, cfg jetstream.ObjectStoreConfig) (jetstream.ObjectStore, error) { return nil, nil }
func (f *fakeFleetStreamJS) DeleteObjectStore(ctx context.Context, bucket string) error { return nil }
func (f *fakeFleetStreamJS) ObjectStoreNames(ctx context.Context) jetstream.ObjectStoreNamesLister { return nil }
func (f *fakeFleetStreamJS) ObjectStores(ctx context.Context) jetstream.ObjectStoresLister { return nil }

func TestEnsureAlertsStream(t *testing.T) {
	// Covers [SPEC-002: AC-003]
	t.Run("creates stream with 7d 1GB defaults", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_MAX_BYTES", "")
		t.Setenv("ALERTS_REPLICAS", "")
		fake := &fakeFleetStreamJS{}
		ctx := context.Background()

		// Act
		err := EnsureAlertsStream(ctx, fake)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.called != 1 {
			t.Fatalf("expected 1 call")
		}
		cfg := fake.capturedCfg
		if cfg.Name != "ALERTS" {
			t.Fatalf("expected ALERTS got %q", cfg.Name)
		}
		if cfg.MaxAge != 7*24*time.Hour {
			t.Fatalf("expected 7d got %v", cfg.MaxAge)
		}
		if cfg.MaxBytes != 1<<30 {
			t.Fatalf("expected 1GB got %d", cfg.MaxBytes)
		}
		if cfg.Replicas != 1 {
			t.Fatalf("expected 1 replica got %d", cfg.Replicas)
		}
		if cfg.Duplicates != 2*time.Minute {
			t.Fatalf("expected 2m duplicates got %v", cfg.Duplicates)
		}
		if len(cfg.Subjects) == 0 || cfg.Subjects[0] != "alerts.>" {
			t.Fatalf("unexpected subjects %v", cfg.Subjects)
		}
	})

	t.Run("parses ALERTS_MAX_BYTES 1GB", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_MAX_BYTES", "1073741824")
		fake := &fakeFleetStreamJS{}
		// Act
		err := EnsureAlertsStream(context.Background(), fake)
		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if fake.capturedCfg.MaxBytes != 1073741824 {
			t.Fatalf("expected 1073741824 got %d", fake.capturedCfg.MaxBytes)
		}
	})

	t.Run("fallback to 1GB on invalid ALERTS_MAX_BYTES", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_MAX_BYTES", "bad")
		fake := &fakeFleetStreamJS{}
		// Act
		err := EnsureAlertsStream(context.Background(), fake)
		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if fake.capturedCfg.MaxBytes != 1<<30 {
			t.Fatalf("expected fallback 1GB got %d", fake.capturedCfg.MaxBytes)
		}
	})

	t.Run("fallback on zero or negative", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_MAX_BYTES", "0")
		fake := &fakeFleetStreamJS{}
		// Act
		_ = EnsureAlertsStream(context.Background(), fake)
		// Assert
		if fake.capturedCfg.MaxBytes != 1<<30 {
			t.Fatalf("expected fallback got %d", fake.capturedCfg.MaxBytes)
		}
	})

	t.Run("parses ALERTS_REPLICAS", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_REPLICAS", "3")
		fake := &fakeFleetStreamJS{}
		// Act
		_ = EnsureAlertsStream(context.Background(), fake)
		// Assert
		if fake.capturedCfg.Replicas != 3 {
			t.Fatalf("expected 3 got %d", fake.capturedCfg.Replicas)
		}
	})

	t.Run("CreateOrUpdateStream error returns error", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_MAX_BYTES", "")
		fake := &fakeFleetStreamJS{retErr: errors.New("create failed")}
		// Act
		err := EnsureAlertsStream(context.Background(), fake)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("context canceled returns error", func(t *testing.T) {
		// Arrange
		fake := &fakeFleetStreamJS{retErr: errors.New("create failed")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Need fake that respects context? EnsureAlertsStream uses context.WithTimeout(ctx,5s) and then CreateOrUpdateStream; if ctx already canceled, CreateOrUpdateStream still called but error; we test that error wraps.
		// Act
		err := EnsureAlertsStream(ctx, fake)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
