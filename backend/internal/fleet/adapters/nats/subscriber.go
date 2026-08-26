package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker"
	sse "fleetmonitoring/backend/internal/fleet/adapters/sse"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

const sseBufferSize = 256 // channel buffer for SSE streams, fits 16GB host

type AlertSubscriber struct {
	js             nats.JetStreamContext
	breaker        *gobreaker.CircuitBreaker
	breakerTimeout time.Duration
}

func NewAlertSubscriber(js nats.JetStreamContext) *AlertSubscriber {
	return &AlertSubscriber{js: js}
}

func NewAlertSubscriberWithBreaker(js nats.JetStreamContext, breaker *gobreaker.CircuitBreaker, timeout time.Duration) *AlertSubscriber {
	return &AlertSubscriber{js: js, breaker: breaker, breakerTimeout: timeout}
}

func (s *AlertSubscriber) SubscribeAlerts(ctx context.Context, lastSeq uint64) (<-chan sse.AlertMsg, func(), error) {
	if s.js == nil {
		return nil, nil, fmt.Errorf("jetstream unavailable: %w", shared.ErrUnavailable)
	}
	if s.breaker != nil && s.breaker.State() == gobreaker.StateOpen {
		return nil, nil, fmt.Errorf("subscribe alerts breaker open: %w", gobreaker.ErrOpenState)
	}
	ch := make(chan sse.AlertMsg, sseBufferSize)
	opts := []nats.SubOpt{nats.AckNone()}
	if lastSeq == 0 {
		opts = append(opts, nats.DeliverAll())
	} else {
		opts = append(opts, nats.StartSequence(lastSeq))
	}
	ctxInner, cancel := context.WithCancel(ctx)
	var closeOnce sync.Once
	closeCh := func() { closeOnce.Do(func() { close(ch) }) }
	var sub *nats.Subscription
	cb := func(m *nats.Msg) {
		select {
		case <-ctxInner.Done():
			return
		default:
		}
		seq := uint64(0)
		if meta, err := m.Metadata(); err == nil && meta != nil {
			seq = meta.Sequence.Stream
		}
		msg := sse.AlertMsg{Seq: seq, Data: m.Data}
		select {
		case <-ctxInner.Done():
			return
		case ch <- msg:
		default:
		}
	}
	var err error
	exec := func() (any, error) {
		sub, err = s.js.Subscribe("alerts.critical", cb, opts...)
		return nil, err
	}
	if s.breaker != nil {
		if _, berr := s.breaker.Execute(exec); berr != nil {
			cancel()
			return nil, nil, fmt.Errorf("subscribe alerts: %w", berr)
		}
	} else {
		sub, err = s.js.Subscribe("alerts.critical", cb, opts...)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("subscribe alerts: %w", err)
		}
	}
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("subscribe alerts: %w", err)
	}
	var unsubOnce sync.Once
	unsub := func() {
		unsubOnce.Do(func() {
			cancel()
			_ = sub.Unsubscribe()
			closeCh()
		})
	}
	go func() {
		<-ctxInner.Done()
		_ = sub.Unsubscribe()
		closeCh()
	}()
	return ch, unsub, nil
}

type TelemetrySubscriber struct {
	js             nats.JetStreamContext
	breaker        *gobreaker.CircuitBreaker
	breakerTimeout time.Duration
}

func NewTelemetrySubscriber(js nats.JetStreamContext) *TelemetrySubscriber {
	return &TelemetrySubscriber{js: js}
}

func NewTelemetrySubscriberWithBreaker(js nats.JetStreamContext, breaker *gobreaker.CircuitBreaker, timeout time.Duration) *TelemetrySubscriber {
	return &TelemetrySubscriber{js: js, breaker: breaker, breakerTimeout: timeout}
}

func (s *TelemetrySubscriber) SubscribePositions(ctx context.Context, plate *string, lastSeq uint64) (<-chan sse.PosMsg, func(), error) {
	if s.js == nil {
		return nil, nil, fmt.Errorf("jetstream unavailable: %w", shared.ErrUnavailable)
	}
	if s.breaker != nil && s.breaker.State() == gobreaker.StateOpen {
		return nil, nil, fmt.Errorf("subscribe positions breaker open: %w", gobreaker.ErrOpenState)
	}
	ch := make(chan sse.PosMsg, sseBufferSize)
	subject := "telemetry.raw.>"
	if plate != nil && *plate != "" {
		subject = "telemetry.raw." + *plate
	}
	opts := []nats.SubOpt{nats.AckNone()}
	if lastSeq == 0 {
		opts = append(opts, nats.DeliverAll())
	} else {
		opts = append(opts, nats.StartSequence(lastSeq))
	}
	ctxInner, cancel := context.WithCancel(ctx)
	var closeOnce sync.Once
	closeCh := func() { closeOnce.Do(func() { close(ch) }) }
	var sub *nats.Subscription
	cb := func(m *nats.Msg) {
		select {
		case <-ctxInner.Done():
			return
		default:
		}
		seq := uint64(0)
		if meta, err := m.Metadata(); err == nil && meta != nil {
			seq = meta.Sequence.Stream
		}
		var raw struct {
			Plate      string    `json:"plate"`
			Lat        *float64  `json:"lat"`
			Lon        *float64  `json:"lon"`
			Speed      int       `json:"speed"`
			ReceivedAt time.Time `json:"received_at"`
		}
		if err := json.Unmarshal(m.Data, &raw); err != nil {
			raw.Plate = ""
		}
		msg := sse.PosMsg{
			Seq:        seq,
			Plate:      raw.Plate,
			Lat:        raw.Lat,
			Lon:        raw.Lon,
			Speed:      raw.Speed,
			ReceivedAt: raw.ReceivedAt,
			Data:       m.Data,
		}
		if msg.ReceivedAt.IsZero() {
			msg.ReceivedAt = time.Now().UTC()
		}
		select {
		case <-ctxInner.Done():
			return
		case ch <- msg:
		default:
		}
	}
	var err error
	exec := func() (any, error) {
		sub, err = s.js.Subscribe(subject, cb, opts...)
		return nil, err
	}
	if s.breaker != nil {
		if _, berr := s.breaker.Execute(exec); berr != nil {
			cancel()
			return nil, nil, fmt.Errorf("subscribe positions: %w", berr)
		}
	} else {
		sub, err = s.js.Subscribe(subject, cb, opts...)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("subscribe positions: %w", err)
		}
	}
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("subscribe positions: %w", err)
	}
	var unsubOnce sync.Once
	unsub := func() {
		unsubOnce.Do(func() {
			cancel()
			_ = sub.Unsubscribe()
			closeCh()
		})
	}
	go func() {
		<-ctxInner.Done()
		_ = sub.Unsubscribe()
		closeCh()
	}()
	return ch, unsub, nil
}
