package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type Server struct {
	httpServer *http.Server
	pool       *pgxpool.Pool
	nc         *nats.Conn
}

func NewServer(handler http.Handler, addr string, nc *nats.Conn, pool *pgxpool.Pool) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
			// WriteTimeout 0 for SSE (long-lived) — BR-006 ping 15s vs LB 60s.
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  120 * time.Second,
		},
		nc:   nc,
		pool: pool,
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http listen failed: %w", err)
		}
		close(errCh)
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	var firstErr error
	if err := s.httpServer.Shutdown(ctx); err != nil {
		firstErr = fmt.Errorf("http shutdown failed: %w", err)
	}
	if s.pool != nil {
		s.pool.Close()
	}
	if s.nc != nil {
		if err := s.nc.Drain(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("nats drain failed: %w", err)
		}
	}
	return firstErr
}
