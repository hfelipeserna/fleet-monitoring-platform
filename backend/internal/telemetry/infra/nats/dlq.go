package nats

import (
	"errors"
	"fmt"
	"sync"
	"time"

	natsio "github.com/nats-io/nats.go"
)

const (
	DefaultDLQLimit = 100
	MaxDLQLimit     = 1000
)

var ErrDLQNotInitialized = errors.New("not initialized")

type DLQMsg interface {
	Data() []byte
	Ack() error
}

type DLQJetStream struct {
	js      natsio.JetStreamContext
	once    sync.Once
	sub     *natsio.Subscription
	initErr error
}

type dlqMsg struct {
	m *natsio.Msg
}

func (d *dlqMsg) Data() []byte { return d.m.Data }
func (d *dlqMsg) Ack() error   { return d.m.Ack() }

func NewDLQJetStream(js natsio.JetStreamContext) *DLQJetStream {
	return &DLQJetStream{js: js}
}

func (d *DLQJetStream) getSub() (*natsio.Subscription, error) {
	d.once.Do(func() {
		sub, err := d.js.PullSubscribe("telemetry.dlq", "dlq-republish", natsio.ManualAck())
		if err != nil {
			d.initErr = fmt.Errorf("dlq pull subscribe failed: %w", err)
			return
		}
		d.sub = sub
	})
	if d.initErr != nil {
		return nil, d.initErr
	}
	if d.sub == nil {
		return nil, fmt.Errorf("dlq pull subscribe failed: %w", ErrDLQNotInitialized)
	}
	return d.sub, nil
}

func (d *DLQJetStream) FetchDLQ(n int) ([]DLQMsg, error) {
	n = SanitizeDLQLimit(n)
	sub, err := d.getSub()
	if err != nil {
		return nil, err
	}
	msgs, err := sub.Fetch(n, natsio.MaxWait(2*time.Second))
	if err != nil && err != natsio.ErrTimeout {
		return nil, fmt.Errorf("dlq fetch failed: %w", err)
	}
	out := make([]DLQMsg, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, &dlqMsg{m: m})
	}
	return out, nil
}

func (d *DLQJetStream) RepublishRaw(subject string, data []byte) error {
	_, err := d.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("dlq republish failed: %w", err)
	}
	return nil
}

func SanitizeDLQLimit(n int) int {
	if n <= 0 {
		return DefaultDLQLimit
	}
	if n > MaxDLQLimit {
		return MaxDLQLimit
	}
	return n
}

func ResolveSubject(plate string) string {
	if plate == "" {
		return "telemetry.raw.unknown"
	}
	return "telemetry.raw." + plate
}
