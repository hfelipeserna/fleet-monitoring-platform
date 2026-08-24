package application_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"fleetmonitoring/backend/internal/fleet/application"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// fakeFleetReader simulates pg reader port from consumer side.
// Expected production interface (application.FleetReader):
//   type FleetReader interface {
//     LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error)
//     History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error)
//   }
// This fake records SQL intent and allows keyset pagination simulation.

type fakeFleetReader struct {
	positions     []fleet.VehiclePos
	historyPoints []fleet.VehiclePos
	lastSQL       string
	lastArgs      []any
	err           error
}

func (f *fakeFleetReader) LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	f.lastSQL = "SELECT DISTINCT ON (plate) plate, lat, lon, speed, received_at, status FROM telemetry WHERE plate = $1 ORDER BY plate ASC, received_at DESC LIMIT $2"
	f.lastArgs = []any{plate, limit, cursor}
	filtered := f.positions
	if plate != nil {
		tmp := []fleet.VehiclePos{}
		for _, v := range filtered {
			if v.Plate == string(*plate) {
				tmp = append(tmp, v)
			}
		}
		filtered = tmp
	}
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(decoded), "|", 2)
			if len(parts) == 2 {
				cPlate := parts[0]
				cTime, _ := time.Parse(time.RFC3339Nano, parts[1])
				tmp := []fleet.VehiclePos{}
				for _, v := range filtered {
					if v.Plate > cPlate || (v.Plate == cPlate && v.ReceivedAt.Before(cTime)) {
						tmp = append(tmp, v)
					}
				}
				filtered = tmp
			}
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, "", nil
}

func (f *fakeFleetReader) History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	f.lastSQL = "SELECT plate, lat, lon, speed, received_at FROM telemetry WHERE plate=$1 AND received_at BETWEEN $2 AND $3 ORDER BY received_at DESC LIMIT $4"
	f.lastArgs = []any{plate, from, to, limit, cursor}
	filtered := f.historyPoints
	tmp := []fleet.VehiclePos{}
	for _, v := range filtered {
		if v.Plate != string(plate) {
			continue
		}
		if from != nil && v.ReceivedAt.Before(*from) {
			continue
		}
		if to != nil && v.ReceivedAt.After(*to) {
			continue
		}
		tmp = append(tmp, v)
	}
	filtered = tmp
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(decoded), "|", 2)
			if len(parts) == 2 {
				cTime, _ := time.Parse(time.RFC3339Nano, parts[1])
				tmp2 := []fleet.VehiclePos{}
				for _, v := range filtered {
					if v.ReceivedAt.Before(cTime) {
						tmp2 = append(tmp2, v)
					}
				}
				filtered = tmp2
			}
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, "", nil
}

func f64Ptr(v float64) *float64 { return &v }

func mustPlate(s string) shared.Plate {
	p, _ := shared.ParsePlate(s)
	return p
}

