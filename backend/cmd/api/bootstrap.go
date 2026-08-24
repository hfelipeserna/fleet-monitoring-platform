package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker"

	fleetapp "fleetmonitoring/backend/internal/fleet/application"
	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
	fleetpg "fleetmonitoring/backend/internal/fleet/adapters/pg"
	fleetdomain "fleetmonitoring/backend/internal/fleet/domain"
	fleetenv "fleetmonitoring/backend/internal/fleet/infra/env"
	fleetnats "fleetmonitoring/backend/internal/fleet/infra/nats"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type querier struct {
	svc     *fleetapp.QueryService
	breaker *gobreaker.CircuitBreaker
	timeout time.Duration
}

func (q *querier) LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleetdomain.VehiclePos, string, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	if q.breaker != nil && q.breaker.State() == gobreaker.StateOpen {
		return nil, "", fmt.Errorf("breaker open: %w", gobreaker.ErrOpenState)
	}
	var (
		out  []fleetdomain.VehiclePos
		next string
		qerr error
	)
	exec := func() (any, error) {
		var plateStr *string
		if plate != nil {
			s := string(*plate)
			plateStr = &s
		}
		var err error
		out, next, err = q.svc.LastPositions(ctx, plateStr, limit, cursor)
		return nil, err
	}
	if q.breaker != nil {
		_, qerr = q.breaker.Execute(exec)
	} else {
		_, qerr = exec()
	}
	if qerr != nil {
		return nil, "", fmt.Errorf("query last positions: %w", qerr)
	}
	return out, next, nil
}

func (q *querier) History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleetdomain.VehiclePos, string, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	if q.breaker != nil && q.breaker.State() == gobreaker.StateOpen {
		return nil, "", fmt.Errorf("breaker open: %w", gobreaker.ErrOpenState)
	}
	var (
		out  []fleetdomain.VehiclePos
		next string
		qerr error
	)
	exec := func() (any, error) {
		var err error
		out, next, err = q.svc.History(ctx, string(plate), from, to, limit, cursor)
		return nil, err
	}
	if q.breaker != nil {
		_, qerr = q.breaker.Execute(exec)
	} else {
		_, qerr = exec()
	}
	if qerr != nil {
		return nil, "", fmt.Errorf("query history: %w", qerr)
	}
	return out, next, nil
}

type opsProvider struct {
	breaker *gobreaker.CircuitBreaker
	nc      *nats.Conn
	pool    *pgxpool.Pool
}

func (o *opsProvider) BreakerState() string {
	if o.breaker == nil {
		return "closed"
	}
	switch o.breaker.State() {
	case gobreaker.StateOpen:
		return "open"
	case gobreaker.StateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

func (o *opsProvider) NATSConnected() bool {
	if o.nc == nil {
		return false
	}
	return o.nc.IsConnected()
}

func (o *opsProvider) DBPoolStat() string {
	if o.pool == nil {
		return "unknown"
	}
	s := o.pool.Stat()
	return fmt.Sprintf("total=%d idle=%d", s.TotalConns(), s.IdleConns())
}

func Bootstrap(ctx context.Context) (*Server, error) {
	databaseURL := fleetenv.GetDatabaseURL()
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set: %w", fmt.Errorf("missing env"))
	}
	natsURL := fleetenv.GetNATSURL()
	apiPort := fleetenv.GetAPIPort()

	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		return nil, fmt.Errorf("nats connect failed: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("jetstream failed: %w", err)
	}
	if err := fleetnats.EnsureStream(ctx, js); err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("ensure alerts stream failed: %w", err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("pgxpool failed: %w", err)
	}
	adapter := fleetpg.NewPgxPoolAdapter(pool)
	reader := fleetpg.NewReader(adapter)
	svc := fleetapp.NewQueryService(reader)
	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "fleet-api",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < 10 {
				return false
			}
			return float64(c.TotalFailures)/float64(c.Requests) >= 0.5
		},
	})
	q := &querier{svc: svc, breaker: breaker, timeout: 2 * time.Second}
	ops := &opsProvider{breaker: breaker, nc: nc, pool: pool}
	handler := fleethttp.NewHandler(q, ops)
	addr := ":" + apiPort
	srv := NewServer(handler, addr, nc, pool)
	return srv, nil
}
