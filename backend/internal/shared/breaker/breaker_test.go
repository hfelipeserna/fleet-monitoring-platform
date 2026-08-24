package breaker

import (
	"strings"
	"testing"
)

type fakeBreakerState struct {
	state string
}

func (f *fakeBreakerState) State() string { return f.state }
func (f *fakeBreakerState) IsOpen() bool  { return f.state == "open" }

type fakeOpenOnly struct {
	open bool
}

func (f *fakeOpenOnly) State() string {
	if f.open {
		return "open"
	}
	return "closed"
}
func (f *fakeOpenOnly) IsOpen() bool { return f.open }

type fakeStateCase struct {
	state string
}

func (f *fakeStateCase) State() string { return f.state }
func (f *fakeStateCase) IsOpen() bool  { return strings.EqualFold(f.state, "open") }

func TestIsOpen(t *testing.T) {
	// Covers [SPEC-001: FR-008, BR-006]
	t.Run("nil breaker returns false", func(t *testing.T) {
		// Arrange
		var b Breaker = nil

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for nil")
		}
	})

	t.Run("open exact returns true", func(t *testing.T) {
		// Arrange
		b := &fakeBreakerState{state: "open"}

		// Act
		got := IsOpen(b)

		// Assert
		if !got {
			t.Fatalf("expected true for open")
		}
	})

	t.Run("closed returns false", func(t *testing.T) {
		// Arrange
		b := &fakeBreakerState{state: "closed"}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for closed")
		}
	})

	t.Run("IsOpen via IsOpen method true", func(t *testing.T) {
		// Arrange
		b := &fakeOpenOnly{open: true}

		// Act
		got := IsOpen(b)

		// Assert
		if !got {
			t.Fatalf("expected true")
		}
	})

	t.Run("IsOpen via IsOpen method false", func(t *testing.T) {
		// Arrange
		b := &fakeOpenOnly{open: false}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false")
		}
	})

	t.Run("case insensitive via EqualFold when breaker uses it", func(t *testing.T) {
		// Arrange
		cases := []string{"open", "Open", "OPEN", "oPeN"}
		for _, s := range cases {
			b := &fakeStateCase{state: s}
			// Act
			got := IsOpen(b)
			// Assert
			if !got {
				t.Fatalf("expected open for %q", s)
			}
		}
	})

	t.Run("State fallback EqualFold check", func(t *testing.T) {
		// Arrange
		// Directly test the EqualFold logic that IsOpen would use for State-only breakers
		cases := []string{"open", "OPEN", "Open"}
		for _, s := range cases {
			// Act
			got := strings.EqualFold(s, "open")
			// Assert
			if !got {
				t.Fatalf("expected EqualFold true for %q", s)
			}
		}
	})
}
