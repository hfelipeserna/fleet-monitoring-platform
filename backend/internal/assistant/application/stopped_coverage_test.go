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

// controllableQuerier allows per-method controllable returns for coverage.
type controllableQuerier struct {
	findFn    func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error)
	summaryFn func(ctx context.Context) (application.FleetSummary, error)
	statusFn  func(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error)
	alertsFn  func(ctx context.Context, limit int) ([]application.Alert, error)

	capturedMin   int
	capturedZone  *string
	capturedLimit int
	capturedAlertLimit int
}

func (c *controllableQuerier) FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	// Arrange helper
	c.capturedMin = minMinutes
	c.capturedZone = zoneID
	c.capturedLimit = limit
	if c.findFn != nil {
		return c.findFn(ctx, minMinutes, zoneID, limit)
	}
	return nil, nil
}
func (c *controllableQuerier) GetFleetSummary(ctx context.Context) (application.FleetSummary, error) {
	// Arrange helper
	if c.summaryFn != nil {
		return c.summaryFn(ctx)
	}
	return application.FleetSummary{}, nil
}
func (c *controllableQuerier) GetVehicleStatus(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error) {
	// Arrange helper
	if c.statusFn != nil {
		return c.statusFn(ctx, plate)
	}
	return application.VehicleStatus{}, nil
}
func (c *controllableQuerier) GetActiveAlerts(ctx context.Context, limit int) ([]application.Alert, error) {
	// Arrange helper
	c.capturedAlertLimit = limit
	if c.alertsFn != nil {
		return c.alertsFn(ctx, limit)
	}
	return nil, nil
}

var _ application.FleetQuerier = (*controllableQuerier)(nil)

func strPtr2(s string) *string { return &s }

// Covers [SPEC-003: AC-001, AC-003, BR-001, BR-004]
func TestFindStopped_ValidationAndClamp(t *testing.T) {
	t.Run("rejects minMinutes 0 (out of range)", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 0, nil, 20)
		// Assert
		if err == nil {
			t.Fatal("expected error for minMinutes 0")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
	t.Run("rejects minMinutes 1441 (above max)", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 1441, nil, 20)
		// Assert
		if err == nil {
			t.Fatal("expected error for minMinutes 1441")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
	t.Run("accepts minMinutes boundaries 1 and 1440", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		// Act
		_, err1 := svc.FindStopped(context.Background(), 1, nil, 20)
		_, err1440 := svc.FindStopped(context.Background(), 1440, nil, 20)
		// Assert
		if err1 != nil {
			t.Fatalf("expected valid for 1, got %v", err1)
		}
		if err1440 != nil {
			t.Fatalf("expected valid for 1440, got %v", err1440)
		}
	})
	t.Run("rejects invalid zoneID not-a-uuid", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		bad := "not-a-uuid"
		// Act
		_, err := svc.FindStopped(context.Background(), 20, &bad, 20)
		// Assert
		if err == nil {
			t.Fatal("expected error for invalid zoneID")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
	t.Run("accepts valid zoneID UUID", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		zone := "550e8400-e29b-41d4-a716-446655440000"
		// Act
		_, err := svc.FindStopped(context.Background(), 20, &zone, 20)
		// Assert
		if err != nil {
			t.Fatalf("expected no error for valid zoneID, got %v", err)
		}
		if q.capturedZone == nil || *q.capturedZone != zone {
			t.Fatalf("expected zone propagated, got %v", q.capturedZone)
		}
	})
	t.Run("clamps limit 0 to default 20", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 0)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if q.capturedLimit != 20 {
			t.Fatalf("expected clamped limit 20 for input 0, got %d", q.capturedLimit)
		}
	})
	t.Run("clamps limit 21 to max 20", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 21)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if q.capturedLimit != 20 {
			t.Fatalf("expected clamped limit 20 for input 21, got %d", q.capturedLimit)
		}
	})
	t.Run("clamps negative limit to default 20", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, -5)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if q.capturedLimit != 20 {
			t.Fatalf("expected clamped limit 20 for input -5, got %d", q.capturedLimit)
		}
	})
}

