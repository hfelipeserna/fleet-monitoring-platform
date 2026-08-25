package pg

import (
	"context"
	"fmt"
	"time"

	"fleetmonitoring/backend/internal/assistant/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// breaker/timeout injected via Querier decorator; caller must wrap with context.WithTimeout 2s and gobreaker in assistant/adapters/genkit/tools.go
// postgis guard optional: if SELECT postgis_version() IS NULL -> return shared.ErrUnavailable (caller checks ErrUnavailable)

const stoppedQuery = `
SELECT DISTINCT ON (t.plate)
  t.plate, cz.id, cz.name, t.received_at,
  EXTRACT(EPOCH FROM (now() - t.received_at))/60,
  t.lat, t.lon
FROM telemetry t
JOIN critical_zones cz ON ST_Within(t.geom::geometry, cz.geom)
WHERE t.speed = 0
  AND t.received_at <= now() - ($1::int * interval '1 min')
  AND ($2::uuid IS NULL OR cz.id = $2::uuid)
ORDER BY t.plate, t.received_at DESC
LIMIT $3
`

type StoppedReader struct {
	pool Querier
}

func NewStoppedReader(pool Querier) *StoppedReader {
	return &StoppedReader{pool: pool}
}

func (r *StoppedReader) FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	clamped, err := r.validateAndClamp(minMinutes, zoneID, limit)
	if err != nil {
		return nil, err
	}
	rows, err := r.queryStopped(ctx, minMinutes, zoneID, clamped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := r.scanStoppedRows(rows, clamped)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *StoppedReader) validateAndClamp(minMinutes int, zoneID *string, limit int) (int, error) {
	return shared.ValidateStoppedParams(minMinutes, limit, zoneID)
}

func (r *StoppedReader) queryStopped(ctx context.Context, minMinutes int, zoneID *string, limit int) (Rows, error) {
	var zoneArg any // any boxing: nil interface -> SQL NULL for $2::uuid IS NULL handling; string value otherwise
	if zoneID != nil {
		zoneArg = *zoneID
	}
	rows, err := r.pool.Query(ctx, stoppedQuery, minMinutes, zoneArg, limit)
	if err != nil {
		return nil, fmt.Errorf("query stopped failed: %w", err)
	}
	return rows, nil
}

func (r *StoppedReader) scanStoppedRows(rows Rows, limit int) ([]domain.StoppedVehicle, error) {
	out := make([]domain.StoppedVehicle, 0, limit)
	for rows.Next() {
		var plate string
		var zid string
		var zname string
		var stoppedSince time.Time
		var durationMin float64
		var lat float64
		var lon float64
		if err := rows.Scan(&plate, &zid, &zname, &stoppedSince, &durationMin, &lat, &lon); err != nil {
			return nil, fmt.Errorf("scan stopped failed: %w", err)
		}
		v := domain.StoppedVehicle{
			Plate:        shared.Plate(plate),
			ZoneID:       zid,
			ZoneName:     zname,
			DurationMin:  int(durationMin),
			Lat:          lat,
			Lon:          lon,
			StoppedSince: stoppedSince,
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	if out == nil {
		out = []domain.StoppedVehicle{}
	}
	return out, nil
}
