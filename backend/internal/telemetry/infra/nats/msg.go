package nats

import (
	"time"

	natsio "github.com/nats-io/nats.go"
)

type NatsMsg struct {
	*natsio.Msg
}

func NewNatsMsg(m *natsio.Msg) *NatsMsg {
	return &NatsMsg{Msg: m}
}

func (n *NatsMsg) Data() []byte {
	return n.Msg.Data
}

func (n *NatsMsg) Ack() error {
	return n.Msg.Ack()
}

func (n *NatsMsg) Nak() error {
	return n.Msg.Nak()
}

func (n *NatsMsg) NakWithDelay(d time.Duration) error {
	return n.Msg.NakWithDelay(d)
}

func (n *NatsMsg) Term() error {
	return n.Msg.Term()
}

func (n *NatsMsg) Delivered() int {
	md, err := n.Msg.Metadata()
	if err != nil {
		return 1
	}
	return int(md.NumDelivered)
}
