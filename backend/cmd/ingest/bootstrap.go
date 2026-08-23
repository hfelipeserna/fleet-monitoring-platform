package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	httpadapter "fleetmonitoring/backend/internal/telemetry/adapters/http"
	natsadapter "fleetmonitoring/backend/internal/telemetry/adapters/nats"
	"fleetmonitoring/backend/internal/telemetry/infra/breaker"
	"fleetmonitoring/backend/internal/telemetry/infra/env"
	"fleetmonitoring/backend/internal/telemetry/infra/jetstream"
	infranats "fleetmonitoring/backend/internal/telemetry/infra/nats"
	"fleetmonitoring/backend/internal/telemetry/infra/rate"

	"github.com/nats-io/nats.go"
)

type config struct {
	natsURL           string
	httpPort          string
	natsMaxPending    int
	jetstreamMaxBytes int64
	publishTimeout    time.Duration
}

func loadConfig() config {
	return config{
		natsURL:           env.Get("NATS_URL", "nats://localhost:4222"),
		httpPort:          env.Get("HTTP_PORT", "8080"),
		natsMaxPending:    env.GetInt("NATS_MAX_PENDING", 1024),
		jetstreamMaxBytes: env.GetInt64("JETSTREAM_MAX_BYTES", jetstream.DefaultMaxBytes),
		publishTimeout:    env.GetDuration("PUBLISH_TIMEOUT", 3*time.Second),
	}
}

func bootstrapNATS(cfg config) (nats.JetStreamContext, *nats.Conn, error) {
	nc, err := nats.Connect(cfg.natsURL, nats.MaxReconnects(-1))
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect failed: %w", err)
	}
	js, err := nc.JetStream(nats.PublishAsyncMaxPending(cfg.natsMaxPending))
	if err != nil {
		_ = nc.Drain()
		return nil, nil, fmt.Errorf("jetstream context failed: %w", err)
	}
	if err := infranats.EnsureStream(js, cfg.jetstreamMaxBytes); err != nil {
		_ = nc.Drain()
		return nil, nil, fmt.Errorf("ensure stream failed: %w", err)
	}
	return js, nc, nil
}

func bootstrapServer(ctx context.Context, js nats.JetStreamContext, cfg config) (*http.Server, *rate.Limiter) {
	brk := breaker.NewBreaker()
	pub := natsadapter.NewPublisherWithBreaker(js, cfg.publishTimeout, brk)
	limiter := rate.NewLimiterWithContext(ctx)
	jsInfo := jetstream.NewInfo(js, "TELEMETRY")
	handler := httpadapter.NewHandler(pub, limiter, brk, jsInfo)
	srv := newHTTPServer(handler, cfg.httpPort)
	return srv, limiter
}
