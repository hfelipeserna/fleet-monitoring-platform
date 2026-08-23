package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker"

	"fleetmonitoring/backend/internal/telemetry/application"
	"fleetmonitoring/backend/internal/telemetry/infra/breaker"
	infranats "fleetmonitoring/backend/internal/telemetry/infra/nats"
)

type jetStreamPublisher struct {
	js nats.JetStreamContext
}

func (p *jetStreamPublisher) Publish(subject string, data []byte) error {
	_, err := p.js.Publish(subject, data)
	return err
}

func consumeLoop(ctx context.Context, js nats.JetStreamContext, consumer *application.Consumer, opts application.ConsumerOptions) {
	sub, err := js.PullSubscribe(opts.Subject, opts.Durable, nats.ManualAck(), nats.MaxAckPending(opts.MaxAckPending), nats.AckWait(opts.AckWait), nats.MaxDeliver(opts.MaxDeliver))
	if err != nil {
		slog.Error("pull subscribe failed", "error", err)
		return
	}
	slog.Info("consumer started", "durable", opts.Durable, "subject", opts.Subject, "maxAckPending", opts.MaxAckPending)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msgs, err := sub.Fetch(500, nats.MaxWait(2*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			slog.Warn("fetch failed", "error", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if len(msgs) == 0 {
			continue
		}
		wrapped := make([]application.Msg, 0, len(msgs))
		for _, m := range msgs {
			wrapped = append(wrapped, infranats.NewNatsMsg(m))
		}
		if err := consumer.ProcessBatch(ctx, wrapped); err != nil {
			slog.Error("process batch failed", "error", err)
		}
	}
}

func serveHealth(port string, dlqHandler http.Handler, dbBreaker *gobreaker.CircuitBreaker, publishBreaker *breaker.Breaker) {
	mux := http.NewServeMux()
	mux.Handle("/internal/dlq/republish", dlqHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		dbState := breakerStateFromGobreaker(dbBreaker)
		publishState := breakerStateFromWrapper(publishBreaker)
		composite := dbState
		if publishState == "open" || dbState == "open" {
			composite = "open"
		} else if publishState == "half-open" || dbState == "half-open" {
			composite = "half-open"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":          "ok",
			"breaker":         composite,
			"breaker_db":      dbState,
			"breaker_publish": publishState,
		})
	})
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	slog.Info("consumer health listening", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("health serve failed", "error", err)
	}
}

func breakerStateFromGobreaker(cb *gobreaker.CircuitBreaker) string {
	if cb == nil {
		return "closed"
	}
	switch cb.State() {
	case gobreaker.StateOpen:
		return "open"
	case gobreaker.StateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

func breakerStateFromWrapper(b *breaker.Breaker) string {
	if b == nil {
		return "closed"
	}
	if b.IsOpen() {
		return "open"
	}
	s := b.State()
	if s == "half-open" {
		return "half-open"
	}
	return "closed"
}
