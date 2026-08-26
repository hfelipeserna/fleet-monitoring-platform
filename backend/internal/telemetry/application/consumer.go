package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
)

type TelemetryWriter interface {
	WriteBatch(ctx context.Context, evts []telemetry.TelemetryEvent) (int64, error)
}

type JetStreamPublisher interface {
	Publish(subject string, data []byte) error
}

type ConsumerOptions struct {
	Durable       string
	Subject       string
	MaxDeliver    int
	MaxAckPending int
	AckWait       time.Duration
	DLQSubject    string
}

type Msg interface {
	Data() []byte
	Ack() error
	Nak() error
	NakWithDelay(time.Duration) error
	Term() error
	Delivered() int
}

type AlertProcessor interface {
	Process(ctx context.Context, plate string, lat, lon *float64, speed int) error
}

type Consumer struct {
	writer         TelemetryWriter
	js             JetStreamPublisher
	opts           ConsumerOptions
	clock          Clock
	alertProcessor AlertProcessor
}

func NewConsumer(writer TelemetryWriter, js JetStreamPublisher, opts ConsumerOptions) *Consumer {
	return NewConsumerWithClock(writer, js, opts, nil)
}

func NewConsumerWithClock(writer TelemetryWriter, js JetStreamPublisher, opts ConsumerOptions, clock Clock) *Consumer {
	if opts.Durable == "" {
		opts.Durable = "fleet-consumer"
	}
	if opts.Subject == "" {
		opts.Subject = "telemetry.raw.*"
	}
	if opts.MaxDeliver == 0 {
		opts.MaxDeliver = 3
	}
	if opts.MaxAckPending == 0 {
		opts.MaxAckPending = 10000
	}
	if opts.AckWait == 0 {
		opts.AckWait = 30 * time.Second
	}
	if opts.DLQSubject == "" {
		opts.DLQSubject = "telemetry.dlq"
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Consumer{writer: writer, js: js, opts: opts, clock: clock}
}

func (c *Consumer) WithAlertProcessor(p AlertProcessor) *Consumer {
	c.alertProcessor = p
	return c
}

func (c *Consumer) Config() ConsumerOptions {
	return c.opts
}

func (c *Consumer) HandleMessage(ctx context.Context, msg Msg) error {
	return c.ProcessBatch(ctx, []Msg{msg})
}

func (c *Consumer) ProcessBatch(ctx context.Context, msgs []Msg) error {
	if len(msgs) == 0 {
		return nil
	}
	validMsgs, events := c.partitionValid(msgs)
	if len(events) == 0 {
		return nil
	}
	if c.writer == nil {
		for _, m := range validMsgs {
			_ = m.Ack()
		}
		return nil
	}
	_, err := c.writer.WriteBatch(ctx, events)
	c.handleWriteResult(validMsgs, err)
	if err == nil && c.alertProcessor != nil {
		for _, evt := range events {
			_ = c.alertProcessor.Process(ctx, evt.Plate, evt.Lat, evt.Lon, evt.Speed)
		}
	}
	return nil
}

func (c *Consumer) partitionValid(msgs []Msg) ([]Msg, []telemetry.TelemetryEvent) {
	validMsgs := make([]Msg, 0, len(msgs))
	events := make([]telemetry.TelemetryEvent, 0, len(msgs))
	now := c.clock.Now()
	for _, m := range msgs {
		evt, err := parsePayloadWithClock(m.Data(), c.clock)
		if err != nil {
			_ = m.Term()
			continue
		}
		if err := evt.ValidateAt(now); err != nil {
			_ = m.Term()
			continue
		}
		validMsgs = append(validMsgs, m)
		events = append(events, evt)
	}
	return validMsgs, events
}

func (c *Consumer) handleWriteResult(validMsgs []Msg, err error) {
	if err == nil {
		for _, m := range validMsgs {
			_ = m.Ack()
		}
		return
	}
	for _, m := range validMsgs {
		delivered := m.Delivered()
		if delivered >= c.opts.MaxDeliver {
			if c.js != nil {
				_ = c.js.Publish(c.opts.DLQSubject, m.Data())
			}
			_ = m.Ack()
		} else {
			_ = m.NakWithDelay(backoffFor(delivered))
		}
	}
}

func backoffFor(delivered int) time.Duration {
	d := time.Second * time.Duration(delivered)
	if d < time.Second {
		d = time.Second
	}
	return d
}

func parsePayload(data []byte) (telemetry.TelemetryEvent, error) {
	return parsePayloadWithClock(data, systemClock{})
}

func parsePayloadWithClock(data []byte, clock Clock) (telemetry.TelemetryEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return telemetry.TelemetryEvent{}, fmt.Errorf("invalid json: %w", err)
	}
	plate, err := parsePlateField(raw)
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	speed, err := parseSpeedField(raw)
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	clientID := parseClientID(raw)
	lat, err := parseOptionalFloatField(raw, "lat")
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	lon, err := parseOptionalFloatField(raw, "lon")
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	now := time.Now().UTC()
	if clock != nil {
		now = clock.Now()
	}
	receivedAt := parseReceivedAtFieldWithClock(raw, now)
	occurredAt, _ := parseOptionalTimeField(raw, "occurred_at")
	return telemetry.TelemetryEvent{
		ClientEventID: clientID,
		Plate:         plate,
		Speed:         speed,
		Lat:           lat,
		Lon:           lon,
		ReceivedAt:    receivedAt,
		OccurredAt:    occurredAt,
	}, nil
}

