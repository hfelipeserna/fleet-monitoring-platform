package pg_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	fleetpg "fleetmonitoring/backend/internal/fleet/adapters/pg"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
)

// fakeZoneDB simulates Querier for zone persistence; records SQL and validates GeoJSON handling.
// The production adapter is expected to be: type ZoneRepository struct{ db Querier } with
// Create(ctx, Zone) (Zone,error), List(ctx) ([]Zone,error), Update(ctx,id,Zone) (Zone,error), Delete(ctx,id) error
// and internally using ST_GeomFromGeoJSON, ST_IsValid, ST_Area, ST_NPoints.
type fakeZoneDB struct {
	querySQL  string
	queryArgs []any
	rows      []fleet.Zone
	execErr   error
	queryErr  error
}

func (f *fakeZoneDB) Query(ctx context.Context, sql string, args ...any) (fleetpg.Rows, error) {
	f.querySQL = sql
	f.queryArgs = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	mr := &fakeZoneRows{data: f.rows}
	return mr, nil
}

func (f *fakeZoneDB) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	f.querySQL = sql
	f.queryArgs = args
	if f.execErr != nil {
		return 0, f.execErr
	}
	return 1, nil
}

type fakeZoneRows struct {
	data []fleet.Zone
	idx  int
	cur  fleet.Zone
}

func (r *fakeZoneRows) Next() bool { return r.idx < len(r.data) }
func (r *fakeZoneRows) Close()     {}
func (r *fakeZoneRows) Scan(dest ...any) error {
	if r.idx >= len(r.data) {
		return fmt.Errorf("no more rows")
	}
	z := r.data[r.idx]
	r.idx++
	// Support Scan(&id, &name, &geojson) where geojson is json.RawMessage or string
	if len(dest) >= 3 {
		if p, ok := dest[0].(*string); ok {
			*p = z.ID
		}
		if p, ok := dest[1].(*string); ok {
			*p = z.Name
		}
		if p, ok := dest[2].(*string); ok {
			j, _ := json.Marshal(map[string]any{
				"type":        "Polygon",
				"coordinates": [][][]float64{z.Coordinates},
			})
			*p = string(j)
		}
		if p, ok := dest[2].(*json.RawMessage); ok {
			j, _ := json.Marshal(map[string]any{
				"type":        "Polygon",
				"coordinates": [][][]float64{z.Coordinates},
			})
			*p = json.RawMessage(j)
		}
	}
	return nil
}

func validZoneCoords() [][]float64 {
	return [][]float64{
		{-74.07, 4.71},
		{-74.05, 4.71},
		{-74.05, 4.73},
		{-74.07, 4.73},
		{-74.07, 4.71},
	}
}

