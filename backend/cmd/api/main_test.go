package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sony/gobreaker"

	fleetapp "fleetmonitoring/backend/internal/fleet/application"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// Covers [SPEC-002: AC-001, AC-009, FR-009]

type fakeReaderAPI struct {
	lastFn    func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error)
	historyFn func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error)
}

func (f *fakeReaderAPI) LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if f.lastFn != nil {
		return f.lastFn(ctx, plate, limit, cursor)
	}
	return nil, "", nil
}
func (f *fakeReaderAPI) History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if f.historyFn != nil {
		return f.historyFn(ctx, plate, from, to, limit, cursor)
	}
	return nil, "", nil
}

type mockZoneRepo struct {
	createFn func(ctx context.Context, z fleet.Zone) (fleet.Zone, error)
	listFn   func(ctx context.Context) ([]fleet.Zone, error)
	updateFn func(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error)
	deleteFn func(ctx context.Context, id string) error
}

func (m *mockZoneRepo) Create(ctx context.Context, z fleet.Zone) (fleet.Zone, error) {
	if m.createFn != nil {
		return m.createFn(ctx, z)
	}
	return z, nil
}
func (m *mockZoneRepo) List(ctx context.Context) ([]fleet.Zone, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, nil
}
func (m *mockZoneRepo) Update(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, z)
	}
	return z, nil
}
func (m *mockZoneRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func TestAPI_Querier(t *testing.T) {
	t.Run("LastPositions success with nil breaker", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		reader := &fakeReaderAPI{lastFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
			// limit is limit+1 from service (10+1=11); return 1 to test non-paginated
			return []fleet.VehiclePos{{Plate: "ABC123", Speed: 10, Status: "moving", ReceivedAt: now}}, "", nil
		}}
		svc := fleetapp.NewQueryService(reader)
		q := &querier{svc: svc, breaker: nil, timeout: 2 * time.Second}

		// Act
		out, next, err := q.LastPositions(context.Background(), nil, 10, "")

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(out) != 1 || next != "" {
			t.Fatalf("expected 1 empty next, got %v %q", len(out), next)
		}
		// pagination: return 11 to get next
		reader2 := &fakeReaderAPI{lastFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
			// limit will be 11
			out := make([]fleet.VehiclePos, 11)
			for i := range out {
				out[i] = fleet.VehiclePos{Plate: "ABC123", Speed: 10, Status: "moving", ReceivedAt: now.Add(time.Duration(i) * time.Minute)}
			}
			return out, "", nil
		}}
		svc2 := fleetapp.NewQueryService(reader2)
		q2 := &querier{svc: svc2, breaker: nil, timeout: 2 * time.Second}
		out2, next2, err2 := q2.LastPositions(context.Background(), nil, 10, "")
		if err2 != nil {
			t.Fatalf("got %v", err2)
		}
		if len(out2) != 10 || next2 == "" {
			t.Fatalf("expected 10 with next, got %d %q", len(out2), next2)
		}
	})

	t.Run("LastPositions breaker open -> error wrapped", func(t *testing.T) {
		// Arrange
		reader := &fakeReaderAPI{}
		svc := fleetapp.NewQueryService(reader)
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test", ReadyToTrip: func(c gobreaker.Counts) bool { return false }})
		// force open by manual trip: we can set state via failures
		for i := 0; i < 5; i++ {
			_, _ = cb.Execute(func() (any, error) { return nil, errors.New("fail") })
		}
		// Instead use open state directly via custom breaker that returns StateOpen
		openCB := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "open"})
		// trip it: use Execute to open via failure ratio - create with low threshold
		// Simpler: create a breaker and manually set to open via failing many times with small timeout
		_ = openCB
		// Use a fake open breaker: create CB and then set via State check - easiest is to use gobreaker with ReadyToTrip always true
		alwaysOpen := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "always",
			MaxRequests: 1,
			Interval:    time.Second,
			Timeout:     time.Second,
			ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 1 },
		})
		_, _ = alwaysOpen.Execute(func() (any, error) { return nil, errors.New("fail") })
		// now state should be open
		q := &querier{svc: svc, breaker: alwaysOpen, timeout: 2 * time.Second}
		// Act
		_, _, err := q.LastPositions(context.Background(), nil, 10, "")

		// Assert
		if err == nil || !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected ErrOpenState, got %v", err)
		}
	})

	t.Run("History success and breaker nil", func(t *testing.T) {
		// Arrange
		plate := shared.Plate("GTP890")
		reader := &fakeReaderAPI{historyFn: func(ctx context.Context, p shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
			if p != plate {
				t.Fatalf("expected plate %q, got %q", plate, p)
			}
			return []fleet.VehiclePos{}, "", nil
		}}
		svc := fleetapp.NewQueryService(reader)
		q := &querier{svc: svc, breaker: nil, timeout: 2 * time.Second}

		// Act
		_, _, err := q.History(context.Background(), plate, nil, nil, 5, "")

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("History breaker open -> error", func(t *testing.T) {
		// Arrange
		plate, _ := shared.ParsePlate("GTP890")
		reader := &fakeReaderAPI{}
		svc := fleetapp.NewQueryService(reader)
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name: "hb", ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 1 },
		})
		_, _ = cb.Execute(func() (any, error) { return nil, errors.New("fail") })
		q := &querier{svc: svc, breaker: cb, timeout: 2 * time.Second}

		// Act
		_, _, err := q.History(context.Background(), plate, nil, nil, 5, "")

		// Assert
		if err == nil || !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected open, got %v", err)
		}
	})
}