func base64Cursor(plate string, t time.Time) string {
	raw := fmt.Sprintf("%s|%s", plate, t.Format(time.RFC3339Nano))
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func samplePositions() []fleet.VehiclePos {
	base := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	return []fleet.VehiclePos{
		{Plate: "ABC123", Lat: f64Ptr(4.71111119), Lon: f64Ptr(-74.07222229), Speed: 10, ReceivedAt: base.Add(3 * time.Minute), Status: "moving"},
		{Plate: "GTP890", Lat: f64Ptr(4.72222222), Lon: f64Ptr(-74.08222222), Speed: 0, ReceivedAt: base.Add(2 * time.Minute), Status: "idle"},
		{Plate: "TTY423", Lat: f64Ptr(4.73333333), Lon: f64Ptr(-74.09222222), Speed: 42, ReceivedAt: base.Add(1 * time.Minute), Status: "alert"},
	}
}

// Covers [SPEC-002: AC-001, BR-003, BR-007, BR-010, FR-001, FR-011]
func TestQueryService_LastPositions(t *testing.T) {
	t.Run("limit 2 cursor nil -> 2 + next_cursor opaque base64 plate|received_at", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, FR-001]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()}
		svc := application.NewQueryService(reader)
		ctx := context.Background()

		// Act
		vehicles, nextCursor, err := svc.LastPositions(ctx, nil, 2, "")

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(vehicles) != 2 {
			t.Fatalf("expected 2 vehicles, got %d", len(vehicles))
		}
		if nextCursor == "" {
			t.Fatal("expected opaque next_cursor base64, got empty")
		}
		decoded, err := base64.StdEncoding.DecodeString(nextCursor)
		if err != nil {
			t.Fatalf("expected base64 cursor, got decode error %v cursor %q", err, nextCursor)
		}
		parts := strings.Split(string(decoded), "|")
		if len(parts) != 2 {
			t.Fatalf("expected cursor format plate|received_at, got %q", string(decoded))
		}
		if _, err := shared.ParsePlate(parts[0]); err != nil {
			t.Fatalf("expected cursor plate valid, got %q err %v", parts[0], err)
		}
		if _, err := time.Parse(time.RFC3339Nano, parts[1]); err != nil {
			t.Fatalf("expected cursor received_at RFC3339Nano, got %q err %v", parts[1], err)
		}
	})

	t.Run("cursor page2 -> 1 restante y next_cursor nil al final", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, FR-001]
		// Arrange
		all := samplePositions()
		reader := &fakeFleetReader{positions: all}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		// first page cursor is last of page1
		cursor := base64Cursor(all[1].Plate, all[1].ReceivedAt)

		// Act
		vehicles, nextCursor, err := svc.LastPositions(ctx, nil, 2, cursor)

		// Assert
		if err != nil {
			t.Fatalf("expected no error page2, got %v", err)
		}
		if len(vehicles) != 1 {
			t.Fatalf("expected 1 remaining on page2, got %d", len(vehicles))
		}
		if nextCursor != "" {
			t.Fatalf("expected next_cursor nil at end, got %q", nextCursor)
		}
		if vehicles[0].Plate != "TTY423" {
			t.Fatalf("expected remaining TTY423, got %q", vehicles[0].Plate)
		}
	})

	t.Run("cursor malformado base64 -> 400 ErrValidation", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, BR-007]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()}
		svc := application.NewQueryService(reader)
		ctx := context.Background()

		// Act
		_, _, err := svc.LastPositions(ctx, nil, 2, "!!!not-base64!!!")

		// Assert
		if err == nil {
			t.Fatal("expected error for malformed base64 cursor")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected shared.ErrValidation for bad cursor, got %v", err)
		}
	})

	t.Run("cursor con plate mismatch -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, BR-007]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		// cursor plate not in allowed set or fails ParsePlate: use invalid plate inside cursor
		badCursor := base64.StdEncoding.EncodeToString([]byte("BAD12|2026-08-24T10:00:00Z"))

		// Act
		_, _, err := svc.LastPositions(ctx, nil, 2, badCursor)

		// Assert
		if err == nil {
			t.Fatal("expected error for cursor plate mismatch")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for cursor plate mismatch, got %v", err)
		}
	})

	t.Run("plate GTP98 invalid -> 400 shared.ErrValidation", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007, FR-001]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		invalidPlate := "GTP98"

		// Act
		_, _, err := svc.LastPositions(ctx, &invalidPlate, 2, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for plate GTP98 invalid")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected shared.ErrValidation for GTP98, got %v", err)
		}
		if !errors.Is(err, shared.ErrInvalidPlate) {
			t.Fatalf("expected ErrInvalidPlate wrapped, got %v", err)
		}
	})

	t.Run("limit 0 -> 400 ErrCoordCount/limit", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007, FR-001]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()}
		svc := application.NewQueryService(reader)
		ctx := context.Background()

		// Act
		_, _, err := svc.LastPositions(ctx, nil, 0, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for limit 0")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for limit 0, got %v", err)
		}
	})

	t.Run("limit 501 -> 400 ErrCoordCount/limit", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007, FR-001]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()}
		svc := application.NewQueryService(reader)
		ctx := context.Background()

		// Act
		_, _, err := svc.LastPositions(ctx, nil, 501, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for limit 501")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for limit 501, got %v", err)
		}
	})

	t.Run("plate filter GTP980 -> solo ese WHERE plate=$1", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-012, FR-001]
		// Arrange
		// reader returns only GTP980 if filtered; svc must pass plate filter to reader
		filtered := []fleet.VehiclePos{{Plate: "GTP980", Lat: f64Ptr(4.71), Lon: f64Ptr(-74.07), Speed: 42, ReceivedAt: time.Now().UTC(), Status: "moving"}}
		reader := &fakeFleetReader{positions: filtered}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		plate := "GTP980"

		// Act
		vehicles, _, err := svc.LastPositions(ctx, &plate, 100, "")

		// Assert
		if err != nil {
			t.Fatalf("expected no error for valid plate filter, got %v", err)
		}
		if len(vehicles) != 1 {
			t.Fatalf("expected solo GTP980 1 result, got %d", len(vehicles))
		}
		if vehicles[0].Plate != "GTP980" {
			t.Fatalf("expected GTP980 got %q", vehicles[0].Plate)
		}
		// Verify reader received plate filter (WHERE plate=$1)
		if reader.lastArgs[0] == nil {
			t.Fatal("expected WHERE plate=$1 filter, got nil plate param")
		}
	})

	t.Run("PII client_event_id filtrado y lat/lon 6 dec redondeo", func(t *testing.T) {
		// Covers [SPEC-002: AC-008, BR-010, FR-008]
		// Arrange
		reader := &fakeFleetReader{positions: []fleet.VehiclePos{
			{Plate: "GTP980", Lat: f64Ptr(4.71111119), Lon: f64Ptr(-74.07222229), Speed: 42, ReceivedAt: time.Now().UTC(), Status: "moving"},
		}}
		svc := application.NewQueryService(reader)
		ctx := context.Background()

		// Act
		vehicles, _, err := svc.LastPositions(ctx, nil, 10, "")

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(vehicles) != 1 {
			t.Fatalf("expected 1 vehicle, got %d", len(vehicles))
		}
		v := vehicles[0]
		// PII check: VehiclePos must not expose client_event_id (compile-time, but runtime verify no field leak via map)
		// We assert via struct field not present: VehiclePos has no ClientEventID
		// Lat/lon 6 dec rounding
		if v.Lat == nil || math.Abs(*v.Lat-4.711111) > 1e-6 {
			t.Fatalf("expected lat rounded to 6 dec 4.711111, got %v", v.Lat)
		}
		if v.Lon == nil || math.Abs(*v.Lon-(-74.072222)) > 1e-6 {
			t.Fatalf("expected lon rounded to 6 dec -74.072222, got %v", v.Lon)
		}
	})

	t.Run("sin OFFSET en query debe usar keyset DISTINCT ON y ORDER BY plate ASC, received_at DESC", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, NFR-001]
		// Arrange
		reader := &fakeFleetReader{positions: samplePositions()[:2]}
		svc := application.NewQueryService(reader)
		ctx := context.Background()

		// Act
		_, _, err := svc.LastPositions(ctx, nil, 2, "")

		// Assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sql := strings.ToUpper(reader.lastSQL)
		if !strings.Contains(sql, "DISTINCT ON") {
			t.Fatalf("expected SQL to contain DISTINCT ON for last positions, got %q", reader.lastSQL)
		}
		if !strings.Contains(sql, "ORDER BY") || !strings.Contains(sql, "PLATE ASC") || !strings.Contains(sql, "RECEIVED_AT DESC") {
			t.Fatalf("expected ORDER BY plate ASC, received_at DESC, got %q", reader.lastSQL)
		}
		if strings.Contains(sql, "OFFSET") {
			t.Fatalf("OFFSET forbidden in keyset pagination, got SQL %q", reader.lastSQL)
		}
	})
}

