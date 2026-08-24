package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sony/gobreaker"
)

type breakerRecorder interface {
	State() gobreaker.State
	Execute(func() (interface{}, error)) (interface{}, error)
}

type AlertPublisher struct {
	js      jetstream.JetStream
	timeout time.Duration
	breaker breakerRecorder
}

func NewAlertPublisher(js jetstream.JetStream) *AlertPublisher {
	return &AlertPublisher{js: js, timeout: 3 * time.Second}
}

func NewAlertPublisherWithBreaker(js jetstream.JetStream, breaker breakerRecorder, timeout time.Duration) *AlertPublisher {
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	return &AlertPublisher{js: js, timeout: timeout, breaker: breaker}
}

func (p *AlertPublisher) Publish(ctx context.Context, alert fleet.Alert) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}
	msgID := alert.MsgID()
	timeout := p.timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if p.breaker != nil && p.breaker.State() == gobreaker.StateOpen {
		return fmt.Errorf("breaker open: %w", gobreaker.ErrOpenState)
	}
	exec := func() (interface{}, error) {
		_, e := p.js.Publish(ctxTimeout, "alerts.critical", payload, jetstream.WithMsgID(msgID))
		return nil, e
	}
	var execErr error
	if p.breaker != nil {
		_, execErr = p.breaker.Execute(exec)
	} else {
		_, execErr = exec()
	}
	if execErr != nil {
		return fmt.Errorf("publish alert: %w", execErr)
	}
	return nil
}
