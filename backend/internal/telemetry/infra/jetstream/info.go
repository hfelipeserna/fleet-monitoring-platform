package jetstream

import (
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Info struct {
	js         nats.JetStreamContext
	streamName string
	mu         sync.Mutex
	lastBytes  uint64
	lastMax    uint64
	lastFetched time.Time
}

func NewInfo(js nats.JetStreamContext, stream string) *Info {
	if stream == "" {
		stream = "TELEMETRY"
	}
	return &Info{js: js, streamName: stream}
}

func (i *Info) Bytes() (uint64, uint64) {
	if i.js == nil {
		return 0, 0
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.lastFetched.IsZero() && time.Since(i.lastFetched) < time.Second {
		return i.lastBytes, i.lastMax
	}
	info, err := i.js.StreamInfo(i.streamName)
	if err != nil {
		slog.Error("jetstream StreamInfo failed", "stream", i.streamName, "error", err)
		if !i.lastFetched.IsZero() {
			return i.lastBytes, i.lastMax
		}
		return 0, 0
	}
	used := info.State.Bytes
	max := uint64(info.Config.MaxBytes)
	if max == 0 {
		max = 5 * 1024 * 1024 * 1024
	}
	i.lastBytes = used
	i.lastMax = max
	i.lastFetched = time.Now()
	return used, max
}