// Covers [SPEC-003: AC-001, FR-003, BR-001]
func TestFindStopped_QuerierAndRowValidation(t *testing.T) {
	t.Run("returns wrapped error when querier fails", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			findFn: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return nil, errors.New("db down")
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 20)
		// Assert
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, errors.New("db down")) && err.Error() == "" {
			t.Fatalf("expected wrapped error, got %v", err)
		}
		if err.Error() != "find stopped failed: db down" && !contains(err.Error(), "find stopped failed") {
			t.Fatalf("expected 'find stopped failed' wrapped, got %v", err)
		}
	})
	t.Run("returns error when row validation fails (invalid plate)", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		q := &controllableQuerier{
			findFn: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return []domain.StoppedVehicle{
					{Plate: shared.Plate("BAD"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona", DurationMin: 27, Lat: 4.711, Lon: -74.072, StoppedSince: now},
				}, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 20)
		// Assert
		if err == nil {
			t.Fatal("expected validation error for bad plate")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation joined, got %v", err)
		}
	})
	t.Run("returns error when row lat out of range", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		q := &controllableQuerier{
			findFn: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return []domain.StoppedVehicle{
					{Plate: shared.Plate("GTP980"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona", DurationMin: 27, Lat: 999, Lon: -74.072, StoppedSince: now},
				}, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.FindStopped(context.Background(), 20, nil, 20)
		// Assert
		if err == nil {
			t.Fatal("expected validation error for lat out of range")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})
	t.Run("returns empty slice when querier returns nil", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			findFn: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return nil, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		res, err := svc.FindStopped(context.Background(), 20, nil, 20)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res == nil {
			t.Fatalf("expected non-nil empty slice, got nil")
		}
		if len(res) != 0 {
			t.Fatalf("expected 0, got %d", len(res))
		}
	})
	t.Run("returns empty slice when querier returns empty slice", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			findFn: func(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
				return []domain.StoppedVehicle{}, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		res, err := svc.FindStopped(context.Background(), 20, nil, 20)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(res) != 0 {
			t.Fatalf("expected 0, got %d", len(res))
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// Covers [SPEC-003: FR-003]
func TestGetFleetSummary_SuccessAndError(t *testing.T) {
	t.Run("returns summary on success", func(t *testing.T) {
		// Arrange
		expected := application.FleetSummary{Total: 10, Moving: 6, Idle: 3, Alert: 1, ByZone: []application.ByZoneCount{{ZoneName: "Zona Norte", Count: 2}}}
		q := &controllableQuerier{
			summaryFn: func(ctx context.Context) (application.FleetSummary, error) {
				return expected, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		got, err := svc.GetFleetSummary(context.Background())
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Total != 10 || got.Moving != 6 {
			t.Fatalf("expected total 10 moving 6, got %+v", got)
		}
		if len(got.ByZone) != 1 || got.ByZone[0].ZoneName != "Zona Norte" {
			t.Fatalf("expected zone data, got %+v", got.ByZone)
		}
	})
	t.Run("returns wrapped error when querier fails", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			summaryFn: func(ctx context.Context) (application.FleetSummary, error) {
				return application.FleetSummary{}, errors.New("query failed")
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.GetFleetSummary(context.Background())
		// Assert
		if err == nil {
			t.Fatal("expected error")
		}
		if !contains(err.Error(), "get fleet summary failed") {
			t.Fatalf("expected wrapped 'get fleet summary failed', got %v", err)
		}
	})
	t.Run("returns empty summary when no data", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			summaryFn: func(ctx context.Context) (application.FleetSummary, error) {
				return application.FleetSummary{}, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		got, err := svc.GetFleetSummary(context.Background())
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Total != 0 {
			t.Fatalf("expected 0 total, got %d", got.Total)
		}
	})
}

// Covers [SPEC-003: FR-003, BR-009]
func TestGetVehicleStatus_SuccessAndError(t *testing.T) {
	t.Run("returns status on success", func(t *testing.T) {
		// Arrange
		plate := shared.Plate("GTP980")
		expected := application.VehicleStatus{Plate: plate, Speed: 45.5, Status: "moving"}
		q := &controllableQuerier{
			statusFn: func(ctx context.Context, p shared.Plate) (application.VehicleStatus, error) {
				if p != plate {
					t.Fatalf("expected plate %v, got %v", plate, p)
				}
				return expected, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		got, err := svc.GetVehicleStatus(context.Background(), plate)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Plate != plate || got.Speed != 45.5 {
			t.Fatalf("expected plate %v speed 45.5, got %+v", plate, got)
		}
	})
	t.Run("returns wrapped error when querier fails", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			statusFn: func(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error) {
				return application.VehicleStatus{}, errors.New("not found")
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.GetVehicleStatus(context.Background(), shared.Plate("GTP980"))
		// Assert
		if err == nil {
			t.Fatal("expected error")
		}
		if !contains(err.Error(), "get vehicle status failed") {
			t.Fatalf("expected wrapped 'get vehicle status failed', got %v", err)
		}
	})
}

// Covers [SPEC-003: FR-003]
func TestGetActiveAlerts_SuccessAndError(t *testing.T) {
	t.Run("returns alerts on success", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		alerts := []application.Alert{{Plate: shared.Plate("GTP980"), AlertType: "stopped", CreatedAt: now}}
		q := &controllableQuerier{
			alertsFn: func(ctx context.Context, limit int) ([]application.Alert, error) {
				return alerts, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		got, err := svc.GetActiveAlerts(context.Background(), 10)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(got) != 1 || got[0].Plate != "GTP980" {
			t.Fatalf("expected 1 alert GTP980, got %v", got)
		}
	})
	t.Run("returns empty slice when no alerts", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			alertsFn: func(ctx context.Context, limit int) ([]application.Alert, error) {
				return []application.Alert{}, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		got, err := svc.GetActiveAlerts(context.Background(), 5)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected 0, got %d", len(got))
		}
	})
	t.Run("returns wrapped error when querier fails", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			alertsFn: func(ctx context.Context, limit int) ([]application.Alert, error) {
				return nil, errors.New("timeout")
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.GetActiveAlerts(context.Background(), 10)
		// Assert
		if err == nil {
			t.Fatal("expected error")
		}
		if !contains(err.Error(), "get active alerts failed") {
			t.Fatalf("expected wrapped 'get active alerts failed', got %v", err)
		}
	})
	t.Run("propagates limit to querier", func(t *testing.T) {
		// Arrange
		q := &controllableQuerier{
			alertsFn: func(ctx context.Context, limit int) ([]application.Alert, error) {
				if limit != 5 {
					t.Fatalf("expected limit 5, got %d", limit)
				}
				return nil, nil
			},
		}
		svc := application.NewStoppedService(q)
		// Act
		_, err := svc.GetActiveAlerts(context.Background(), 5)
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if q.capturedAlertLimit != 5 {
			t.Fatalf("expected captured 5, got %d", q.capturedAlertLimit)
		}
	})
}
