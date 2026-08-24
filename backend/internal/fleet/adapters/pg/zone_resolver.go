package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

type PGZoneResolver struct {
	db Querier
}

type ZoneResolver = PGZoneResolver

func NewZoneResolver(db Querier) *PGZoneResolver {
	return &PGZoneResolver{db: db}
}

func NewPGZoneResolver(db Querier) *PGZoneResolver {
	return &PGZoneResolver{db: db}
}

func (r *PGZoneResolver) IsInside(ctx context.Context, plate string, lat, lon float64) (*string, bool, error) {
	_ = plate // plate unused: zones global, kept for future tenant sharding
	sql := `SELECT id FROM critical_zones WHERE ST_Within(ST_SetSRID(ST_MakePoint($1,$2),4326)::geometry, geom) ORDER BY ST_Area(geom) ASC, id LIMIT 1`
	rows, err := r.db.Query(ctx, sql, lon, lat)
	if err != nil {
		return nil, false, fmt.Errorf("zone resolve query: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, false, fmt.Errorf("scan zone id: %w", err)
		}
		if err := rows.Err(); err != nil {
			return nil, false, fmt.Errorf("zone rows err: %w", err)
		}
		return &id, true, nil
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("zone rows err: %w", err)
	}
	return nil, false, nil
}

type zoneBreakerRecorder interface {
	State() gobreaker.State
	Execute(func() (interface{}, error)) (interface{}, error)
}

type ZoneResolverWithBreaker struct {
	inner   *PGZoneResolver
	breaker zoneBreakerRecorder
	timeout time.Duration
}

func NewZoneResolverWithBreaker(inner *PGZoneResolver, breaker zoneBreakerRecorder, timeout time.Duration) *ZoneResolverWithBreaker {
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &ZoneResolverWithBreaker{inner: inner, breaker: breaker, timeout: timeout}
}

func withBreaker[T any](ctx context.Context, breaker zoneBreakerRecorder, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if breaker != nil && breaker.State() == gobreaker.StateOpen {
		var zero T
		return zero, fmt.Errorf("breaker open: %w", gobreaker.ErrOpenState)
	}
	var out T
	var qerr error
	exec := func() (any, error) {
		var err error
		out, err = fn(ctxTimeout)
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

func (w *ZoneResolverWithBreaker) IsInside(ctx context.Context, plate string, lat, lon float64) (*string, bool, error) {
	type result struct {
		id     *string
		inside bool
	}
	res, err := withBreaker(ctx, w.breaker, w.timeout, func(c context.Context) (result, error) {
		zid, inside, e := w.inner.IsInside(c, plate, lat, lon)
		return result{id: zid, inside: inside}, e
	})
	if err != nil {
		return nil, false, err
	}
	return res.id, res.inside, nil
}
