package application

import (
	"context"
	"testing"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// Covers [SPEC-002: AC-003, AC-004]

func TestCoverage_FleetApp(t *testing.T) {
	t.Run("CreateZone ListZones UpdateZone DeleteZone wrappers", func(t *testing.T) {
		// Arrange
		repo := &fakeZoneRepoApp{
			createFn: func(ctx context.Context, z fleet.Zone) (fleet.Zone, error) { return z, nil },
			listFn:   func(ctx context.Context) ([]fleet.Zone, error) { return []fleet.Zone{}, nil },
			updateFn: func(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) { return z, nil },
			deleteFn: func(ctx context.Context, id string) error { return nil },
		}
		svc := NewZoneService(repo)

		// Act
		coords := [][]float64{{-74.07, 4.71}, {-74.05, 4.71}, {-74.05, 4.73}, {-74.07, 4.73}, {-74.07, 4.71}}
		_, err1 := svc.CreateZone("Test", coords)
		_, err2 := svc.ListZones()
		_, err3 := svc.UpdateZone("550e8400-e29b-41d4-a716-446655440001", "Test2", coords)
		err4 := svc.DeleteZone("550e8400-e29b-41d4-a716-446655440001")

		// Assert
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			t.Fatalf("expected wrappers to succeed, got %v %v %v %v", err1, err2, err3, err4)
		}
	})

	t.Run("History and LastPositions with validation", func(t *testing.T) {
		// Arrange
		reader := &fakeReaderApp{
			lastFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return []fleet.VehiclePos{{Plate: "GTP890", Speed: 10, ReceivedAt: time.Now().UTC(), Status: "moving"}}, "", nil
			},
			historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return []fleet.VehiclePos{}, "", nil
			},
		}
		svc := NewQueryService(reader)
		// Act: History with valid and invalid
		_, _, err := svc.History(context.Background(), "GTP890", nil, nil, 5, "")
		plateStr := "GTP890"
		_, _, errLP := svc.LastPositions(context.Background(), &plateStr, 5, "")
		// Assert
		if err != nil {
			t.Fatalf("expected nil history, got %v", err)
		}
		if errLP != nil {
			t.Fatalf("expected nil lastpositions, got %v", errLP)
		}
		_, _, err2 := svc.History(context.Background(), "BAD", nil, nil, 5, "")
		if err2 == nil {
			t.Fatalf("expected error bad plate")
		}
	})

	t.Run("Delete validation", func(t *testing.T) {
		// Arrange
		repo := &fakeZoneRepoApp{}
		svc := NewZoneService(repo)
		// Act
		err := svc.Delete(context.Background(), "not-uuid")
		// Assert
		if err == nil {
			t.Fatalf("expected error invalid uuid")
		}
	})
}

type fakeZoneRepoApp struct {
	createFn func(context.Context, fleet.Zone) (fleet.Zone, error)
	listFn   func(context.Context) ([]fleet.Zone, error)
	updateFn func(context.Context, string, fleet.Zone) (fleet.Zone, error)
	deleteFn func(context.Context, string) error
}
func (f *fakeZoneRepoApp) Create(ctx context.Context, z fleet.Zone) (fleet.Zone, error) { if f.createFn != nil { return f.createFn(ctx, z) }; return z, nil }
func (f *fakeZoneRepoApp) List(ctx context.Context) ([]fleet.Zone, error) { if f.listFn != nil { return f.listFn(ctx) }; return nil, nil }
func (f *fakeZoneRepoApp) Update(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) { if f.updateFn != nil { return f.updateFn(ctx, id, z) }; return z, nil }
func (f *fakeZoneRepoApp) Delete(ctx context.Context, id string) error { if f.deleteFn != nil { return f.deleteFn(ctx, id) }; return nil }

type fakeReaderApp struct {
	lastFn    func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error)
	historyFn func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error)
}
func (f *fakeReaderApp) LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if f.lastFn != nil {
		return f.lastFn(ctx, plate, limit, cursor)
	}
	return nil, "", nil
}
func (f *fakeReaderApp) History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if f.historyFn != nil {
		return f.historyFn(ctx, plate, from, to, limit, cursor)
	}
	return nil, "", nil
}
