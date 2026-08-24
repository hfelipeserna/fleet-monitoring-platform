package nats

import (
	"errors"
	"testing"

	"fleetmonitoring/backend/internal/telemetry/application"
	natsio "github.com/nats-io/nats.go"
)

type fakeStreamJS struct {
	streamInfoErr error
	streamInfo *natsio.StreamInfo
	addStreamErr error
	addStreamCfg *natsio.StreamConfig
	consumerInfoErr error
	addConsumerErr error
	consumerCfg *natsio.ConsumerConfig
}

func (f *fakeStreamJS) StreamInfo(string, ...natsio.JSOpt) (*natsio.StreamInfo, error) {
	return f.streamInfo, f.streamInfoErr
}
func (f *fakeStreamJS) AddStream(cfg *natsio.StreamConfig, opts ...natsio.JSOpt) (*natsio.StreamInfo, error) {
	f.addStreamCfg = cfg
	return nil, f.addStreamErr
}
func (f *fakeStreamJS) ConsumerInfo(string, string, ...natsio.JSOpt) (*natsio.ConsumerInfo, error) {
	return nil, f.consumerInfoErr
}
func (f *fakeStreamJS) AddConsumer(stream string, cfg *natsio.ConsumerConfig, opts ...natsio.JSOpt) (*natsio.ConsumerInfo, error) {
	f.consumerCfg = cfg
	return nil, f.addConsumerErr
}

// stubs for rest of JetStreamContext
func (f *fakeStreamJS) Publish(string, []byte, ...natsio.PubOpt) (*natsio.PubAck, error) { return nil, nil }
func (f *fakeStreamJS) PublishMsg(*natsio.Msg, ...natsio.PubOpt) (*natsio.PubAck, error) { return nil, nil }
func (f *fakeStreamJS) PublishAsync(string, []byte, ...natsio.PubOpt) (natsio.PubAckFuture, error) { return nil, nil }
func (f *fakeStreamJS) PublishMsgAsync(*natsio.Msg, ...natsio.PubOpt) (natsio.PubAckFuture, error) { return nil, nil }
func (f *fakeStreamJS) PublishAsyncPending() int { return 0 }
func (f *fakeStreamJS) PublishAsyncComplete() <-chan struct{} { return nil }
func (f *fakeStreamJS) CleanupPublisher() {}
func (f *fakeStreamJS) Subscribe(string, natsio.MsgHandler, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) SubscribeSync(string, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) ChanSubscribe(string, chan *natsio.Msg, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) ChanQueueSubscribe(string, string, chan *natsio.Msg, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) QueueSubscribe(string, string, natsio.MsgHandler, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) QueueSubscribeSync(string, string, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) PullSubscribe(string, string, ...natsio.SubOpt) (*natsio.Subscription, error) { return nil, nil }
func (f *fakeStreamJS) UpdateStream(*natsio.StreamConfig, ...natsio.JSOpt) (*natsio.StreamInfo, error) { return nil, nil }
func (f *fakeStreamJS) DeleteStream(string, ...natsio.JSOpt) error { return nil }
func (f *fakeStreamJS) PurgeStream(string, ...natsio.JSOpt) error { return nil }
func (f *fakeStreamJS) StreamsInfo(...natsio.JSOpt) <-chan *natsio.StreamInfo { return nil }
func (f *fakeStreamJS) Streams(...natsio.JSOpt) <-chan *natsio.StreamInfo { return nil }
func (f *fakeStreamJS) StreamNames(...natsio.JSOpt) <-chan string { return nil }
func (f *fakeStreamJS) GetMsg(string, uint64, ...natsio.JSOpt) (*natsio.RawStreamMsg, error) { return nil, nil }
func (f *fakeStreamJS) GetLastMsg(string, string, ...natsio.JSOpt) (*natsio.RawStreamMsg, error) { return nil, nil }
func (f *fakeStreamJS) DeleteMsg(string, uint64, ...natsio.JSOpt) error { return nil }
func (f *fakeStreamJS) SecureDeleteMsg(string, uint64, ...natsio.JSOpt) error { return nil }
func (f *fakeStreamJS) UpdateConsumer(string, *natsio.ConsumerConfig, ...natsio.JSOpt) (*natsio.ConsumerInfo, error) { return nil, nil }
func (f *fakeStreamJS) DeleteConsumer(string, string, ...natsio.JSOpt) error { return nil }
func (f *fakeStreamJS) ConsumersInfo(string, ...natsio.JSOpt) <-chan *natsio.ConsumerInfo { return nil }
func (f *fakeStreamJS) Consumers(string, ...natsio.JSOpt) <-chan *natsio.ConsumerInfo { return nil }
func (f *fakeStreamJS) ConsumerNames(string, ...natsio.JSOpt) <-chan string { return nil }
func (f *fakeStreamJS) AccountInfo(...natsio.JSOpt) (*natsio.AccountInfo, error) { return nil, nil }
func (f *fakeStreamJS) StreamNameBySubject(string, ...natsio.JSOpt) (string, error) { return "", nil }
func (f *fakeStreamJS) KeyValue(string) (natsio.KeyValue, error) { return nil, nil }
func (f *fakeStreamJS) CreateKeyValue(*natsio.KeyValueConfig) (natsio.KeyValue, error) { return nil, nil }
func (f *fakeStreamJS) DeleteKeyValue(string) error { return nil }
func (f *fakeStreamJS) KeyValueStoreNames() <-chan string { return nil }
func (f *fakeStreamJS) KeyValueStores() <-chan natsio.KeyValueStatus { return nil }
func (f *fakeStreamJS) ObjectStore(string) (natsio.ObjectStore, error) { return nil, nil }
func (f *fakeStreamJS) CreateObjectStore(*natsio.ObjectStoreConfig) (natsio.ObjectStore, error) { return nil, nil }
func (f *fakeStreamJS) DeleteObjectStore(string) error { return nil }
func (f *fakeStreamJS) ObjectStoreNames(...natsio.ObjectOpt) <-chan string { return nil }
func (f *fakeStreamJS) ObjectStores(...natsio.ObjectOpt) <-chan natsio.ObjectStoreStatus { return nil }

