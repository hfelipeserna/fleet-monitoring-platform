package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "fleetmonitoring/backend/internal/telemetry/adapters/http"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	js, nc, err := bootstrapNATS(cfg)
	if err != nil {
		slog.Error("bootstrap nats failed", "error", err)
		return
	}
	defer func() {
		if err := nc.Drain(); err != nil {
			slog.Error("nats drain failed", "error", err)
		}
	}()
	pool, err := bootstrapPool(ctx, cfg.databaseURL)
	if err != nil {
		slog.Error("bootstrap pool failed", "error", err)
		return
	}
	defer pool.Close()
	writer, dbBreaker, publishBreaker := bootstrapBreakers(pool)
	consumer, opts := bootstrapConsumer(writer, js)
	detector := bootstrapAlertDetector(pool, js)
	consumer.WithAlertProcessor(detector)
	dlqJS := bootstrapDLQ(js)
	dlqHandler := httpadapter.NewDLQHandler(dlqJS)
	go serveHealth(cfg.healthPort, dlqHandler, dbBreaker, publishBreaker)
	go consumeLoop(ctx, js, consumer, opts)
	<-ctx.Done()
	slog.Info("shutdown signal received")
	time.Sleep(2 * time.Second)
	slog.Info("consumer stopped")
}
