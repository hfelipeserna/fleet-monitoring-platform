package sse

import "time"

type AlertMsg struct {
	Seq  uint64
	Data []byte
}

type PosMsg struct {
	Seq        uint64
	Plate      string
	Lat        *float64
	Lon        *float64
	Speed      int
	ReceivedAt time.Time
	Data       []byte
}

type Option func(*handlerConfig)

type handlerConfig struct {
	pingInterval time.Duration
}

func WithPingInterval(d time.Duration) Option {
	return func(c *handlerConfig) { c.pingInterval = d }
}
