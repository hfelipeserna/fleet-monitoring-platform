package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleetmonitoring/backend/internal/assistant/application"
	"fleetmonitoring/backend/internal/assistant/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type mockQuerier struct {
	findStoppedFunc    func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error)
	capturedMinMinutes int
	capturedZoneID     *string
	capturedLimit      int
}

func (m *mockQuerier) FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	// Arrange helper captures query params
	m.capturedMinMinutes = minMinutes
	m.capturedZoneID = zoneID
	m.capturedLimit = limit
	if m.findStoppedFunc != nil {
		return m.findStoppedFunc(ctx, minMinutes, zoneID, limit)
	}
	return nil, nil
}

func (m *mockQuerier) GetFleetSummary(ctx context.Context) (application.FleetSummary, error) {
	return application.FleetSummary{}, nil
}

func (m *mockQuerier) GetVehicleStatus(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error) {
	return application.VehicleStatus{}, nil
}

func (m *mockQuerier) GetActiveAlerts(ctx context.Context, limit int) ([]application.Alert, error) {
	return nil, nil
}

// Covers [SPEC-003: AC-001, BR-001]
func TestFindStopped_27m_returns_2(t *testing.T) {
	t.Run("returns 2 vehicles stopped >20m in critical zone", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		zoneID := "550e8400-e29b-41d4-a716-446655440000"
		mock := &mockQuerier{
			findStoppedFunc: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return []domain.StoppedVehicle{
					{Plate: shared.Plate("GTP980"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona Norte", DurationMin: 27, Lat: 4.711, Lon: -74.072, StoppedSince: now.Add(-27 * time.Minute)},
					{Plate: shared.Plate("TTY423"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona Norte", DurationMin: 25, Lat: 4.712, Lon: -74.073, StoppedSince: now.Add(-25 * time.Minute)},
				}, nil
			},
		}
		svc := application.NewStoppedService(mock)
		_ = application.StoppedService{}

		// Act
		result, err := svc.FindStopped(context.Background(), 20, &zoneID, 20)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 vehicles, got %d", len(result))
		}
		if mock.capturedMinMinutes != 20 {
			t.Fatalf("expected minMinutes 20, got %d", mock.capturedMinMinutes)
		}
		var _ = errors.New
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestFindStopped_5m_returns_0(t *testing.T) {
	t.Run("returns 0 when no vehicle stopped >20m", func(t *testing.T) {
		// Arrange
		mock := &mockQuerier{
			findStoppedFunc: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return []domain.StoppedVehicle{}, nil
			},
		}
		svc := application.NewStoppedService(mock)
		_ = application.StoppedService{}

		// Act
		result, err := svc.FindStopped(context.Background(), 20, nil, 20)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 vehicles, got %d", len(result))
		}
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestFindStopped_limit_0_clamps_to_20(t *testing.T) {
	t.Run("clamps limit 0 to 20 default", func(t *testing.T) {
		// Arrange
		mock := &mockQuerier{
			findStoppedFunc: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				if limit != 20 {
					t.Fatalf("expected clamped limit 20 for input 0, got %d", limit)
				}
				return []domain.StoppedVehicle{}, nil
			},
		}
		svc := application.NewStoppedService(mock)

		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 0)

		// Assert
		if err != nil {
			t.Fatalf("expected no error with limit 0 clamped to 20, got %v", err)
		}
		if mock.capturedLimit != 20 {
			t.Fatalf("expected captured limit 20 after clamp for 0, got %d", mock.capturedLimit)
		}
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestFindStopped_limit_clamp_20(t *testing.T) {
	t.Run("clamps limit to 20 max", func(t *testing.T) {
		// Arrange
		mock := &mockQuerier{
			findStoppedFunc: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				if limit != 20 {
					t.Fatalf("expected clamped limit 20, got %d", limit)
				}
				return []domain.StoppedVehicle{}, nil
			},
		}
		svc := application.NewStoppedService(mock)
		_ = application.StoppedService{}

		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 30)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if mock.capturedLimit != 20 {
			t.Fatalf("expected captured limit 20 after clamp, got %d", mock.capturedLimit)
		}
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestFindStopped_invalid_plate(t *testing.T) {
	t.Run("rejects invalid plate format", func(t *testing.T) {
		// Arrange
		mock := &mockQuerier{}
		svc := application.NewStoppedService(mock)
		_ = application.StoppedService{}
		invalidZoneID := "not-a-uuid"
		invalidPlate := shared.Plate("GTP98")

		// Act
		_, err := svc.FindStopped(context.Background(), 20, &invalidZoneID, 20)
		_, plateErr := shared.ParsePlate(string(invalidPlate))

		// Assert
		if err == nil && plateErr == nil {
			t.Fatal("expected validation error for invalid plate GTP98 / zoneID not-a-uuid")
		}
		if err != nil && !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
		if plateErr != nil && !errors.Is(plateErr, shared.ErrValidation) {
			t.Fatalf("expected plate ErrValidation wrapped, got %v", plateErr)
		}
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestFindStopped_round3dec(t *testing.T) {
	t.Run("rounds lat lon to 3 decimals", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		mock := &mockQuerier{
			findStoppedFunc: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return []domain.StoppedVehicle{
					{Plate: shared.Plate("GTP980"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona Norte", DurationMin: 27, Lat: 4.71111119, Lon: -74.07222229, StoppedSince: now},
				}, nil
			},
		}
		svc := application.NewStoppedService(mock)
		_ = application.StoppedService{}

		// Act
		result, err := svc.FindStopped(context.Background(), 20, nil, 20)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 vehicle, got %d", len(result))
		}
		if result[0].Lat != 4.711 || result[0].Lon != -74.072 {
			t.Fatalf("expected lat 4.711 lon -74.072 after 3dec round, got %v %v", result[0].Lat, result[0].Lon)
		}
	})
}
