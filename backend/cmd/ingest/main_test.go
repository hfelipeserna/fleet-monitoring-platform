//go:build !race

package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats-server/v2/server"
)

// Covers [SPEC-001: BR-006, FR-011]

func TestIngest_LoadConfig(t *testing.T) {
	t.Run("defaults when no env", func(t *testing.T) {
		// Arrange
		t.Setenv("NATS_URL", "")
		t.Setenv("HTTP_PORT", "")
		t.Setenv("NATS_MAX_PENDING", "")
		t.Setenv("JETSTREAM_MAX_BYTES", "")
		t.Setenv("PUBLISH_TIMEOUT", "")

		// Act
		cfg := loadConfig()

		// Assert
		if cfg.natsURL != "nats://localhost:4222" {
			t.Fatalf("expected default nats, got %q", cfg.natsURL)
		}
		if cfg.httpPort != "8080" {
			t.Fatalf("expected 8080, got %q", cfg.httpPort)
		}
		if cfg.natsMaxPending != 1024 {
			t.Fatalf("expected 1024, got %d", cfg.natsMaxPending)
		}
		if cfg.publishTimeout != 3*time.Second {
			t.Fatalf("expected 3s, got %v", cfg.publishTimeout)
		}
	})

	t.Run("env overrides", func(t *testing.T) {
		// Arrange
		t.Setenv("NATS_URL", "nats://custom:4222")
		t.Setenv("HTTP_PORT", "9090")
		t.Setenv("NATS_MAX_PENDING", "2048")
		t.Setenv("PUBLISH_TIMEOUT", "5s")

		// Act
		cfg := loadConfig()

		// Assert
		if cfg.natsURL != "nats://custom:4222" {
			t.Fatalf("got %q", cfg.natsURL)
		}
		if cfg.httpPort != "9090" {
			t.Fatalf("got %q", cfg.httpPort)
		}
		if cfg.natsMaxPending != 2048 {
			t.Fatalf("got %d", cfg.natsMaxPending)
		}
		if cfg.publishTimeout != 5*time.Second {
			t.Fatalf("got %v", cfg.publishTimeout)
		}
	})
}

func TestIngest_NewHTTPServer(t *testing.T) {
	t.Run("newHTTPServer sets Addr and timeouts", func(t *testing.T) {
		// Arrange
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		// Act
		srv := newHTTPServer(handler, "8080")

		// Assert
		if srv.Addr != ":8080" {
			t.Fatalf("expected :8080, got %q", srv.Addr)
		}
		if srv.ReadTimeout != 5*time.Second || srv.WriteTimeout != 10*time.Second || srv.IdleTimeout != 30*time.Second {
			t.Fatalf("unexpected timeouts %v %v %v", srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
		}
		if srv.Handler == nil {
			t.Fatalf("expected handler not nil")
		}
	})
}

func TestIngest_BootstrapServer(t *testing.T) {
	t.Run("bootstrapServer with nil js returns server and limiter", func(t *testing.T) {
		// Arrange
		cfg := config{natsURL: "nats://localhost:4222", httpPort: "8080", natsMaxPending: 1024, publishTimeout: 3 * time.Second}
		ctx := context.Background()
		var js nats.JetStreamContext = nil

		// Act
		srv, limiter := bootstrapServer(ctx, js, cfg)

		// Assert
		if srv == nil || limiter == nil {
			t.Fatalf("expected non-nil server and limiter")
		}
		if srv.Addr != ":8080" {
			t.Fatalf("expected :8080, got %q", srv.Addr)
		}
		limiter.Stop()
	})

	t.Run("bootstrapServer with different port", func(t *testing.T) {
		// Arrange
		cfg := config{httpPort: "9090", publishTimeout: 2 * time.Second}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Act
		srv, limiter := bootstrapServer(ctx, nil, cfg)

		// Assert
		if srv.Addr != ":9090" {
			t.Fatalf("got %q", srv.Addr)
		}
		limiter.Stop()
	})
}

func TestIngest_BootstrapNATS_Failure(t *testing.T) {
	t.Run("invalid NATS_URL fails quickly", func(t *testing.T) {
		// Arrange
		cfg := config{natsURL: "nats://invalid.invalid:4222", natsMaxPending: 1024, jetstreamMaxBytes: 5 << 30}

		// Act
		_, _, err := bootstrapNATS(cfg)

		// Assert
		if err == nil {
			t.Fatalf("expected error for invalid NATS")
		}
		if !containsIngest(err.Error(), "nats") {
			t.Fatalf("expected nats error, got %v", err)
		}
	})

	t.Run("valid NATS_URL with embedded server succeeds", func(t *testing.T) {
		// Arrange
		opts := &server.Options{Port: -1, JetStream: true}
		ns, err := server.NewServer(opts)
		if err != nil {
			t.Fatalf("failed to create server: %v", err)
		}
		go ns.Start()
		defer ns.Shutdown()
		if !ns.ReadyForConnections(5 * time.Second) {
			t.Fatalf("server not ready")
		}
		cfg := config{natsURL: ns.ClientURL(), natsMaxPending: 1024, jetstreamMaxBytes: 5 << 30}

		// Act
		js, nc, err := bootstrapNATS(cfg)

		// Assert
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if js == nil || nc == nil {
			t.Fatalf("expected js and nc not nil")
		}
		_ = nc.Drain()
		// also test bootstrapServer with real js
		ctx := context.Background()
		srv, limiter := bootstrapServer(ctx, js, cfg)
		if srv == nil || limiter == nil {
			t.Fatalf("expected server/limiter")
		}
		limiter.Stop()
	})

	t.Run("NATS without JetStream fails EnsureStream", func(t *testing.T) {
		// Arrange
		opts := &server.Options{Port: -1, JetStream: false}
		ns, err := server.NewServer(opts)
		if err != nil {
			t.Fatalf("failed to create server: %v", err)
		}
		go ns.Start()
		defer ns.Shutdown()
		if !ns.ReadyForConnections(5 * time.Second) {
			t.Fatalf("server not ready")
		}
		cfg := config{natsURL: ns.ClientURL(), natsMaxPending: 1024, jetstreamMaxBytes: 5 << 30}
		// Act
		_, _, err = bootstrapNATS(cfg)
		// Assert
		if err == nil {
			t.Fatalf("expected EnsureStream failure without JetStream")
		}
		if !containsIngest(err.Error(), "ensure") {
			t.Fatalf("expected ensure error, got %v", err)
		}
	})
}

func TestIngest_Main(t *testing.T) {
	t.Run("main with invalid NATS returns quickly", func(t *testing.T) {
		// Arrange
		t.Setenv("NATS_URL", "nats://invalid.invalid:4222")
		t.Setenv("HTTP_PORT", "18080")
		done := make(chan struct{})
		go func() {
			main()
			close(done)
		}()
		// Act
		select {
		case <-done:
			// Assert: main returned quickly due to bootstrap failure
		case <-time.After(4 * time.Second):
			t.Fatalf("main did not return within timeout, may be blocking on signal")
		}
	})
}

func containsIngest(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