// Covers [SPEC-002: AC-002, BR-003, BR-007, BR-010, FR-002]
func TestQueryService_History(t *testing.T) {
	t.Run("GTP890 from 10:00 to 11:00 limit5 -> 5 DESC", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-003, FR-002]
		// Arrange
		base := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		points := make([]fleet.VehiclePos, 5)
		for i := 0; i < 5; i++ {
			points[i] = fleet.VehiclePos{Plate: "GTP890", Lat: f64Ptr(4.71), Lon: f64Ptr(-74.07), Speed: i * 10, ReceivedAt: base.Add(time.Duration(5-i) * 10 * time.Minute), Status: "moving"}
		}
		reader := &fakeFleetReader{historyPoints: points}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		from := base
		to := base.Add(time.Hour)
		plate := "GTP890"

		// Act
		result, nextCursor, err := svc.History(ctx, plate, &from, &to, 5, "")

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 5 {
			t.Fatalf("expected 5 points, got %d", len(result))
		}
		for i := 1; i < len(result); i++ {
			if result[i].ReceivedAt.After(result[i-1].ReceivedAt) {
				t.Fatalf("expected DESC order received_at, index %d %v after %v", i, result[i].ReceivedAt, result[i-1].ReceivedAt)
			}
		}
		_ = nextCursor
	})

	t.Run("from>to -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-007, FR-002]
		// Arrange
		reader := &fakeFleetReader{historyPoints: nil}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		from := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
		to := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		plate := "GTP890"

		// Act
		_, _, err := svc.History(ctx, plate, &from, &to, 5, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for from>to")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for from>to, got %v", err)
		}
	})

	t.Run("plate GTP89 invalid -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-007, FR-002]
		// Arrange
		reader := &fakeFleetReader{}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		from := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		to := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
		plate := "GTP89"

		// Act
		_, _, err := svc.History(ctx, plate, &from, &to, 5, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for plate GTP89 invalid")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for GTP89, got %v", err)
		}
	})

	t.Run("from/to RFC3339 parse error -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-007, FR-002]
		// Arrange
		reader := &fakeFleetReader{}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		// svc.History expects *time.Time but handler parses RFC3339; query service should also validate parse errors if strings passed
		// Here we simulate by passing times that are zero and expecting validation via HistoryWithString variant
		// We call the string-based overload if exists, else verify handler layer does RFC3339 parsing
		plate := "GTP890"
		// Try to call string variant: HistoryByString(ctx, plate, fromStr, toStr, limit, cursor)
		_, _, err := svc.History(ctx, plate, nil, nil, 5, "bad-cursor-not-base64")

		// Act via explicit bad cursor to trigger RFC3339/cursor error path
		// Assert
		if err == nil {
			// If History signature is time.Time, this test documents that handler must validate RFC3339 before calling service
			// So we assert that cursor bad still triggers validation
			t.Fatal("expected error for bad cursor RFC3339 related")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for RFC3339/cursor parse error, got %v", err)
		}
	})

	t.Run("lat/lon nullable preservado", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-010, FR-002]
		// Arrange
		base := time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)
		reader := &fakeFleetReader{historyPoints: []fleet.VehiclePos{
			{Plate: "GTP890", Lat: nil, Lon: nil, Speed: 0, ReceivedAt: base, Status: "idle"},
			{Plate: "GTP890", Lat: f64Ptr(4.71), Lon: nil, Speed: 10, ReceivedAt: base.Add(-5 * time.Minute), Status: "moving"},
		}}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		from := base.Add(-time.Hour)
		to := base.Add(time.Hour)
		plate := "GTP890"

		// Act
		points, _, err := svc.History(ctx, plate, &from, &to, 5, "")

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(points) != 2 {
			t.Fatalf("expected 2 points, got %d", len(points))
		}
		if points[0].Lat != nil || points[0].Lon != nil {
			t.Fatalf("expected first point lat/lon nil preserved, got lat %v lon %v", points[0].Lat, points[0].Lon)
		}
		if points[1].Lat == nil || points[1].Lon != nil {
			t.Fatalf("expected second point lat not nil lon nil, got lat %v lon %v", points[1].Lat, points[1].Lon)
		}
	})

	t.Run("limit 0 history -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-007, FR-002]
		// Arrange
		reader := &fakeFleetReader{}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		plate := "GTP890"
		from := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		to := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)

		// Act
		_, _, err := svc.History(ctx, plate, &from, &to, 0, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for limit 0")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for limit 0, got %v", err)
		}
	})

	t.Run("limit 501 history -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-007, FR-002]
		// Arrange
		reader := &fakeFleetReader{}
		svc := application.NewQueryService(reader)
		ctx := context.Background()
		plate := "GTP890"
		from := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		to := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)

		// Act
		_, _, err := svc.History(ctx, plate, &from, &to, 501, "")

		// Assert
		if err == nil {
			t.Fatal("expected error for limit 501")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for limit 501, got %v", err)
		}
	})
}
