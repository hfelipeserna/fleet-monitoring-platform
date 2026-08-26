package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker"

	fleetapp "fleetmonitoring/backend/internal/fleet/application"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	fleetpg "fleetmonitoring/backend/internal/fleet/adapters/pg"
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
	_ = newAlertPublishBreaker()
	_ = newZoneResolverBreaker()
	return writer, dbBreaker, publishBreaker
}

func newAlertPublishBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "fleet-alert-publish",
		MaxRequests: 10,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < 10 {
				return false
			}
			return float64(c.TotalFailures)/float64(c.Requests) >= 0.5
		},
	})
}

func newZoneResolverBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "fleet-zone-resolver",
		MaxRequests: 10,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < 10 {
				return false
			}
			return float64(c.TotalFailures)/float64(c.Requests) >= 0.5
		},
	})
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

type jsPublisher struct {
	js nats.JetStreamContext
}

func (p *jsPublisher) Publish(ctx context.Context, alert fleet.Alert) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}
	msgID := alert.MsgID()
	_, err = p.js.Publish("alerts.critical", payload, nats.MsgId(msgID))
	if err != nil {
		return fmt.Errorf("publish alert: %w", err)
	}
	return nil
}

func ensureAlertsStreamJS(js nats.JetStreamContext) {
	_, err := js.StreamInfo("ALERTS")
	if err == nil {
		return
	}
	_, _ = js.AddStream(&nats.StreamConfig{
		Name:       "ALERTS",
		Subjects:   []string{"alerts.critical"},
		Storage:    nats.FileStorage,
		Retention:  nats.LimitsPolicy,
		Discard:    nats.DiscardOld,
		MaxAge:     7 * 24 * time.Hour,
		MaxBytes:   1 * 1024 * 1024 * 1024,
		Duplicates: 2 * time.Minute,
	})
}

func bootstrapAlertDetector(pool *pgxpool.Pool, js nats.JetStreamContext) *fleetapp.AlertDetector {
	ensureAlertsStreamJS(js)
	publisher := &jsPublisher{js: js}
	adapter := fleetpg.NewPgxPoolAdapter(pool)
	resolver := fleetpg.NewPGZoneResolver(adapter)
	zoneBreaker := newZoneResolverBreaker()
	resolverWithBreaker := fleetpg.NewZoneResolverWithBreaker(resolver, zoneBreaker, 2*time.Second)
	detector := fleetapp.NewAlertDetector(publisher, resolverWithBreaker)
	return detector
}

func bootstrapDLQ(js nats.JetStreamContext) *infranats.DLQJetStream {
	return infranats.NewDLQJetStream(js)
}
