package nats

import (
	"log/slog"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
	natsio "github.com/nats-io/nats.go"
)

const (
	streamMaxAge     = 24 * time.Hour
	streamDuplicates = 24 * time.Hour
)

func EnsureStream(js natsio.JetStreamContext, maxBytes int64) error {
	_, err := js.StreamInfo("TELEMETRY")
	if err == nil {
		return nil
	}
	_, err = js.AddStream(&natsio.StreamConfig{
		Name:       "TELEMETRY",
		Subjects:   []string{"telemetry.raw.>"},
		Storage:    natsio.FileStorage,
		Retention:  natsio.LimitsPolicy,
		Discard:    natsio.DiscardOld,
		MaxAge:     streamMaxAge,
		MaxBytes:   maxBytes,
		Duplicates: streamDuplicates,
	})
	if err != nil {
		slog.Warn("ensure stream failed", "error", err)
		return err
	}
	return nil
}

func EnsureConsumer(js natsio.JetStreamContext, opts application.ConsumerOptions) error {
	_, err := js.ConsumerInfo("TELEMETRY", opts.Durable)
	if err == nil {
		return nil
	}
	_, err = js.AddConsumer("TELEMETRY", &natsio.ConsumerConfig{
		Durable:       opts.Durable,
		DeliverPolicy: natsio.DeliverAllPolicy,
		AckPolicy:     natsio.AckExplicitPolicy,
		AckWait:       opts.AckWait,
		MaxDeliver:    opts.MaxDeliver,
		MaxAckPending: opts.MaxAckPending,
		FilterSubject: opts.Subject,
	})
	if err != nil {
		slog.Warn("ensure consumer failed", "error", err)
		return err
	}
	return nil
}