func TestEnsureStream(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("existing stream does not call AddStream", func(t *testing.T) {
		// Arrange
		fake := &fakeStreamJS{streamInfo: &natsio.StreamInfo{}, streamInfoErr: nil}

		// Act
		err := EnsureStream(fake, 5<<30)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.addStreamCfg != nil {
			t.Fatalf("expected no AddStream")
		}
	})

	t.Run("missing stream creates with config", func(t *testing.T) {
		// Arrange
		fake := &fakeStreamJS{streamInfoErr: errors.New("not found")}

		// Act
		err := EnsureStream(fake, 5<<30)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.addStreamCfg == nil {
			t.Fatalf("expected AddStream called")
		}
		if fake.addStreamCfg.Name != "TELEMETRY" {
			t.Fatalf("expected TELEMETRY got %q", fake.addStreamCfg.Name)
		}
		if fake.addStreamCfg.MaxBytes != 5<<30 {
			t.Fatalf("expected maxBytes 5<<30 got %d", fake.addStreamCfg.MaxBytes)
		}
		if len(fake.addStreamCfg.Subjects) == 0 || fake.addStreamCfg.Subjects[0] != "telemetry.raw.>" {
			t.Fatalf("unexpected subjects %v", fake.addStreamCfg.Subjects)
		}
	})

	t.Run("add stream error returns error", func(t *testing.T) {
		// Arrange
		fake := &fakeStreamJS{streamInfoErr: errors.New("not found"), addStreamErr: errors.New("add failed")}

		// Act
		err := EnsureStream(fake, 5<<30)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestEnsureConsumer(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("existing consumer does nothing", func(t *testing.T) {
		// Arrange
		fake := &fakeStreamJS{consumerInfoErr: nil}

		// Act
		err := EnsureConsumer(fake, application.ConsumerOptions{Durable: "telemetry-workers"})

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("missing consumer creates with durable", func(t *testing.T) {
		// Arrange
		fake := &fakeStreamJS{consumerInfoErr: errors.New("not found")}

		// Act
		err := EnsureConsumer(fake, application.ConsumerOptions{Durable: "telemetry-workers", Subject: "telemetry.raw.>"})

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.consumerCfg == nil {
			t.Fatalf("expected AddConsumer called")
		}
		if fake.consumerCfg.Durable != "telemetry-workers" {
			t.Fatalf("expected durable telemetry-workers got %q", fake.consumerCfg.Durable)
		}
	})

	t.Run("add consumer error returns error", func(t *testing.T) {
		// Arrange
		fake := &fakeStreamJS{consumerInfoErr: errors.New("not found"), addConsumerErr: errors.New("add failed")}

		// Act
		err := EnsureConsumer(fake, application.ConsumerOptions{Durable: "x"})

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
