package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	natsMaxPending := envOrInt("NATS_MAX_PENDING", 1024)
	jetstreamMaxBytes := envOrInt64("JETSTREAM_MAX_BYTES", 5*1024*1024*1024)
	publishTimeout := envOrDuration("PUBLISH_TIMEOUT", 3*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		slog.Error("nats connect failed", "error", err, "url", natsURL)
		return
	}
	defer func() {
		if err := nc.Drain(); err != nil {
			slog.Error("nats drain failed", "error", err)
		}
	}()

	js, err := nc.JetStream(nats.PublishAsyncMaxPending(natsMaxPending))
	if err != nil {
		slog.Error("jetstream context failed", "error", err)
		return
	}
	ensureStream(js, jetstreamMaxBytes)

	brk := breaker.NewBreaker()
	pub := natsadapter.NewPublisherWithBreaker(js, publishTimeout, brk)
	limiter := rate.NewLimiterWithContext(ctx)
	defer limiter.Stop()
	jsInfo := jetstream.NewInfo(js, "TELEMETRY")

	handler := httpadapter.NewHandler(pub, limiter, brk, jsInfo)

	srv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		slog.Info("ingest listening", "port", httpPort, "nats", natsURL, "max_pending", natsMaxPending, "max_bytes", jetstreamMaxBytes, "publish_timeout", publishTimeout)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http serve failed", "error", err)
			stop()
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

func ensureStream(js nats.JetStreamContext, maxBytes int64) {
	_, err := js.StreamInfo("TELEMETRY")
	if err == nil {
		return
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:       "TELEMETRY",
		Subjects:   []string{"telemetry.raw.>"},
		Storage:    nats.FileStorage,
		Retention:  nats.LimitsPolicy,
		Discard:    nats.DiscardOld,
		MaxAge:     24 * time.Hour,
		MaxBytes:   maxBytes,
		Duplicates: 24 * time.Hour,
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

func envOrInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid env int, using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return n
}

func envOrInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		slog.Warn("invalid env int64, using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return n
}

func envOrDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid env duration, using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return d
}
