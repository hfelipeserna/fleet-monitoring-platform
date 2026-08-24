package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type ZoneDB interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
}

var ErrDuplicateZoneName = fleet.ErrDuplicateZoneName

type ZoneRepository struct {
	db ZoneDB
}

func NewZoneRepository(db ZoneDB) *ZoneRepository {
	return &ZoneRepository{db: db}
}

func isDuplicateZoneError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "critical_zones_name_unique") || strings.Contains(pgErr.Message, "critical_zones_name_unique") {
			return true
		}
	}
	return false
}

func (a *PgxPoolAdapter) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := a.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("exec failed: %w", err)
	}
	return tag.RowsAffected(), nil
}

func mapCheckError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "23") {
		return fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, fleet.ErrInvalidPolygon, err))
	}
	return nil
}

func (r *ZoneRepository) Create(ctx context.Context, z fleet.Zone) (fleet.Zone, error) {
	if err := z.Validate(); err != nil {
		return fleet.Zone{}, fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, err))
	}
	geo := map[string]any{
		"type":        "Polygon",
		"coordinates": [][][]float64{z.Coordinates},
	}
	b, err := json.Marshal(geo)
	if err != nil {
		return fleet.Zone{}, fmt.Errorf("marshal geojson failed: %w", err)
	}
	sql := `INSERT INTO critical_zones (id, name, geom) VALUES ($1, $2, ST_SetSRID(ST_GeomFromGeoJSON($3),4326))`
	tag, err := r.db.Exec(ctx, sql, z.ID, z.Name, string(b))
	if err != nil {
		if isDuplicateZoneError(err) {
			return fleet.Zone{}, fmt.Errorf("duplicate zone name: %w", errors.Join(shared.ErrValidation, ErrDuplicateZoneName, err))
		}
		if ve := mapCheckError(err); ve != nil {
			return fleet.Zone{}, ve
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fleet.Zone{}, fmt.Errorf("zone %s not found: %w", z.ID, errors.Join(fleet.ErrNotFound, err))
		}
		return fleet.Zone{}, fmt.Errorf("insert zone failed: %w", err)
	}
	if tag == 0 {
		return fleet.Zone{}, fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, fleet.ErrInvalidPolygon, fmt.Errorf("invalid geometry")))
	}
	return z, nil
}

func (r *ZoneRepository) List(ctx context.Context) ([]fleet.Zone, error) {
	sql := `SELECT id, name, ST_AsGeoJSON(geom) FROM critical_zones`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list zones query failed: %w", err)
	}
	defer rows.Close()
	var out []fleet.Zone
	for rows.Next() {
		var id, name string
		var geoJSONStr string
		if err := rows.Scan(&id, &name, &geoJSONStr); err != nil {
			return nil, fmt.Errorf("scan zone failed: %w", err)
		}
		if geoJSONStr == "" {
			continue
		}
		var gj struct {
			Type        string          `json:"type"`
			Coordinates [][][]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal([]byte(geoJSONStr), &gj); err != nil {
			return nil, fmt.Errorf("unmarshal geojson failed: %w", err)
		}
		var coords [][]float64
		if len(gj.Coordinates) > 0 {
			coords = gj.Coordinates[0]
			for i := range coords {
				for j := range coords[i] {
					coords[i][j] = shared.Round6(coords[i][j])
				}
			}
		}
		z := fleet.Zone{ID: id, Name: name, Coordinates: coords}
		out = append(out, z)
	}
	if out == nil {
		out = []fleet.Zone{}
	}
	return out, nil
}

func (r *ZoneRepository) Update(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) {
	if z.ID != id {
		z.ID = id
	}
	if err := z.Validate(); err != nil {
		return fleet.Zone{}, fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, err))
	}
	geo := map[string]any{
		"type":        "Polygon",
		"coordinates": [][][]float64{z.Coordinates},
	}
	b, err := json.Marshal(geo)
	if err != nil {
		return fleet.Zone{}, fmt.Errorf("marshal geojson failed: %w", err)
	}
	sql := `UPDATE critical_zones SET name=$2, geom=ST_SetSRID(ST_GeomFromGeoJSON($3),4326) WHERE id=$1`
	tag, err := r.db.Exec(ctx, sql, id, z.Name, string(b))
	if err != nil {
		if isDuplicateZoneError(err) {
			return fleet.Zone{}, fmt.Errorf("duplicate zone name: %w", errors.Join(shared.ErrValidation, ErrDuplicateZoneName, err))
		}
		if ve := mapCheckError(err); ve != nil {
			return fleet.Zone{}, ve
		}
		if errors.Is(err, fleet.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fleet.Zone{}, fmt.Errorf("zone %s not found: %w", id, errors.Join(fleet.ErrNotFound, err))
		}
		return fleet.Zone{}, fmt.Errorf("update zone failed: %w", err)
	}
	if tag == 0 {
		return fleet.Zone{}, fmt.Errorf("zone %s not found: %w", id, errors.Join(fleet.ErrNotFound, fmt.Errorf("not found")))
	}
	return z, nil
}

func (r *ZoneRepository) Delete(ctx context.Context, id string) error {
	sql := `DELETE FROM critical_zones WHERE id=$1`
	tag, err := r.db.Exec(ctx, sql, id)
	if err != nil {
		if errors.Is(err, fleet.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fmt.Errorf("zone %s not found: %w", id, errors.Join(fleet.ErrNotFound, err))
		}
		return fmt.Errorf("delete zone failed: %w", err)
	}
	if tag == 0 {
		return fmt.Errorf("zone %s not found: %w", id, errors.Join(fleet.ErrNotFound, fmt.Errorf("not found")))
	}
	return nil
}

func (r *ZoneRepository) Count(ctx context.Context) (int, error) {
	rows, err := r.db.Query(ctx, `SELECT count(*) FROM critical_zones`)
	if err != nil {
		return 0, fmt.Errorf("count zones: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("count zones: %w", err)
		}
		return 0, nil
	}
	var n int
	if err := rows.Scan(&n); err != nil {
		return 0, fmt.Errorf("scan count: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("count zones: %w", err)
	}
	return n, nil
}