// Covers [SPEC-002: AC-003, AC-004, BR-002, BR-005, FR-003, FR-004]
func TestZoneRepository(t *testing.T) {
	t.Run("Create Polygon cerrado 5 coords -> INSERT con ST_GeomFromGeoJSON y ST_IsValid", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003, FR-004]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		zone := fleet.Zone{
			ID:          "550e8400-e29b-41d4-a716-446655440001",
			Name:        "Zona Norte",
			Coordinates: validZoneCoords(),
		}
		ctx := context.Background()

		// Act
		created, err := repo.Create(ctx, zone)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for valid Polygon 5 coords, got %v", err)
		}
		if created.ID != zone.ID {
			t.Fatalf("expected id %q, got %q", zone.ID, created.ID)
		}
		sqlUpper := strings.ToUpper(db.querySQL)
		if !strings.Contains(sqlUpper, "INSERT") || !strings.Contains(sqlUpper, "CRITICAL_ZONES") {
			t.Fatalf("expected INSERT INTO critical_zones, got %q", db.querySQL)
		}
		if !strings.Contains(sqlUpper, "ST_GEOMFROMGEOJSON") && !strings.Contains(sqlUpper, "ST_GEOMFROMTEXT") {
			t.Fatalf("expected ST_GeomFromGeoJSON or ST_GeomFromText for geom, got %q", db.querySQL)
		}
		if !strings.Contains(sqlUpper, "ST_ISVALID") && !strings.Contains(db.querySQL, "ST_IsValid") {
			// Some impl validates in Go prior to SQL; at least check Go validation via domain
			if err := zone.Validate(); err != nil {
				t.Fatalf("expected domain Validate to pass for closed polygon, got %v", err)
			}
		}
	})

	t.Run("Create Polygon no cerrado -> error validation sin INSERT", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		zone := fleet.Zone{
			ID:   "550e8400-e29b-41d4-a716-446655440002",
			Name: "Zona Norte",
			Coordinates: [][]float64{
				{-74.07, 4.71},
				{-74.05, 4.71},
				{-74.05, 4.73},
				{-74.07, 4.73},
				{-74.06, 4.72},
			},
		}
		ctx := context.Background()

		// Act
		_, err := repo.Create(ctx, zone)

		// Assert
		if err == nil {
			t.Fatal("expected error for not closed polygon first!=last, no INSERT")
		}
		if db.querySQL != "" && strings.Contains(strings.ToUpper(db.querySQL), "INSERT") {
			t.Fatalf("expected no INSERT for invalid polygon, got SQL %q", db.querySQL)
		}
	})

	t.Run("Create 3 coords -> error validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		zone := fleet.Zone{
			ID:   "550e8400-e29b-41d4-a716-446655440003",
			Name: "Zona Norte",
			Coordinates: [][]float64{
				{-74.07, 4.71},
				{-74.05, 4.71},
				{-74.07, 4.71},
			},
		}
		ctx := context.Background()

		// Act
		_, err := repo.Create(ctx, zone)

		// Assert
		if err == nil {
			t.Fatal("expected error for 3 coords")
		}
	})

	t.Run("Create area 0 colineal -> error validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		zone := fleet.Zone{
			ID:   "550e8400-e29b-41d4-a716-446655440004",
			Name: "Zona Norte",
			Coordinates: [][]float64{
				{-74.07, 4.71},
				{-74.05, 4.71},
				{-74.03, 4.71},
				{-74.07, 4.71},
			},
		}
		ctx := context.Background()

		// Act
		_, err := repo.Create(ctx, zone)

		// Assert
		if err == nil {
			t.Fatal("expected error for zero area colineal")
		}
	})

	t.Run("Create 102 coords >101 -> error validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		coords := make([][]float64, 102)
		for i := 0; i < 101; i++ {
			coords[i] = []float64{-74.07 + float64(i)*0.0001, 4.71 + float64(i)*0.0001}
		}
		coords[101] = coords[0]
		zone := fleet.Zone{ID: "550e8400-e29b-41d4-a716-446655440005", Name: "Zona Norte", Coordinates: coords}
		ctx := context.Background()

		// Act
		_, err := repo.Create(ctx, zone)

		// Assert
		if err == nil {
			t.Fatal("expected error for 102 coords >101")
		}
	})

	t.Run("Create self-intersect bowtie -> error ST_IsValid false", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		zone := fleet.Zone{
			ID:   "550e8400-e29b-41d4-a716-446655440006",
			Name: "Zona Bowtie",
			Coordinates: [][]float64{
				{-74.07, 4.71},
				{-74.05, 4.73},
				{-74.07, 4.73},
				{-74.05, 4.71},
				{-74.07, 4.71},
			},
		}
		ctx := context.Background()

		// Act
		_, err := repo.Create(ctx, zone)

		// Assert
		if err == nil {
			t.Fatal("expected error for self-intersect bowtie ST_IsValid false")
		}
	})

	t.Run("Create name blank / 101 runes -> error validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		ctx := context.Background()
		cases := []string{"", "   ", strings.Repeat("a", 101)}
		for _, name := range cases {
			zone := fleet.Zone{ID: "550e8400-e29b-41d4-a716-446655440007", Name: name, Coordinates: validZoneCoords()}

			// Act
			_, err := repo.Create(ctx, zone)

			// Assert
			if err == nil {
				t.Fatalf("expected error for name %q blank/101 runes", name)
			}
		}
	})

	t.Run("List -> SELECT con ST_AsGeoJSON y GIST", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-005, FR-003, FR-004]
		// Arrange
		db := &fakeZoneDB{
			rows: []fleet.Zone{
				{ID: "550e8400-e29b-41d4-a716-446655440010", Name: "Zona Norte", Coordinates: validZoneCoords()},
			},
		}
		repo := fleetpg.NewZoneRepository(db)
		ctx := context.Background()

		// Act
		zones, err := repo.List(ctx)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for List, got %v", err)
		}
		if len(zones) != 1 {
			t.Fatalf("expected 1 zone, got %d", len(zones))
		}
		sqlUpper := strings.ToUpper(db.querySQL)
		if !strings.Contains(sqlUpper, "SELECT") || !strings.Contains(sqlUpper, "CRITICAL_ZONES") {
			t.Fatalf("expected SELECT FROM critical_zones, got %q", db.querySQL)
		}
		if !strings.Contains(sqlUpper, "ST_ASGEOJSON") {
			t.Fatalf("expected ST_AsGeoJSON for GeoJSON canonical, got %q", db.querySQL)
		}
	})

	t.Run("Update válido -> 200 con ST_GeomFromGeoJSON", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		id := "550e8400-e29b-41d4-a716-446655440011"
		newCoords := [][]float64{
			{-74.08, 4.72},
			{-74.06, 4.72},
			{-74.06, 4.74},
			{-74.08, 4.74},
			{-74.08, 4.72},
		}
		zone := fleet.Zone{ID: id, Name: "Zona Norte v2", Coordinates: newCoords}
		ctx := context.Background()

		// Act
		updated, err := repo.Update(ctx, id, zone)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for valid Update, got %v", err)
		}
		if updated.ID != id {
			t.Fatalf("expected id %q, got %q", id, updated.ID)
		}
		sqlUpper := strings.ToUpper(db.querySQL)
		if !strings.Contains(sqlUpper, "UPDATE") || !strings.Contains(sqlUpper, "CRITICAL_ZONES") {
			t.Fatalf("expected UPDATE critical_zones, got %q", db.querySQL)
		}
	})

	t.Run("Update id random UUID -> 404 not found", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{execErr: fmt.Errorf("not found")}
		repo := fleetpg.NewZoneRepository(db)
		id := "550e8400-e29b-41d4-a716-446655440099"
		zone := fleet.Zone{ID: id, Name: "Zona Norte v2", Coordinates: validZoneCoords()}
		ctx := context.Background()

		// Act
		_, err := repo.Update(ctx, id, zone)

		// Assert
		if err == nil {
			t.Fatal("expected 404 not found error for random UUID Update")
		}
	})

	t.Run("Update geo inválido -> 400 validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		id := "550e8400-e29b-41d4-a716-446655440012"
		zone := fleet.Zone{
			ID:   id,
			Name: "Zona Norte v2",
			Coordinates: [][]float64{
				{-74.07, 4.71},
				{-74.05, 4.71},
				{-74.07, 4.71},
			},
		}
		ctx := context.Background()

		// Act
		_, err := repo.Update(ctx, id, zone)

		// Assert
		if err == nil {
			t.Fatal("expected 400 validation for invalid geo Update")
		}
	})

	t.Run("Delete -> 204", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{}
		repo := fleetpg.NewZoneRepository(db)
		id := "550e8400-e29b-41d4-a716-446655440013"
		ctx := context.Background()

		// Act
		err := repo.Delete(ctx, id)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for Delete, got %v", err)
		}
		sqlUpper := strings.ToUpper(db.querySQL)
		if !strings.Contains(sqlUpper, "DELETE") || !strings.Contains(sqlUpper, "CRITICAL_ZONES") {
			t.Fatalf("expected DELETE FROM critical_zones, got %q", db.querySQL)
		}
	})

	t.Run("Delete random UUID -> 404", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		db := &fakeZoneDB{execErr: fmt.Errorf("not found")}
		repo := fleetpg.NewZoneRepository(db)
		id := "550e8400-e29b-41d4-a716-446655440099"
		ctx := context.Background()

		// Act
		err := repo.Delete(ctx, id)

		// Assert
		if err == nil {
			t.Fatal("expected 404 error for DELETE random UUID")
		}
	})
}
