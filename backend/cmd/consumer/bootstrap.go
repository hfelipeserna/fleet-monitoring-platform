package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker"

	"fleetmonitoring/backend/internal/telemetry/adapters/pg"
	"fleetmonitoring/backend/internal/telemetry/application"
	"fleetmonitoring/backend/internal/telemetry/infra/breaker"
	"fleetmonitoring/backend/internal/telemetry/infra/env"
	"fleetmonitoring/backend/internal/telemetry/infra/jetstream"
	infranats "fleetmonitoring/backend/internal/telemetry/infra/nats"
)

const (
	dbBreakerMaxRequests       = 5
	dbBreakerInterval          = 30 * time.Second
	dbBreakerTimeout           = 30 * time.Second
	dbBreakerRequestsThreshold = 10
	dbBreakerFailureRatio      = 0.5
)

type config struct {
	natsURL     string
	databaseURL string
	healthPort  string
}

func loadConfig() config {
	return config{
		natsURL:     env.Get("NATS_URL", "nats://localhost:4222"),
		databaseURL: env.Get("DATABASE_URL", "postgres://fleet:fleet@localhost:5432/fleet?sslmode=disable"),
		healthPort:  env.Get("CONSUMER_HEALTH_PORT", "8081"),
	}
}

func bootstrapNATS(cfg config) (nats.JetStreamContext, *nats.Conn, error) {
	nc, err := nats.Connect(cfg.natsURL, nats.MaxReconnects(-1))
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect failed: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, nil, fmt.Errorf("jetstream context failed: %w", err)
	}
	maxBytes := env.GetInt64("JETSTREAM_MAX_BYTES", jetstream.DefaultMaxBytes)
	if err := infranats.EnsureStream(js, maxBytes); err != nil {
		_ = nc.Drain()
		return nil, nil, fmt.Errorf("ensure stream failed: %w", err)
	}
	return js, nc, nil
}

func bootstrapPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool creation failed: %w", err)
	}
	return pool, nil
}

func bootstrapWriter(pool *pgxpool.Pool) (*pg.Writer, *gobreaker.CircuitBreaker) {
	cbSettings := gobreaker.Settings{
		Name:        "telemetry-consumer-db",
		MaxRequests: dbBreakerMaxRequests,
		Interval:    dbBreakerInterval,
		Timeout:     dbBreakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < dbBreakerRequestsThreshold {
				return false
			}
			return float64(counts.TotalFailures)/float64(counts.Requests) >= dbBreakerFailureRatio
		},
	}
	dbBreaker := gobreaker.NewCircuitBreaker(cbSettings)
	writer := pg.NewWriterWithBreaker(pool, dbBreaker)
	return writer, dbBreaker
}

func bootstrapBreakers(pool *pgxpool.Pool) (*pg.Writer, *gobreaker.CircuitBreaker, *breaker.Breaker) {
	writer, dbBreaker := bootstrapWriter(pool)
	publishBreaker := breaker.NewBreaker()
	return writer, dbBreaker, publishBreaker
}

func bootstrapConsumer(writer application.TelemetryWriter, js nats.JetStreamContext) (*application.Consumer, application.ConsumerOptions) {
	jetPublisher := &jetStreamPublisher{js: js}
	opts := application.ConsumerOptions{
		Durable:       "fleet-consumer",
		Subject:       "telemetry.raw.*",
		MaxDeliver:    3,
		MaxAckPending: 10000,
		AckWait:       30 * time.Second,
		DLQSubject:    "telemetry.dlq",
	}
	consumer := application.NewConsumer(writer, jetPublisher, opts)
	_ = infranats.EnsureConsumer(js, opts)
	return consumer, opts
}

func bootstrapDLQ(js nats.JetStreamContext) *infranats.DLQJetStream {
	return infranats.NewDLQJetStream(js)
}
