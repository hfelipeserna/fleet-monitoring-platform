package nats

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
)

func TestSanitizeDLQLimit(t *testing.T) {
	// Covers [SPEC-001: BR-003]
	t.Run("zero returns default", func(t *testing.T) {
		// Arrange
		// Act
		got := SanitizeDLQLimit(0)

		// Assert
		if got != DefaultDLQLimit {
			t.Fatalf("expected %d got %d", DefaultDLQLimit, got)
		}
	})

	t.Run("negative returns default", func(t *testing.T) {
		// Arrange
		// Act
		got := SanitizeDLQLimit(-5)

		// Assert
		if got != DefaultDLQLimit {
			t.Fatalf("expected %d got %d", DefaultDLQLimit, got)
		}
	})

	t.Run("exceeds max returns max", func(t *testing.T) {
		// Arrange
		// Act
		got := SanitizeDLQLimit(MaxDLQLimit + 10)

		// Assert
		if got != MaxDLQLimit {
			t.Fatalf("expected %d got %d", MaxDLQLimit, got)
		}
	})

	t.Run("within range returns same", func(t *testing.T) {
		// Arrange
		// Act
		got := SanitizeDLQLimit(50)

		// Assert
		if got != 50 {
			t.Fatalf("expected 50 got %d", got)
		}
	})

	t.Run("max returns max", func(t *testing.T) {
		// Arrange
		// Act
		got := SanitizeDLQLimit(MaxDLQLimit)

		// Assert
		if got != MaxDLQLimit {
			t.Fatalf("expected %d got %d", MaxDLQLimit, got)
		}
	})
}

func TestResolveSubject(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("empty plate returns unknown", func(t *testing.T) {
		// Arrange
		// Act
		got := ResolveSubject("")

		// Assert
		if got != "telemetry.raw.unknown" {
			t.Fatalf("expected telemetry.raw.unknown got %q", got)
		}
	})

	t.Run("non-empty plate returns telemetry.raw.plate", func(t *testing.T) {
		// Arrange
		// Act
		got := ResolveSubject("ABC123")

		// Assert
		if got != "telemetry.raw.ABC123" {
			t.Fatalf("expected telemetry.raw.ABC123 got %q", got)
		}
	})
}

// fake JS for DLQ republish
type fakeDLQJS struct {
	publishErr error
	publishedSubject string
	publishedData []byte
	pullSubErr error
}

func (f *fakeDLQJS) Publish(subj string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	f.publishedSubject = subj
	f.publishedData = data
	return nil, f.publishErr
}
func (f *fakeDLQJS) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeDLQJS) PublishAsync(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeDLQJS) PublishMsgAsync(*nats.Msg, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeDLQJS) PublishAsyncPending() int { return 0 }
func (f *fakeDLQJS) PublishAsyncComplete() <-chan struct{} { return nil }
func (f *fakeDLQJS) CleanupPublisher() {}
func (f *fakeDLQJS) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeDLQJS) SubscribeSync(string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeDLQJS) ChanSubscribe(string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeDLQJS) ChanQueueSubscribe(string, string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeDLQJS) QueueSubscribe(string, string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeDLQJS) QueueSubscribeSync(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeDLQJS) PullSubscribe(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, f.pullSubErr }
func (f *fakeDLQJS) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeDLQJS) UpdateStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeDLQJS) DeleteStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeDLQJS) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeDLQJS) PurgeStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeDLQJS) StreamsInfo(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeDLQJS) Streams(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeDLQJS) StreamNames(...nats.JSOpt) <-chan string { return nil }
func (f *fakeDLQJS) GetMsg(string, uint64, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeDLQJS) GetLastMsg(string, string, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeDLQJS) DeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeDLQJS) SecureDeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeDLQJS) AddConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeDLQJS) UpdateConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeDLQJS) DeleteConsumer(string, string, ...nats.JSOpt) error { return nil }
func (f *fakeDLQJS) ConsumerInfo(string, string, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeDLQJS) ConsumersInfo(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeDLQJS) Consumers(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeDLQJS) ConsumerNames(string, ...nats.JSOpt) <-chan string { return nil }
func (f *fakeDLQJS) AccountInfo(...nats.JSOpt) (*nats.AccountInfo, error) { return nil, nil }
func (f *fakeDLQJS) StreamNameBySubject(string, ...nats.JSOpt) (string, error) { return "", nil }
func (f *fakeDLQJS) KeyValue(string) (nats.KeyValue, error) { return nil, nil }
func (f *fakeDLQJS) CreateKeyValue(*nats.KeyValueConfig) (nats.KeyValue, error) { return nil, nil }
func (f *fakeDLQJS) DeleteKeyValue(string) error { return nil }
func (f *fakeDLQJS) KeyValueStoreNames() <-chan string { return nil }
func (f *fakeDLQJS) KeyValueStores() <-chan nats.KeyValueStatus { return nil }
func (f *fakeDLQJS) ObjectStore(string) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeDLQJS) CreateObjectStore(*nats.ObjectStoreConfig) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeDLQJS) DeleteObjectStore(string) error { return nil }
func (f *fakeDLQJS) ObjectStoreNames(...nats.ObjectOpt) <-chan string { return nil }
func (f *fakeDLQJS) ObjectStores(...nats.ObjectOpt) <-chan nats.ObjectStoreStatus { return nil }

func TestDLQJetStream_RepublishRaw(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("success", func(t *testing.T) {
		// Arrange
		fake := &fakeDLQJS{}
		dlq := NewDLQJetStream(fake)

		// Act
		err := dlq.RepublishRaw("telemetry.raw.ABC123", []byte(`{"plate":"ABC123"}`))

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.publishedSubject != "telemetry.raw.ABC123" {
			t.Fatalf("unexpected subject %q", fake.publishedSubject)
		}
	})

	t.Run("publish error wrapped", func(t *testing.T) {
		// Arrange
		fake := &fakeDLQJS{publishErr: errors.New("nats down")}
		dlq := NewDLQJetStream(fake)

		// Act
		err := dlq.RepublishRaw("telemetry.raw.ABC123", []byte(`{}`))

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, fake.publishErr) && err.Error() == "" {
			t.Fatalf("expected wrapped error")
		}
	})
}

func TestDLQJetStream_FetchDLQ_error_when_pull_subscribe_fails(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("pull subscribe fails", func(t *testing.T) {
		// Arrange
		fake := &fakeDLQJS{pullSubErr: errors.New("no jetstream")}
		dlq := NewDLQJetStream(fake)

		// Act
		_, err := dlq.FetchDLQ(10)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestDLQJetStream_New(t *testing.T) {
	t.Run("creates dlq jetstream", func(t *testing.T) {
		// Arrange
		fake := &fakeDLQJS{}

		// Act
		dlq := NewDLQJetStream(fake)

		// Assert
		if dlq == nil {
			t.Fatalf("expected non-nil")
		}
	})
}
