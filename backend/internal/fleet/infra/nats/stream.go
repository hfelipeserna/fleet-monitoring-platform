package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	alertsStream    = "ALERTS"
	alertsSubject   = "alerts.critical"
	alertsWild      = "alerts.>"
	telemetryStream = "TELEMETRY"
	telemetrySubj   = "telemetry.raw.>"
	defaultMaxAge   = 7 * 24 * time.Hour
	defaultMaxBytes = 1 << 30
	dupWindow       = 2 * time.Minute
)

func EnsureStream(ctx context.Context, js nats.JetStreamContext) error {
	cfg := &nats.StreamConfig{
		Name:       alertsStream,
		Subjects:   []string{alertsWild},
		Storage:    nats.FileStorage,
		Retention:  nats.LimitsPolicy,
		Discard:    nats.DiscardOld,
		MaxAge:     defaultMaxAge,
		MaxBytes:   defaultMaxBytes,
		Duplicates: dupWindow,
	}
	return ensureStream(ctx, js, cfg)
}

func EnsureStreamWithContext(ctx context.Context, js nats.JetStreamContext) error {
	return EnsureStream(ctx, js)
}

func EnsureAlertsStream(js nats.JetStreamContext) error {
	return EnsureStream(context.Background(), js)
}

func EnsureTelemetryStream(js nats.JetStreamContext, maxBytes int64) error {
	return EnsureTelemetryStreamWithContext(context.Background(), js, maxBytes)
}

func EnsureTelemetryStreamWithContext(ctx context.Context, js nats.JetStreamContext, maxBytes int64) error {
	if maxBytes == 0 {
		maxBytes = 5 << 30
	}
	cfg := &nats.StreamConfig{
		Name:       telemetryStream,
		Subjects:   []string{telemetrySubj},
		Storage:    nats.FileStorage,
		Retention:  nats.LimitsPolicy,
		Discard:    nats.DiscardOld,
		MaxAge:     24 * time.Hour,
		MaxBytes:   maxBytes,
		Duplicates: 2 * time.Minute,
	}
	return ensureStream(ctx, js, cfg)
}

func ensureStream(ctx context.Context, js nats.JetStreamContext, cfg *nats.StreamConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		return fmt.Errorf("ensure stream %s context canceled: %w", cfg.Name, ctx.Err())
	default:
	}
	info, err := js.StreamInfo(cfg.Name)
	if err == nil && info != nil {
		cfg.MaxBytes = max(cfg.MaxBytes, info.Config.MaxBytes)
		select {
		case <-ctx.Done():
			return fmt.Errorf("update stream %s context canceled: %w", cfg.Name, ctx.Err())
		default:
		}
		if _, e := js.UpdateStream(cfg); e != nil {
			return fmt.Errorf("update stream %s failed: %w", cfg.Name, e)
		}
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("create stream %s context canceled: %w", cfg.Name, ctx.Err())
	default:
	}
	if _, e := js.AddStream(cfg); e != nil {
		return fmt.Errorf("create stream %s failed: %w", cfg.Name, e)
	}
	return nil
}
