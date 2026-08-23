package jetstream

import (
	"github.com/nats-io/nats.go"
)

type Info struct {
	js         nats.JetStreamContext
	streamName string
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
	info, err := i.js.StreamInfo(i.streamName)
	if err != nil {
		return 0, 0
	}
	used := info.State.Bytes
	max := uint64(info.Config.MaxBytes)
	if max == 0 {
		max = 5 * 1024 * 1024 * 1024
		if info.Config.MaxBytes == 0 {
			max = 5 * 1024 * 1024 * 1024
		}
	}
	return used, max
}