func parsePlateField(raw map[string]json.RawMessage) (string, error) {
	plateRaw, ok := raw["plate"]
	if !ok {
		return "", fmt.Errorf("missing plate: %w", ErrValidation)
	}
	var plate string
	if err := json.Unmarshal(plateRaw, &plate); err != nil {
		return "", fmt.Errorf("invalid plate: %w", err)
	}
	if _, err := shared.ParsePlate(plate); err != nil {
		return "", fmt.Errorf("invalid plate: %w", err)
	}
	return plate, nil
}

func parseSpeedField(raw map[string]json.RawMessage) (int, error) {
	speedRaw, ok := raw["speed"]
	if !ok {
		return 0, fmt.Errorf("missing speed: %w", ErrValidation)
	}
	return parseSpeed(speedRaw)
}

func parseSpeed(raw json.RawMessage) (int, error) {
	s := string(raw)
	if s == "null" || s == "" {
		return 0, fmt.Errorf("missing speed: %w", ErrValidation)
	}
	for _, c := range s {
		if c == '.' || c == 'e' || c == 'E' {
			return 0, fmt.Errorf("speed must be integer: %w", ErrValidation)
		}
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("speed must be integer: %w", err)
	}
	return v, nil
}

func parseClientID(raw map[string]json.RawMessage) string {
	if v, ok := raw["client_event_id"]; ok {
		var s string
		_ = json.Unmarshal(v, &s)
		return s
	}
	return ""
}

func parseOptionalFloatField(raw map[string]json.RawMessage, key string) (*float64, error) {
	v, ok := raw[key]
	if !ok {
		return nil, nil
	}
	if isNullOrEmpty(v) {
		return nil, nil
	}
	var f float64
	if err := json.Unmarshal(v, &f); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	return &f, nil
}

func parseOptionalTimeField(raw map[string]json.RawMessage, key string) (*time.Time, error) {
	v, ok := raw[key]
	if !ok {
		return nil, nil
	}
	if isNullOrEmpty(v) {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	return nil, nil
}

func parseReceivedAtField(raw map[string]json.RawMessage) time.Time {
	return parseReceivedAtFieldWithClock(raw, time.Now().UTC())
}

func parseReceivedAtFieldWithClock(raw map[string]json.RawMessage, now time.Time) time.Time {
	if t, _ := parseOptionalTimeField(raw, "received_at"); t != nil {
		return *t
	}
	return now
}

func isNullOrEmpty(v json.RawMessage) bool {
	s := string(v)
	return s == "null" || s == ""
}
