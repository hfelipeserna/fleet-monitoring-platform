package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
)

type fakeInfraJS struct {
	streamInfo *nats.StreamInfo
	streamInfoErr error
	addErr error
	updateErr error
	addCfg *nats.StreamConfig
	updateCfg *nats.StreamConfig
	addCalls int
	updateCalls int
}

func (f *fakeInfraJS) StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	return f.streamInfo, f.streamInfoErr
}
func (f *fakeInfraJS) AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.addCalls++
	f.addCfg = cfg
	return nil, f.addErr
}
func (f *fakeInfraJS) UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.updateCalls++
	f.updateCfg = cfg
	return nil, f.updateErr
}
func (f *fakeInfraJS) Publish(string, []byte, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeInfraJS) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeInfraJS) PublishAsync(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeInfraJS) PublishMsgAsync(*nats.Msg, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeInfraJS) PublishAsyncPending() int { return 0 }
func (f *fakeInfraJS) PublishAsyncComplete() <-chan struct{} { return nil }
func (f *fakeInfraJS) CleanupPublisher() {}
func (f *fakeInfraJS) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) SubscribeSync(string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) ChanSubscribe(string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) ChanQueueSubscribe(string, string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) QueueSubscribe(string, string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) QueueSubscribeSync(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) PullSubscribe(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeInfraJS) DeleteStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeInfraJS) PurgeStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeInfraJS) StreamsInfo(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeInfraJS) Streams(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeInfraJS) StreamNames(...nats.JSOpt) <-chan string { return nil }
func (f *fakeInfraJS) GetMsg(string, uint64, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeInfraJS) GetLastMsg(string, string, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeInfraJS) DeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeInfraJS) SecureDeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeInfraJS) AddConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeInfraJS) UpdateConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeInfraJS) DeleteConsumer(string, string, ...nats.JSOpt) error { return nil }
func (f *fakeInfraJS) ConsumerInfo(string, string, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeInfraJS) ConsumersInfo(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeInfraJS) Consumers(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeInfraJS) ConsumerNames(string, ...nats.JSOpt) <-chan string { return nil }
func (f *fakeInfraJS) AccountInfo(...nats.JSOpt) (*nats.AccountInfo, error) { return nil, nil }
func (f *fakeInfraJS) StreamNameBySubject(string, ...nats.JSOpt) (string, error) { return "", nil }
func (f *fakeInfraJS) KeyValue(string) (nats.KeyValue, error) { return nil, nil }
func (f *fakeInfraJS) CreateKeyValue(*nats.KeyValueConfig) (nats.KeyValue, error) { return nil, nil }
func (f *fakeInfraJS) DeleteKeyValue(string) error { return nil }
func (f *fakeInfraJS) KeyValueStoreNames() <-chan string { return nil }
func (f *fakeInfraJS) KeyValueStores() <-chan nats.KeyValueStatus { return nil }
func (f *fakeInfraJS) ObjectStore(string) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeInfraJS) CreateObjectStore(*nats.ObjectStoreConfig) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeInfraJS) DeleteObjectStore(string) error { return nil }
func (f *fakeInfraJS) ObjectStoreNames(...nats.ObjectOpt) <-chan string { return nil }
func (f *fakeInfraJS) ObjectStores(...nats.ObjectOpt) <-chan nats.ObjectStoreStatus { return nil }

func TestEnsureStream_FleetInfra(t *testing.T) {
	// Covers [SPEC-002: AC-003]
	t.Run("creates ALERTS stream when not exists", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{streamInfoErr: errors.New("not found")}
		ctx := context.Background()

		// Act
		err := EnsureStream(ctx, fake)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.addCalls != 1 {
			t.Fatalf("expected AddStream 1")
		}
		if fake.addCfg.Name != "ALERTS" {
			t.Fatalf("expected ALERTS got %q", fake.addCfg.Name)
		}
		if fake.addCfg.MaxBytes != 1<<30 {
			t.Fatalf("expected 1GB got %d", fake.addCfg.MaxBytes)
		}
	})

	t.Run("updates existing stream with max MaxBytes", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{
			streamInfo: &nats.StreamInfo{Config: nats.StreamConfig{MaxBytes: 2 << 30}},
			streamInfoErr: nil,
		}
		// Act
		err := EnsureStream(context.Background(), fake)
		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if fake.updateCalls != 1 {
			t.Fatalf("expected UpdateStream")
		}
		if fake.updateCfg.MaxBytes != 2<<30 {
			t.Fatalf("expected max 2GB got %d", fake.updateCfg.MaxBytes)
		}
	})

	t.Run("context canceled returns error", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{streamInfoErr: errors.New("not found")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Act
		err := EnsureStream(ctx, fake)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("AddStream error returns error", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{streamInfoErr: errors.New("not found"), addErr: errors.New("add failed")}
		// Act
		err := EnsureStream(context.Background(), fake)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("UpdateStream error returns error", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{
			streamInfo: &nats.StreamInfo{Config: nats.StreamConfig{MaxBytes: 1 << 30}},
			updateErr: errors.New("update failed"),
		}
		// Act
		err := EnsureStream(context.Background(), fake)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestEnsureTelemetryStream_FleetInfra(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("creates TELEMETRY with 5GB default when zero", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{streamInfoErr: errors.New("not found")}
		// Act
		err := EnsureTelemetryStreamWithContext(context.Background(), fake, 0)
		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if fake.addCfg.MaxBytes != 5<<30 {
			t.Fatalf("expected 5GB got %d", fake.addCfg.MaxBytes)
		}
		if fake.addCfg.Name != "TELEMETRY" {
			t.Fatalf("expected TELEMETRY")
		}
	})

	t.Run("creates with custom maxBytes", func(t *testing.T) {
		// Arrange
		fake := &fakeInfraJS{streamInfoErr: errors.New("not found")}
		// Act
		err := EnsureTelemetryStreamWithContext(context.Background(), fake, 1<<30)
		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if fake.addCfg.MaxBytes != 1<<30 {
			t.Fatalf("expected 1GB")
		}
	})
}
