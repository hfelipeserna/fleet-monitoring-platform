package jetstream

import (
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type fakeJSInfo struct {
	info *nats.StreamInfo
	err  error
	calls int
}

func (f *fakeJSInfo) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.calls++
	return f.info, f.err
}

// stubs to satisfy JetStreamContext interface for Info only need StreamInfo
func (f *fakeJSInfo) Publish(string, []byte, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeJSInfo) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) { return nil, nil }
func (f *fakeJSInfo) PublishAsync(string, []byte, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeJSInfo) PublishMsgAsync(*nats.Msg, ...nats.PubOpt) (nats.PubAckFuture, error) { return nil, nil }
func (f *fakeJSInfo) PublishAsyncPending() int { return 0 }
func (f *fakeJSInfo) PublishAsyncComplete() <-chan struct{} { return nil }
func (f *fakeJSInfo) CleanupPublisher() {}
func (f *fakeJSInfo) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) SubscribeSync(string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) ChanSubscribe(string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) ChanQueueSubscribe(string, string, chan *nats.Msg, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) QueueSubscribe(string, string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) QueueSubscribeSync(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) PullSubscribe(string, string, ...nats.SubOpt) (*nats.Subscription, error) { return nil, nil }
func (f *fakeJSInfo) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeJSInfo) UpdateStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (f *fakeJSInfo) DeleteStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeJSInfo) StreamInfoWithContext() {}
func (f *fakeJSInfo) PurgeStream(string, ...nats.JSOpt) error { return nil }
func (f *fakeJSInfo) StreamsInfo(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeJSInfo) Streams(...nats.JSOpt) <-chan *nats.StreamInfo { return nil }
func (f *fakeJSInfo) StreamNames(...nats.JSOpt) <-chan string { return nil }
func (f *fakeJSInfo) GetMsg(string, uint64, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeJSInfo) GetLastMsg(string, string, ...nats.JSOpt) (*nats.RawStreamMsg, error) { return nil, nil }
func (f *fakeJSInfo) DeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeJSInfo) SecureDeleteMsg(string, uint64, ...nats.JSOpt) error { return nil }
func (f *fakeJSInfo) AddConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeJSInfo) UpdateConsumer(string, *nats.ConsumerConfig, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeJSInfo) DeleteConsumer(string, string, ...nats.JSOpt) error { return nil }
func (f *fakeJSInfo) ConsumerInfo(string, string, ...nats.JSOpt) (*nats.ConsumerInfo, error) { return nil, nil }
func (f *fakeJSInfo) ConsumersInfo(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeJSInfo) Consumers(string, ...nats.JSOpt) <-chan *nats.ConsumerInfo { return nil }
func (f *fakeJSInfo) ConsumerNames(string, ...nats.JSOpt) <-chan string { return nil }
func (f *fakeJSInfo) AccountInfo(...nats.JSOpt) (*nats.AccountInfo, error) { return nil, nil }
func (f *fakeJSInfo) StreamNameBySubject(string, ...nats.JSOpt) (string, error) { return "", nil }
func (f *fakeJSInfo) KeyValue(string) (nats.KeyValue, error) { return nil, nil }
func (f *fakeJSInfo) CreateKeyValue(*nats.KeyValueConfig) (nats.KeyValue, error) { return nil, nil }
func (f *fakeJSInfo) DeleteKeyValue(string) error { return nil }
func (f *fakeJSInfo) KeyValueStoreNames() <-chan string { return nil }
func (f *fakeJSInfo) KeyValueStores() <-chan nats.KeyValueStatus { return nil }
func (f *fakeJSInfo) ObjectStore(string) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeJSInfo) CreateObjectStore(*nats.ObjectStoreConfig) (nats.ObjectStore, error) { return nil, nil }
func (f *fakeJSInfo) DeleteObjectStore(string) error { return nil }
func (f *fakeJSInfo) ObjectStoreNames(...nats.ObjectOpt) <-chan string { return nil }
func (f *fakeJSInfo) ObjectStores(...nats.ObjectOpt) <-chan nats.ObjectStoreStatus { return nil }