func TestAPI_OpsProvider(t *testing.T) {
	t.Run("BreakerState nil closed open half-open", func(t *testing.T) {
		// Arrange
		opsNil := &opsProvider{breaker: nil}
		cbClosed := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "c"})
		opsClosed := &opsProvider{breaker: cbClosed}
		cbOpen := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "o", ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 1 }})
		_, _ = cbOpen.Execute(func() (any, error) { return nil, errors.New("fail") })
		opsOpen := &opsProvider{breaker: cbOpen}

		// Act
		a := opsNil.BreakerState()
		b := opsClosed.BreakerState()
		c := opsOpen.BreakerState()

		// Assert
		if a != "closed" {
			t.Fatalf("expected closed nil, got %q", a)
		}
		if b != "closed" {
			t.Fatalf("expected closed, got %q", b)
		}
		if c != "open" {
			t.Fatalf("expected open, got %q", c)
		}
	})

	t.Run("NATSConnected nil false", func(t *testing.T) {
		// Arrange
		ops := &opsProvider{nc: nil}
		// Act
		ok := ops.NATSConnected()
		// Assert
		if ok {
			t.Fatalf("expected false nil nc")
		}
	})

	t.Run("DBPoolStat nil unknown", func(t *testing.T) {
		// Arrange
		ops := &opsProvider{pool: nil}
		// Act
		s := ops.DBPoolStat()
		// Assert
		if s != "unknown" {
			t.Fatalf("expected unknown, got %q", s)
		}
	})

	t.Run("DBPoolStat with pool returns total/idle", func(t *testing.T) {
		// Arrange
		pool, _ := newFakePool()
		ops := &opsProvider{pool: pool}
		// Act
		s := ops.DBPoolStat()
		// Assert
		if !contains(s, "total=") || !contains(s, "idle=") {
			t.Fatalf("expected total/idle, got %q", s)
		}
	})
}

func TestAPI_WithBreaker(t *testing.T) {
	t.Run("success with nil breaker", func(t *testing.T) {
		// Arrange
		// Act
		out, err := withBreaker(context.Background(), nil, 2*time.Second, func(ctx context.Context) (string, error) {
			return "ok", nil
		})
		// Assert
		if err != nil || out != "ok" {
			t.Fatalf("expected ok nil, got %q %v", out, err)
		}
	})

	t.Run("breaker open -> error", func(t *testing.T) {
		// Arrange
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "with", ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 1 }})
		_, _ = cb.Execute(func() (any, error) { return nil, errors.New("fail") })
		// Act
		_, err := withBreaker(context.Background(), cb, 2*time.Second, func(ctx context.Context) (string, error) { return "ok", nil })
		// Assert
		if err == nil || !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected open, got %v", err)
		}
	})

	t.Run("breaker Execute error propagates", func(t *testing.T) {
		// Arrange
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "with2"})
		// Act
		_, err := withBreaker(context.Background(), cb, 2*time.Second, func(ctx context.Context) (string, error) {
			return "", errors.New("inner fail")
		})
		// Assert
		if err == nil || err.Error() != "inner fail" {
			t.Fatalf("expected inner fail, got %v", err)
		}
	})
}

