package metrics

import (
	"sync"
	"testing"
)

// Covers [SPEC-001: AC-007, BR-006, FR-008]

func TestCounterVec(t *testing.T) {
	t.Run("Inc and Add and Total and Snapshot and Range", func(t *testing.T) {
		// Arrange
		var c CounterVec

		// Act
		c.Inc("plate-A")
		c.Add("plate-A", 2)
		c.Add("plate-B", 5)
		total := c.Total()
		snap := c.Snapshot()

		// Assert
		if total != 8 {
			t.Fatalf("expected total 8, got %d", total)
		}
		if snap["plate-A"] != 3 {
			t.Fatalf("expected plate-A 3, got %d", snap["plate-A"])
		}
		if snap["plate-B"] != 5 {
			t.Fatalf("expected plate-B 5, got %d", snap["plate-B"])
		}
		// Range
		seen := map[string]int64{}
		c.Range(func(label string, value int64) bool {
			seen[label] = value
			return true
		})
		if seen["plate-A"] != 3 || seen["plate-B"] != 5 {
			t.Fatalf("expected Range to see both, got %v", seen)
		}
	})

	t.Run("concurrent Add", func(t *testing.T) {
		// Arrange
		var c CounterVec
		var wg sync.WaitGroup

		// Act
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.Add("concurrent", 1)
			}()
		}
		wg.Wait()

		// Assert
		if c.Total() != 100 {
			t.Fatalf("expected 100, got %d", c.Total())
		}
		snap := c.Snapshot()
		if snap["concurrent"] != 100 {
			t.Fatalf("expected 100, got %d", snap["concurrent"])
		}
	})
}
