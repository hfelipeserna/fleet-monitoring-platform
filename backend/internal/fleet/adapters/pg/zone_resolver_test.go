package pg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"
)

type fakeRows struct {
	nextCalls int
	nextRet []bool
	scanID string
	scanErr error
	err error
	closeCalled bool
}

func (f *fakeRows) Next() bool {
	if len(f.nextRet) == 0 {
		return false
	}
	ret := f.nextRet[0]
	f.nextRet = f.nextRet[1:]
	f.nextCalls++
	return ret
}
func (f *fakeRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	if len(dest) > 0 {
		if s, ok := dest[0].(*string); ok {
			*s = f.scanID
		}
	}
	return nil
}
func (f *fakeRows) Close() { f.closeCalled = true }
func (f *fakeRows) Err() error { return f.err }

type fakeQuerier struct {
	rows Rows
	err error
	capturedSQL string
}

func (f *fakeQuerier) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	f.capturedSQL = sql
	return f.rows, f.err
}

type fakeZoneBreaker struct {
	state gobreaker.State
	execErr error
}

func (f *fakeZoneBreaker) State() gobreaker.State { return f.state }
func (f *fakeZoneBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	return fn()
}

func TestPGZoneResolver_IsInside(t *testing.T) {
	// Covers [SPEC-002: AC-003]
	t.Run("found returns id and inside true", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{true}, scanID: "zone-123"}
		q := &fakeQuerier{rows: rows}
		r := NewZoneResolver(q)

		// Act
		id, inside, err := r.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if !inside {
			t.Fatalf("expected inside true")
		}
		if id == nil || *id != "zone-123" {
			t.Fatalf("expected zone-123 got %v", id)
		}
	})

	t.Run("not found returns nil false", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{false}}
		q := &fakeQuerier{rows: rows}
		r := NewZoneResolver(q)

		// Act
		id, inside, err := r.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if inside {
			t.Fatalf("expected false")
		}
		if id != nil {
			t.Fatalf("expected nil")
		}
	})

	t.Run("query error returns error", func(t *testing.T) {
		// Arrange
		q := &fakeQuerier{err: errors.New("db down")}
		r := NewZoneResolver(q)

		// Act
		_, _, err := r.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("scan error returns error", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{true}, scanErr: errors.New("scan fail")}
		q := &fakeQuerier{rows: rows}
		r := NewZoneResolver(q)

		// Act
		_, _, err := r.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("rows Err returns error", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{false}, err: errors.New("rows err")}
		q := &fakeQuerier{rows: rows}
		r := NewZoneResolver(q)

		// Act
		_, _, err := r.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestZoneResolverWithBreaker_IsInside(t *testing.T) {
	// Covers [SPEC-002: FR-008]
	t.Run("breaker open returns error", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{true}, scanID: "z"}
		q := &fakeQuerier{rows: rows}
		inner := NewZoneResolver(q)
		brk := &fakeZoneBreaker{state: gobreaker.StateOpen}
		w := NewZoneResolverWithBreaker(inner, brk, 2*time.Second)

		// Act
		_, _, err := w.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected ErrOpenState got %v", err)
		}
	})

	t.Run("success via breaker", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{true}, scanID: "zone-999"}
		q := &fakeQuerier{rows: rows}
		inner := NewZoneResolver(q)
		brk := &fakeZoneBreaker{state: gobreaker.StateClosed}
		w := NewZoneResolverWithBreaker(inner, brk, 2*time.Second)

		// Act
		id, inside, err := w.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if !inside || *id != "zone-999" {
			t.Fatalf("unexpected %v %v", id, inside)
		}
	})

	t.Run("breaker Execute error", func(t *testing.T) {
		// Arrange
		rows := &fakeRows{nextRet: []bool{true}}
		q := &fakeQuerier{rows: rows}
		inner := NewZoneResolver(q)
		brk := &fakeZoneBreaker{state: gobreaker.StateClosed, execErr: errors.New("breaker exec fail")}
		w := NewZoneResolverWithBreaker(inner, brk, 2*time.Second)

		// Act
		_, _, err := w.IsInside(context.Background(), "ABC123", 4.7, -74.0)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
