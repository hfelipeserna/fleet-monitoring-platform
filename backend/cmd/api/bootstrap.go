package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

type zoneBreaker struct {
	svc     *fleetapp.ZoneService
	breaker *gobreaker.CircuitBreaker
	timeout time.Duration
}

func withBreaker[T any](ctx context.Context, breaker *gobreaker.CircuitBreaker, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if breaker != nil && breaker.State() == gobreaker.StateOpen {
		var zero T
		return zero, fmt.Errorf("breaker open: %w", gobreaker.ErrOpenState)
	}
	var out T
	var qerr error
	exec := func() (any, error) {
		var err error
		out, err = fn(ctx)
		return nil, err
	}
	if breaker != nil {
		_, qerr = breaker.Execute(exec)
	} else {
		_, qerr = exec()
	}
	if qerr != nil {
		var zero T
		return zero, qerr
	}
	return out, nil
}

func (z *zoneBreaker) Create(ctx context.Context, name string, coords [][]float64) (fleetdomain.Zone, error) {
	out, err := withBreaker(ctx, z.breaker, z.timeout, func(c context.Context) (fleetdomain.Zone, error) {
		return z.svc.Create(c, name, coords)
	})
	if err != nil {
		return fleetdomain.Zone{}, fmt.Errorf("create zone: %w", err)
	}
	return out, nil
}

func (z *zoneBreaker) List(ctx context.Context) ([]fleetdomain.Zone, error) {
	out, err := withBreaker(ctx, z.breaker, z.timeout, func(c context.Context) ([]fleetdomain.Zone, error) {
		return z.svc.List(c)
	})
	if err != nil {
		return nil, fmt.Errorf("list zones: %w", err)
	}
	return out, nil
}

func (z *zoneBreaker) Update(ctx context.Context, id string, name string, coords [][]float64) (fleetdomain.Zone, error) {
	out, err := withBreaker(ctx, z.breaker, z.timeout, func(c context.Context) (fleetdomain.Zone, error) {
		return z.svc.Update(c, id, name, coords)
	})
	if err != nil {
		return fleetdomain.Zone{}, fmt.Errorf("update zone: %w", err)
	}
	return out, nil
}

func (z *zoneBreaker) Delete(ctx context.Context, id string) error {
	_, err := withBreaker(ctx, z.breaker, z.timeout, func(c context.Context) (struct{}, error) {
		return struct{}{}, z.svc.Delete(c, id)
	})
	if err != nil {
		return fmt.Errorf("delete zone: %w", err)
	}
	return nil
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
	// separate breakers: fleet-read for queries and fleet-zone for writes isolate failure domains
	// zone failures must not trip read circuit and vice versa
	adapter := fleetpg.NewPgxPoolAdapter(pool)
	reader := fleetpg.NewReader(adapter)
	svc := fleetapp.NewQueryService(reader)
	readBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "fleet-read",
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
	zoneBreakerCB := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "fleet-zone",
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
	alertPublishBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
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
	zoneResolverBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
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
	_ = alertPublishBreaker
	_ = zoneResolverBreaker
	zoneRepo := fleetpg.NewZoneRepository(adapter)
	zoneSvc := fleetapp.NewZoneService(zoneRepo)
	zoneWrapped := &zoneBreaker{svc: zoneSvc, breaker: zoneBreakerCB, timeout: 2 * time.Second}
	zoneHandler := fleethttp.NewZoneHandler(zoneWrapped)
	q := &querier{svc: svc, breaker: readBreaker, timeout: 2 * time.Second}
	ops := &opsProvider{breaker: readBreaker, nc: nc, pool: pool}
	handler := fleethttp.NewHandler(q, ops)
	combined := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/zones" || strings.HasPrefix(r.URL.Path, "/api/zones/") {
			zoneHandler.ServeHTTP(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
	addr := ":" + apiPort
	srv := NewServer(combined, addr, nc, pool)
	return srv, nil
}