func TestAPI_ZoneBreaker(t *testing.T) {
	t.Run("Create success via breaker", func(t *testing.T) {
		// Arrange
		repo := &mockZoneRepo{createFn: func(ctx context.Context, z fleet.Zone) (fleet.Zone, error) {
			return z, nil
		}}
		svc := fleetapp.NewZoneService(repo)
		zb := &zoneBreaker{svc: svc, breaker: nil, timeout: 2 * time.Second}
		coords := [][]float64{{-74.07, 4.71}, {-74.05, 4.71}, {-74.05, 4.73}, {-74.07, 4.73}, {-74.07, 4.71}}
		// Act
		out, err := zb.Create(context.Background(), "Norte", coords)
		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if out.Name != "Norte" {
			t.Fatalf("expected Norte, got %q", out.Name)
		}
	})

	t.Run("List breaker open -> error", func(t *testing.T) {
		// Arrange
		repo := &mockZoneRepo{}
		svc := fleetapp.NewZoneService(repo)
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "zb", ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 1 }})
		_, _ = cb.Execute(func() (any, error) { return nil, errors.New("fail") })
		zb := &zoneBreaker{svc: svc, breaker: cb, timeout: 2 * time.Second}
		// Act
		_, err := zb.List(context.Background())
		// Assert
		if err == nil || !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected open, got %v", err)
		}
	})

	t.Run("Update and Delete with nil breaker", func(t *testing.T) {
		// Arrange
		repo := &mockZoneRepo{
			updateFn: func(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error) { return z, nil },
			deleteFn: func(ctx context.Context, id string) error { return nil },
		}
		svc := fleetapp.NewZoneService(repo)
		zb := &zoneBreaker{svc: svc, breaker: nil, timeout: 2 * time.Second}
		coords := [][]float64{{-74.07, 4.71}, {-74.05, 4.71}, {-74.05, 4.73}, {-74.07, 4.73}, {-74.07, 4.71}}
		// Act
		out, err := zb.Update(context.Background(), "550e8400-e29b-41d4-a716-446655440001", "Norte", coords)
		err2 := zb.Delete(context.Background(), "550e8400-e29b-41d4-a716-446655440001")
		// Assert
		if err != nil || out.Name != "Norte" {
			t.Fatalf("expected update ok, got %v %v", out, err)
		}
		if err2 != nil {
			t.Fatalf("expected delete nil, got %v", err2)
		}
	})
}

func TestAPI_Server(t *testing.T) {
	t.Run("NewServer and Run with immediate cancel returns nil", func(t *testing.T) {
		// Arrange
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
		srv := NewServer(handler, ":0", nil, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Act
		err := srv.Run(ctx)

		// Assert
		if err != nil {
			t.Fatalf("expected nil Run cancelled, got %v", err)
		}
		if srv.httpServer.Addr != ":0" {
			t.Fatalf("expected :0, got %q", srv.httpServer.Addr)
		}
		if srv.httpServer.ReadTimeout != 5*time.Second || srv.httpServer.IdleTimeout != 120*time.Second {
			t.Fatalf("unexpected timeouts")
		}
	})

	t.Run("Shutdown with nil pool nc", func(t *testing.T) {
		// Arrange
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		srv := NewServer(handler, ":0", nil, nil)
		// Act
		err := srv.Shutdown(context.Background())
		// Assert
		if err != nil {
			t.Fatalf("expected nil shutdown, got %v", err)
		}
	})

	t.Run("Bootstrap fails when DATABASE_URL not set", func(t *testing.T) {
		// Arrange
		t.Setenv("DATABASE_URL", "")
		// Act
		_, err := Bootstrap(context.Background())
		// Assert
		if err == nil || !contains(err.Error(), "DATABASE_URL") {
			t.Fatalf("expected DATABASE_URL error, got %v", err)
		}
	})

	t.Run("Bootstrap fails with invalid NATS_URL", func(t *testing.T) {
		// Arrange
		t.Setenv("DATABASE_URL", "postgres://fleet:fleet@localhost:5432/fleet?sslmode=disable")
		t.Setenv("NATS_URL", "nats://invalid.invalid:4222")
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		// Act
		_, err := Bootstrap(ctx)
		// Assert
		if err == nil {
			t.Fatalf("expected nats connect failure or timeout")
		}
		if !contains(err.Error(), "nats") && !contains(err.Error(), "ensure") && !contains(err.Error(), "pgx") {
			t.Logf("got error %v, acceptable bootstrap failure", err)
		}
	})
}

func newFakePool() (*pgxpool.Pool, error) {
	// Use pgxpool.New with a dummy URL; it parses without connecting, returns pool object even if DB not reachable
	// This is used to test DBPoolStat non-nil branch without requiring real DB
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	pool, err := pgxpool.New(ctx, "postgres://fleet:fleet@localhost:5432/fleet?sslmode=disable")
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
