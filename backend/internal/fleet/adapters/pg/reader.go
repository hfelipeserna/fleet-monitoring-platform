package pg

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

type Reader struct {
	db Querier
}

func NewReader(db Querier) *Reader {
	return &Reader{db: db}
}

type PgxPoolAdapter struct {
	pool *pgxpool.Pool
}

func NewPgxPoolAdapter(pool *pgxpool.Pool) *PgxPoolAdapter {
	return &PgxPoolAdapter{pool: pool}
}

func (a *PgxPoolAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &pgxRowsAdapter{rows: rows}, nil
}

type pgxRowsAdapter struct {
	rows pgx.Rows
}

func (r *pgxRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *pgxRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *pgxRowsAdapter) Close()                 { r.rows.Close() }
func (r *pgxRowsAdapter) Err() error             { return r.rows.Err() }

func decodeCursor(cursor string) (string, time.Time, error) {
	p, t, err := shared.DecodeCursor(cursor)
	if err != nil {
		return "", time.Time{}, err
	}
	return string(p), t, nil
}

func encodeCursor(plate string, t time.Time) string {
	raw := fmt.Sprintf("%s|%s", plate, t.Format(time.RFC3339Nano))
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func (r *Reader) LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if limit < 1 || limit > 500 {
		return nil, "", fmt.Errorf("limit %d out of range 1..500: %w", limit, shared.ErrValidation)
	}
	if cursor != "" {
		if _, _, err := decodeCursor(cursor); err != nil {
			return nil, "", err
		}
	}
	sql := "SELECT DISTINCT ON (plate) plate, lat, lon, speed, received_at FROM telemetry"
	args := []any{}
	where := []string{}
	if plate != nil {
		where = append(where, fmt.Sprintf("plate = $%d", len(args)+1))
		args = append(args, string(*plate))
	}
	if cursor != "" {
		decoded, _ := base64.StdEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(decoded), "|", 2)
		cPlate := parts[0]
		cTimeStr := parts[1]
		cTime, _ := time.Parse(time.RFC3339Nano, cTimeStr)
		where = append(where, fmt.Sprintf("(plate > $%d OR (plate = $%d AND received_at < $%d))", len(args)+1, len(args)+1, len(args)+2))
		args = append(args, cPlate, cTime)
	}
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += " ORDER BY plate ASC, received_at DESC"
	sql += fmt.Sprintf(" LIMIT $%d", len(args)+1)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query LastPositions failed: %w", err)
	}
	defer rows.Close()
	var out []fleet.VehiclePos
	for rows.Next() {
		var plateStr string
		var lat, lon *float64
		var speed int
		var receivedAt time.Time
		if err := rows.Scan(&plateStr, &lat, &lon, &speed, &receivedAt); err != nil {
			return nil, "", fmt.Errorf("scan LastPositions failed: %w", err)
		}
		if lat != nil {
			v := shared.Round6(*lat)
			lat = &v
		}
		if lon != nil {
			v := shared.Round6(*lon)
			lon = &v
		}
		if _, err := shared.ParsePlate(plateStr); err != nil {
			continue
		}
		status := "idle"
		if speed > 0 {
			status = "moving"
		}
		out = append(out, fleet.VehiclePos{Plate: plateStr, Lat: lat, Lon: lon, Speed: speed, ReceivedAt: receivedAt, Status: status})
	}
	if len(out) > limit {
		last := out[limit-1]
		next := encodeCursor(last.Plate, last.ReceivedAt)
		out = out[:limit]
		return out, next, nil
	}
	return out, "", nil
}

func (r *Reader) History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if limit < 1 || limit > 500 {
		return nil, "", fmt.Errorf("limit %d out of range 1..500: %w", limit, shared.ErrValidation)
	}
	if cursor != "" {
		if _, _, err := decodeCursor(cursor); err != nil {
			return nil, "", err
		}
		decoded, _ := base64.StdEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(decoded), "|", 2)
		if parts[0] != string(plate) {
			return nil, "", fmt.Errorf("cursor plate mismatch: %w", shared.ErrValidation)
		}
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, "", fmt.Errorf("from after to: %w", shared.ErrValidation)
	}
	sql := "SELECT plate, lat, lon, speed, received_at FROM telemetry WHERE plate=$1"
	args := []any{string(plate)}
	idx := 2
	if from != nil {
		sql += fmt.Sprintf(" AND received_at >= $%d", idx)
		args = append(args, *from)
		idx++
	}
	if to != nil {
		sql += fmt.Sprintf(" AND received_at <= $%d", idx)
		args = append(args, *to)
		idx++
	}
	if cursor != "" {
		decoded, _ := base64.StdEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(decoded), "|", 2)
		cTime, _ := time.Parse(time.RFC3339Nano, parts[1])
		sql += fmt.Sprintf(" AND received_at < $%d", idx)
		args = append(args, cTime)
		idx++
	}
	sql += " ORDER BY received_at DESC"
	sql += fmt.Sprintf(" LIMIT $%d", idx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query History failed: %w", err)
	}
	defer rows.Close()
	var out []fleet.VehiclePos
	for rows.Next() {
		var plateStr string
		var lat, lon *float64
		var speed int
		var receivedAt time.Time
		if err := rows.Scan(&plateStr, &lat, &lon, &speed, &receivedAt); err != nil {
			return nil, "", fmt.Errorf("scan History failed: %w", err)
		}
		if lat != nil {
			v := shared.Round6(*lat)
			lat = &v
		}
		if lon != nil {
			v := shared.Round6(*lon)
			lon = &v
		}
		status := "idle"
		if speed > 0 {
			status = "moving"
		}
		out = append(out, fleet.VehiclePos{Plate: plateStr, Lat: lat, Lon: lon, Speed: speed, ReceivedAt: receivedAt, Status: status})
	}
	if len(out) > limit {
		last := out[limit-1]
		next := encodeCursor(last.Plate, last.ReceivedAt)
		out = out[:limit]
		return out, next, nil
	}
	return out, "", nil
}
