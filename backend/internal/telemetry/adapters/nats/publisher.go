package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
	"fleetmonitoring/backend/internal/telemetry/domain"
	"github.com/nats-io/nats.go"
)

const defaultPublishTimeout = 3 * time.Second

type breakerRecorder interface {
	RecordSuccess()
	RecordFailure()
}

type Publisher struct {
	js      nats.JetStreamContext
	timeout time.Duration
	breaker breakerRecorder
}

func NewPublisher(js nats.JetStreamContext, timeout time.Duration) *Publisher {
	return NewPublisherWithBreaker(js, timeout, nil)
}

func NewPublisherWithBreaker(js nats.JetStreamContext, timeout time.Duration, brk breakerRecorder) *Publisher {
	if timeout == 0 {
		timeout = defaultPublishTimeout
	}
	return &Publisher{js: js, timeout: timeout, breaker: brk}
}

func (p *Publisher) recordSuccess() {
	if p.breaker != nil {
		p.breaker.RecordSuccess()
	}
}

func (p *Publisher) recordFailure() {
	if p.breaker != nil {
		p.breaker.RecordFailure()
	}
}

func (p *Publisher) Publish(ctx context.Context, evt domain.TelemetryEvent) error {
	data, err := marshalEvent(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	subject := fmt.Sprintf("telemetry.raw.%s", evt.Plate)
	f, err := p.js.PublishAsync(subject, data, nats.MsgId(evt.ClientEventID))
	if err != nil {
		p.recordFailure()
		if isBackpressure(err) {
			return fmt.Errorf("publish async backpressure: %w", errors.Join(application.ErrBackpressure, err))
		}
		return fmt.Errorf("publish async: %w", err)
	}
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	return p.waitPublishAck(ctx, f, timer)
}

func (p *Publisher) waitPublishAck(ctx context.Context, fut nats.PubAckFuture, timer *time.Timer) error {
	select {
	case <-ctx.Done():
		p.recordFailure()
		return fmt.Errorf("publish context canceled: %w", errors.Join(application.ErrBackpressure, ctx.Err()))
	case <-timer.C:
		p.recordFailure()
		return fmt.Errorf("publish timeout after %v: %w", p.timeout, application.ErrBackpressure)
	case ack := <-fut.Ok():
		if ack == nil {
			p.recordFailure()
			return fmt.Errorf("publish ack nil: %w", application.ErrBackpressure)
		}
		p.recordSuccess()
		return nil
	case errCh := <-fut.Err():
		if errCh != nil {
			p.recordFailure()
			if isBackpressure(errCh) {
				return fmt.Errorf("publish ack error backpressure: %w", errors.Join(application.ErrBackpressure, errCh))
			}
			return fmt.Errorf("publish ack error: %w", errCh)
		}
		p.recordSuccess()
		return nil
	}
}

func (p *Publisher) PublishBatch(ctx context.Context, evts []domain.TelemetryEvent) error {
	if len(evts) == 0 {
		return fmt.Errorf("empty batch: %w", application.ErrValidation)
	}
	pendingFuts, err := p.publishAllAsync(evts)
	if err != nil {
		return err
	}
	return p.awaitAcks(ctx, pendingFuts)
}

type pending struct {
	fut nats.PubAckFuture
}

func (p *Publisher) publishAllAsync(evts []domain.TelemetryEvent) ([]pending, error) {
	pendingFuts := make([]pending, 0, len(evts))
	for _, evt := range evts {
		data, err := marshalEvent(evt)
		if err != nil {
			return nil, fmt.Errorf("marshal batch event: %w", err)
		}
		subject := fmt.Sprintf("telemetry.raw.%s", evt.Plate)
		f, err := p.js.PublishAsync(subject, data, nats.MsgId(evt.ClientEventID))
		if err != nil {
			p.recordFailure()
			if isBackpressure(err) {
				return nil, fmt.Errorf("publish batch async backpressure: %w", errors.Join(application.ErrBackpressure, err))
			}
			return nil, fmt.Errorf("publish batch async: %w", err)
		}
		pendingFuts = append(pendingFuts, pending{fut: f})
	}
	return pendingFuts, nil
}

func (p *Publisher) awaitAcks(ctx context.Context, pendingFuts []pending) error {
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		p.recordFailure()
		return fmt.Errorf("publish batch context canceled: %w", errors.Join(application.ErrBackpressure, ctx.Err()))
	case <-timer.C:
		p.recordFailure()
		return fmt.Errorf("publish batch timeout after %v: %w", p.timeout, application.ErrBackpressure)
	case <-p.js.PublishAsyncComplete():
		for _, pf := range pendingFuts {
			if err := p.handleAckResult(ctx, pf, timer); err != nil {
				return err
			}
		}
		p.recordSuccess()
		return nil
	}
}

func (p *Publisher) handleAckResult(ctx context.Context, pf pending, timer *time.Timer) error {
	select {
	case ack := <-pf.fut.Ok():
		if ack == nil {
			p.recordFailure()
			return fmt.Errorf("publish batch ack nil: %w", application.ErrBackpressure)
		}
		return nil
	case err := <-pf.fut.Err():
		if err != nil {
			p.recordFailure()
			if isBackpressure(err) {
				return fmt.Errorf("publish batch ack backpressure: %w", errors.Join(application.ErrBackpressure, err))
			}
			return fmt.Errorf("publish batch ack error: %w", err)
		}
		return nil
	case <-timer.C:
		p.recordFailure()
		return fmt.Errorf("publish batch timeout waiting ack: %w", application.ErrBackpressure)
	case <-ctx.Done():
		p.recordFailure()
		return fmt.Errorf("publish batch context canceled waiting ack: %w", errors.Join(application.ErrBackpressure, ctx.Err()))
	}
}

func marshalEvent(evt domain.TelemetryEvent) ([]byte, error) {
	payload := map[string]any{
		"plate":           evt.Plate,
		"speed":           evt.Speed,
		"lat":             evt.Lat,
		"lon":             evt.Lon,
		"client_event_id": evt.ClientEventID,
		"received_at":     evt.ReceivedAt.Format(time.RFC3339Nano),
	}
	if evt.OccurredAt != nil {
		payload["occurred_at"] = evt.OccurredAt.Format(time.RFC3339Nano)
	}
	return json.Marshal(payload)
}

func isBackpressure(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "backpressure") || strings.Contains(s, "max_pending") || strings.Contains(s, "max pending") || strings.Contains(s, "pending exceeded") || strings.Contains(s, "too many")
}