func TestInfo_Bytes(t *testing.T) {
	// Covers [SPEC-001: BR-006]
	t.Run("returns 0,0 when js nil", func(t *testing.T) {
		// Arrange
		info := NewInfo(nil, "TELEMETRY")

		// Act
		used, max := info.Bytes()

		// Assert
		if used != 0 || max != 0 {
			t.Fatalf("expected 0,0 got %d,%d", used, max)
		}
	})

	t.Run("returns bytes and max from StreamInfo", func(t *testing.T) {
		// Arrange
		fake := &fakeJSInfo{info: &nats.StreamInfo{State: nats.StreamState{Bytes: 12345}, Config: nats.StreamConfig{MaxBytes: 1 << 30}}}
		info := NewInfo(fake, "TELEMETRY")

		// Act
		used, max := info.Bytes()

		// Assert
		if used != 12345 || max != 1<<30 {
			t.Fatalf("expected 12345,1<<30 got %d,%d", used, max)
		}
	})

	t.Run("fallback to 5 GiB when MaxBytes 0", func(t *testing.T) {
		// Arrange
		fake := &fakeJSInfo{info: &nats.StreamInfo{State: nats.StreamState{Bytes: 100}, Config: nats.StreamConfig{MaxBytes: 0}}}
		info := NewInfo(fake, "TELEMETRY")

		// Act
		used, max := info.Bytes()

		// Assert
		if max != DefaultMaxBytes {
			t.Fatalf("expected fallback %d got %d", DefaultMaxBytes, max)
		}
		if used != 100 {
			t.Fatalf("expected used 100 got %d", used)
		}
	})

	t.Run("caches within 1 second", func(t *testing.T) {
		// Arrange
		fake := &fakeJSInfo{info: &nats.StreamInfo{State: nats.StreamState{Bytes: 10}, Config: nats.StreamConfig{MaxBytes: 100}}}
		info := NewInfo(fake, "TELEMETRY")
		_, _ = info.Bytes()

		// Act
		used, max := info.Bytes()

		// Assert
		if fake.calls != 1 {
			t.Fatalf("expected cached, calls %d", fake.calls)
		}
		if used != 10 || max != 100 {
			t.Fatalf("unexpected %d,%d", used, max)
		}
	})

	t.Run("returns cached on error after first success", func(t *testing.T) {
		// Arrange
		fake := &fakeJSInfo{info: &nats.StreamInfo{State: nats.StreamState{Bytes: 77}, Config: nats.StreamConfig{MaxBytes: 200}}}
		info := NewInfo(fake, "TELEMETRY")
		_, _ = info.Bytes()
		// inject error but still within cache window, but we need to expire cache to trigger fetch; wait >1s or manipulate time? instead force second fetch after expiry
		// trick: set lastFetched to old time by creating new info with expired? Instead we sleep 1.1s
		time.Sleep(1100 * time.Millisecond)
		fake.err = errors.New("fail")
		fake.info = nil

		// Act
		used, max := info.Bytes()

		// Assert
		if used != 77 || max != 200 {
			t.Fatalf("expected cached 77,200 got %d,%d", used, max)
		}
	})

	t.Run("returns 0,0 on first fetch error", func(t *testing.T) {
		// Arrange
		fake := &fakeJSInfo{err: errors.New("not found")}
		info := NewInfo(fake, "TELEMETRY")

		// Act
		used, max := info.Bytes()

		// Assert
		if used != 0 || max != 0 {
			t.Fatalf("expected 0,0 got %d,%d", used, max)
		}
	})

	t.Run("defaults stream name to TELEMETRY when empty", func(t *testing.T) {
		// Arrange
		fake := &fakeJSInfo{info: &nats.StreamInfo{State: nats.StreamState{Bytes: 1}, Config: nats.StreamConfig{MaxBytes: 10}}}
		info := NewInfo(fake, "")

		// Act
		used, max := info.Bytes()

		// Assert
		if used != 1 || max != 10 {
			t.Fatalf("unexpected %d,%d", used, max)
		}
	})
}
