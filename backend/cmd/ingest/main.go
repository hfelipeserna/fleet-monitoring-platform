package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	natsadapter "fleetmonitoring/backend/internal/telemetry/adapters/nats"
	httpadapter "fleetmonitoring/backend/internal/telemetry/adapters/http"
	"fleetmonitoring/backend/internal/telemetry/infra/breaker"
	"fleetmonitoring/backend/internal/telemetry/infra/jetstream"
	"fleetmonitoring/backend/internal/telemetry/infra/rate"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	natsURL := envOr("NATS_URL", "nats://localhost:4222")
	httpPort := envOr("HTTP_PORT", "8080")

	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		slog.Error("nats connect failed", "error", err, "url", natsURL)
		os.Exit(1)
	}
	defer nc.Drain()

	js, err := nc.JetStream(nats.PublishAsyncMaxPending(1024))
	if err != nil {
		slog.Error("jetstream context failed", "error", err)
		os.Exit(1)
	}
	ensureStream(js)

	pub := natsadapter.NewPublisher(js, 3*time.Second)
	limiter := rate.NewLimiter()
	brk := breaker.NewBreaker()
	jsInfo := jetstream.NewInfo(js, "TELEMETRY")

	handler := httpadapter.NewHandler(pub, limiter, brk, jsInfo)

	srv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("ingest listening", "port", httpPort, "nats", natsURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http serve failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
	slog.Info("ingest stopped")
}

func ensureStream(js nats.JetStreamContext) {
	_, err := js.StreamInfo("TELEMETRY")
	if err == nil {
		return
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      "TELEMETRY",
		Subjects:  []string{"telemetry.raw.>"},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Discard:   nats.DiscardOld,
		MaxAge:    24 * time.Hour,
		MaxBytes:  5 * 1024 * 1024 * 1024,
	})
	if err != nil {
		slog.Warn("ensure stream failed", "error", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
