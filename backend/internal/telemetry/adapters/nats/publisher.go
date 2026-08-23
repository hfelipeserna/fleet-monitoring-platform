package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"fleetmonitoring/backend/internal/telemetry/application"
	"fleetmonitoring/backend/internal/telemetry/domain"
)

type Publisher struct {
	js      nats.JetStreamContext
	timeout time.Duration
}

func NewPublisher(js nats.JetStreamContext, timeout time.Duration) *Publisher {
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	return &Publisher{js: js, timeout: timeout}
}

func (p *Publisher) Publish(ctx context.Context, evt domain.TelemetryEvent) error {
	data, err := marshalEvent(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	subject := fmt.Sprintf("telemetry.raw.%s", evt.Plate)
	f, err := p.js.PublishAsync(subject, data, nats.MsgId(evt.ClientEventID))
	if err != nil {
		if isBackpressure(err) {
			return fmt.Errorf("publish async backpressure: %w", errors.Join(application.ErrBackpressure, err))
		}
		return fmt.Errorf("publish async: %w", err)
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("publish context canceled: %w", errors.Join(application.ErrBackpressure, ctx.Err()))
	case <-time.After(p.timeout):
		return fmt.Errorf("publish timeout after %v: %w", p.timeout, application.ErrBackpressure)
	case ack := <-f.Ok():
		if ack == nil {
			return fmt.Errorf("publish ack nil: %w", application.ErrBackpressure)
		}
		return nil
	case errCh := <-f.Err():
		if errCh != nil {
			if isBackpressure(errCh) {
				return fmt.Errorf("publish ack error backpressure: %w", errors.Join(application.ErrBackpressure, errCh))
			}
			return fmt.Errorf("publish ack error: %w", errCh)
		}
		return nil
	case <-p.js.PublishAsyncComplete():
		return nil
	}
}

func (p *Publisher) PublishBatch(ctx context.Context, evts []domain.TelemetryEvent) error {
	if len(evts) == 0 {
		return fmt.Errorf("empty batch: %w", application.ErrValidation)
	}
	type pending struct {
		fut nats.PubAckFuture
	}
	pendingFuts := make([]pending, 0, len(evts))
	for _, evt := range evts {
		data, err := marshalEvent(evt)
		if err != nil {
			return fmt.Errorf("marshal batch event: %w", err)
		}
		subject := fmt.Sprintf("telemetry.raw.%s", evt.Plate)
		f, err := p.js.PublishAsync(subject, data, nats.MsgId(evt.ClientEventID))
		if err != nil {
			if isBackpressure(err) {
				return fmt.Errorf("publish batch async backpressure: %w", errors.Join(application.ErrBackpressure, err))
			}
			return fmt.Errorf("publish batch async: %w", err)
		}
		pendingFuts = append(pendingFuts, pending{fut: f})
	}
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("publish batch context canceled: %w", errors.Join(application.ErrBackpressure, ctx.Err()))
	case <-timer.C:
		return fmt.Errorf("publish batch timeout after %v: %w", p.timeout, application.ErrBackpressure)
	case <-p.js.PublishAsyncComplete():
		for _, pf := range pendingFuts {
			select {
			case err := <-pf.fut.Err():
				if err != nil {
					if isBackpressure(err) {
						return fmt.Errorf("publish batch ack backpressure: %w", errors.Join(application.ErrBackpressure, err))
					}
					return fmt.Errorf("publish batch ack error: %w", err)
				}
			default:
			}
		}
		return nil
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
	s := err.Error()
	lower := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			lower += string(c - 'A' + 'a')
		} else {
			lower += string(c)
		}
	}
	return contains(lower, "backpressure") || contains(lower, "max_pending") || contains(lower, "max pending") || contains(lower, "pending exceeded") || contains(lower, "too many")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsSlow(s, substr))
}
func containsSlow(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
