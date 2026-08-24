package nats

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func envMaxBytes() int64 {
	if v := os.Getenv("ALERTS_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 1 << 30
}

func envReplicas() int {
	if v := os.Getenv("ALERTS_REPLICAS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

func EnsureAlertsStream(ctx context.Context, js jetstream.JetStream) error {
	maxBytes := envMaxBytes()
	replicas := envReplicas()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cfg := jetstream.StreamConfig{
		Name:       "ALERTS",
		Subjects:   []string{"alerts.>"},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		Discard:    jetstream.DiscardOld,
		MaxAge:     7 * 24 * time.Hour,
		MaxBytes:   maxBytes,
		Replicas:   replicas,
		Duplicates: 2 * time.Minute,
	}
	_, err := js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("create alerts stream context canceled: %w", ctx.Err())
		default:
		}
		return fmt.Errorf("create alerts stream: %w", err)
	}
	return nil
}
