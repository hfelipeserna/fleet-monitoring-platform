package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"fleetmonitoring/backend/internal/fleet/application"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type fakeZoneRepository struct {
	createFn func(ctx context.Context, z fleet.Zone) (fleet.Zone, error)
	listFn   func(ctx context.Context) ([]fleet.Zone, error)
	updateFn func(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error)
	deleteFn func(ctx context.Context, id string) error
}

func (f *fakeZoneRepository) Create(ctx context.Context, z fleet.Zone) (fleet.Zone, error) {
	if f.createFn != nil {
		return f.createFn(ctx, z)
	}
	return z, nil
}

func (f *fakeZoneRepository) List(ctx context.Context) ([]fleet.Zone, error) {
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return nil, nil
}

func (f *fakeZoneRepository) Update(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, z)
	}
	return z, nil
}

func (f *fakeZoneRepository) Delete(ctx context.Context, id string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

func newZoneService(repo *fakeZoneRepository) *application.ZoneService {
	// Expected: func NewZoneService(repo ZoneRepository) *ZoneService
	// Will fail RED until application/zone.go implements it.
	return application.NewZoneService(repo)
}

var _ = shared.ErrValidation

func validAppPolygon() [][]float64 {
	return [][]float64{
		{-74.07, 4.71},
		{-74.05, 4.71},
		{-74.05, 4.73},
		{-74.07, 4.73},
		{-74.07, 4.71},
	}
}

// Covers [SPEC-002: AC-003, AC-004, BR-002, BR-005, FR-003]
func TestZoneService(t *testing.T) {
	t.Run("Create Polygon cerrado 5 coords válido -> 201 con id UUID generado", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{
			createFn: func(ctx context.Context, z fleet.Zone) (fleet.Zone, error) {
				return fleet.Zone{ID: "550e8400-e29b-41d4-a716-446655440001", Name: z.Name, Coordinates: z.Coordinates}, nil
			},
		}
		svc := newZoneService(repo)
		ctx := context.Background()

		// Act
		created, err := svc.Create(ctx, "Zona Norte", validAppPolygon())

		// Assert
		if err != nil {
			t.Fatalf("expected no error for valid Polygon 5 coords, got %v", err)
		}
		if created.ID == "" {
			t.Fatalf("expected generated UUID id, got empty")
		}
		if len(created.ID) != 36 {
			t.Fatalf("expected UUID length 36, got %q", created.ID)
		}
		if created.Name != "Zona Norte" {
			t.Fatalf("expected name Zona Norte, got %q", created.Name)
		}
		if len(created.Coordinates) != 5 {
			t.Fatalf("expected 5 coords, got %d", len(created.Coordinates))
		}
	})

	t.Run("Create Polygon no cerrado first!=last -> 400 validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.06, 4.72},
		}

		// Act
		_, err := svc.Create(ctx, "Zona Norte", coords)

		// Assert
		if err == nil {
			t.Fatal("expected error for not closed polygon first!=last")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "valid") && !strings.Contains(err.Error(), "closed") {
			// Also accept wrapped ErrValidation
			if !isValidation(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
		}
	})

	t.Run("Create 3 coords -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}

		// Act
		_, err := svc.Create(ctx, "Zona Norte", coords)

		// Assert
		if err == nil {
			t.Fatal("expected error for 3 coords")
		}
		if !isValidation(err) {
			t.Fatalf("expected ErrValidation for 3 coords, got %v", err)
		}
	})

	t.Run("Create area 0 colineal 4 coords -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.03, 4.71},
			{-74.07, 4.71},
		}

		// Act
		_, err := svc.Create(ctx, "Zona Norte", coords)

		// Assert
		if err == nil {
			t.Fatal("expected error for zero area colineal")
		}
		if !isValidation(err) {
			t.Fatalf("expected ErrValidation for zero area, got %v", err)
		}
	})

	t.Run("Create 102 coords >101 -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		coords := make([][]float64, 102)
		for i := 0; i < 101; i++ {
			coords[i] = []float64{-74.07 + float64(i)*0.0001, 4.71 + float64(i)*0.0001}
		}
		coords[101] = coords[0]

		// Act
		_, err := svc.Create(ctx, "Zona Norte", coords)

		// Assert
		if err == nil {
			t.Fatal("expected error for 102 coords >101")
		}
		if !isValidation(err) {
			t.Fatalf("expected ErrValidation for >101 coords, got %v", err)
		}
	})

	t.Run("Create name blank / 101 runes -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		cases := []string{"", "   ", strings.Repeat("a", 101)}
		for _, name := range cases {
			// Act
			_, err := svc.Create(ctx, name, validAppPolygon())

			// Assert
			if err == nil {
				t.Fatalf("expected error for name %q blank/101 runes", name)
			}
			if !isValidation(err) {
				t.Fatalf("expected ErrValidation for name %q, got %v", name, err)
			}
		}
	})

	t.Run("Create self-intersect bowtie -> 400 ST_IsValid false", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}

		// Act
		_, err := svc.Create(ctx, "Zona Bowtie", coords)

		// Assert
		if err == nil {
			t.Fatal("expected error for self-intersect bowtie ST_IsValid false")
		}
		if !isValidation(err) {
			t.Fatalf("expected ErrValidation for bowtie, got %v", err)
		}
	})

	t.Run("List -> FeatureCollection zones", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-005, FR-003]
		// Arrange
		repo := &fakeZoneRepository{
			listFn: func(ctx context.Context) ([]fleet.Zone, error) {
				return []fleet.Zone{
					{ID: "550e8400-e29b-41d4-a716-446655440010", Name: "Zona Norte", Coordinates: validAppPolygon()},
				}, nil
			},
		}
		svc := newZoneService(repo)
		ctx := context.Background()

		// Act
		zones, err := svc.List(ctx)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for List, got %v", err)
		}
		if len(zones) != 1 {
			t.Fatalf("expected 1 zone, got %d", len(zones))
		}
		if zones[0].Name != "Zona Norte" {
			t.Fatalf("expected Zona Norte, got %q", zones[0].Name)
		}
	})

	t.Run("Update válido -> 200", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		id := "550e8400-e29b-41d4-a716-446655440003"
		repo := &fakeZoneRepository{
			updateFn: func(ctx context.Context, uid string, z fleet.Zone) (fleet.Zone, error) {
				return fleet.Zone{ID: uid, Name: z.Name, Coordinates: z.Coordinates}, nil
			},
		}
		svc := newZoneService(repo)
		ctx := context.Background()
		newCoords := [][]float64{
			{-74.08, 4.72},
			{-74.06, 4.72},
			{-74.06, 4.74},
			{-74.08, 4.74},
			{-74.08, 4.72},
		}

		// Act
		updated, err := svc.Update(ctx, id, "Zona Norte v2", newCoords)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for valid Update, got %v", err)
		}
		if updated.ID != id {
			t.Fatalf("expected id %q, got %q", id, updated.ID)
		}
		if updated.Name != "Zona Norte v2" {
			t.Fatalf("expected name Zona Norte v2, got %q", updated.Name)
		}
	})

	t.Run("Update id random UUID -> 404", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{
			updateFn: func(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) {
				return fleet.Zone{}, fmt.Errorf("not found")
			},
		}
		svc := newZoneService(repo)
		ctx := context.Background()

		// Act
		_, err := svc.Update(ctx, "550e8400-e29b-41d4-a716-446655440099", "Zona Norte v2", validAppPolygon())

		// Assert
		if err == nil {
			t.Fatal("expected 404 not found error for random UUID Update")
		}
		if !isNotFound(err) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Update geo inválido -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{}
		svc := newZoneService(repo)
		ctx := context.Background()
		invalidCoords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}

		// Act
		_, err := svc.Update(ctx, "550e8400-e29b-41d4-a716-446655440003", "Zona Norte v2", invalidCoords)

		// Assert
		if err == nil {
			t.Fatal("expected 400 validation for invalid geo Update")
		}
		if !isValidation(err) {
			t.Fatalf("expected ErrValidation for invalid geo Update, got %v", err)
		}
	})

	t.Run("Delete -> 204", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{
			deleteFn: func(ctx context.Context, id string) error { return nil },
		}
		svc := newZoneService(repo)
		ctx := context.Background()

		// Act
		err := svc.Delete(ctx, "550e8400-e29b-41d4-a716-446655440004")

		// Assert
		if err != nil {
			t.Fatalf("expected no error for Delete, got %v", err)
		}
	})

	t.Run("Delete random UUID -> 404", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		repo := &fakeZoneRepository{
			deleteFn: func(ctx context.Context, id string) error { return fmt.Errorf("not found") },
		}
		svc := newZoneService(repo)
		ctx := context.Background()

		// Act
		err := svc.Delete(ctx, "550e8400-e29b-41d4-a716-446655440099")

		// Assert
		if err == nil {
			t.Fatal("expected 404 error for DELETE random UUID")
		}
		if !isNotFound(err) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func isValidation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "valid") || isWrapped(err, shared.ErrValidation) || isWrapped(err, fleet.ErrValidation)
}

func isNotFound(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func isWrapped(err error, target error) bool {
	// naive wrapper check without errors.Is to avoid import cycle in RED template
	type unwrapper interface{ Unwrap() error }
	for err != nil {
		if err == target {
			return true
		}
		if uw, ok := err.(unwrapper); ok {
			err = uw.Unwrap()
		} else {
			break
		}
	}
	return false
}
