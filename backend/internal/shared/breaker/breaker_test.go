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

type fakeStateOnly struct {
	state string
}

func (f *fakeStateOnly) State() string { return f.state }

type fakeIsOpenOnly struct {
	open bool
}

func (f *fakeIsOpenOnly) IsOpen() bool { return f.open }

type fakeNeither struct{}

type fakeBoth struct {
	isOpen bool
	state  string
}

func (f *fakeBoth) IsOpen() bool { return f.isOpen }
func (f *fakeBoth) State() string { return f.state }

type fakeHalfOpen struct{}

func (f *fakeHalfOpen) State() string { return "half-open" }
func (f *fakeHalfOpen) IsOpen() bool  { return false }

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

	t.Run("nil any returns false", func(t *testing.T) {
		// Arrange
		var b any = nil

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for nil any")
		}
	})

	t.Run("typed nil pointer returns false", func(t *testing.T) {
		// Arrange
		var b *fakeBreakerState = nil

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for typed nil")
		}
	})

	t.Run("typed nil Breaker interface returns false", func(t *testing.T) {
		// Arrange
		var b *fakeOpenOnly = nil
		var iface any = b

		// Act
		got := IsOpen(iface)

		// Assert
		if got {
			t.Fatalf("expected false for typed nil Breaker")
		}
	})

	t.Run("typed nil StateOnly returns false", func(t *testing.T) {
		// Arrange
		var b *fakeStateOnly = nil

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for typed nil StateOnly")
		}
	})

	t.Run("typed nil IsOpenOnly returns false", func(t *testing.T) {
		// Arrange
		var b *fakeIsOpenOnly = nil

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for typed nil IsOpenOnly")
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

	t.Run("half-open via IsOpen false", func(t *testing.T) {
		// Arrange
		b := &fakeHalfOpen{}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for half-open")
		}
	})

	t.Run("half-open via State only returns false", func(t *testing.T) {
		// Arrange
		b := &fakeStateOnly{state: "half-open"}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for half-open StateOnly")
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

	t.Run("IsOpenOnly true", func(t *testing.T) {
		// Arrange
		b := &fakeIsOpenOnly{open: true}

		// Act
		got := IsOpen(b)

		// Assert
		if !got {
			t.Fatalf("expected true for IsOpenOnly true")
		}
	})

	t.Run("IsOpenOnly false", func(t *testing.T) {
		// Arrange
		b := &fakeIsOpenOnly{open: false}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for IsOpenOnly false")
		}
	})

	t.Run("StateOnly open returns true", func(t *testing.T) {
		// Arrange
		b := &fakeStateOnly{state: "open"}

		// Act
		got := IsOpen(b)

		// Assert
		if !got {
			t.Fatalf("expected true for StateOnly open")
		}
	})

	t.Run("StateOnly closed returns false", func(t *testing.T) {
		// Arrange
		b := &fakeStateOnly{state: "closed"}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for StateOnly closed")
		}
	})

	t.Run("StateOnly case insensitive open", func(t *testing.T) {
		// Arrange
		cases := []struct {
			state string
			want  bool
		}{
			{state: "open", want: true},
			{state: "Open", want: true},
			{state: "OPEN", want: true},
			{state: "oPeN", want: true},
			{state: "closed", want: false},
			{state: "half-open", want: false},
			{state: "CLOSED", want: false},
		}
		for _, tc := range cases {
			// Arrange
			b := &fakeStateOnly{state: tc.state}

			// Act
			got := IsOpen(b)

			// Assert
			if got != tc.want {
				t.Fatalf("StateOnly %q: expected %v got %v", tc.state, tc.want, got)
			}
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

	t.Run("neither interface returns false", func(t *testing.T) {
		// Arrange
		b := &fakeNeither{}
		alt := "plain string"
		alt2 := 42

		// Act
		got1 := IsOpen(b)
		got2 := IsOpen(alt)
		got3 := IsOpen(alt2)

		// Assert
		if got1 {
			t.Fatalf("expected false for neither")
		}
		if got2 {
			t.Fatalf("expected false for string")
		}
		if got3 {
			t.Fatalf("expected false for int")
		}
	})

	t.Run("both interfaces IsOpen takes precedence true", func(t *testing.T) {
		// Arrange
		b := &fakeBoth{isOpen: true, state: "closed"}

		// Act
		got := IsOpen(b)

		// Assert
		if !got {
			t.Fatalf("expected true: IsOpen should take precedence over State")
		}
	})

	t.Run("both interfaces IsOpen takes precedence false", func(t *testing.T) {
		// Arrange
		b := &fakeBoth{isOpen: false, state: "open"}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false: IsOpen false should take precedence even if State is open")
		}
	})

	t.Run("Breaker interface open", func(t *testing.T) {
		// Arrange
		var b Breaker = &fakeBreakerState{state: "open"}

		// Act
		got := IsOpen(b)

		// Assert
		if !got {
			t.Fatalf("expected true for Breaker open")
		}
	})

	t.Run("Breaker interface closed", func(t *testing.T) {
		// Arrange
		var b Breaker = &fakeBreakerState{state: "closed"}

		// Act
		got := IsOpen(b)

		// Assert
		if got {
			t.Fatalf("expected false for Breaker closed")
		}
	})
}

func TestIsNilHelper(t *testing.T) {
	t.Run("nil any", func(t *testing.T) {
		// Arrange
		var v any = nil
		// Act
		got := isNil(v)
		// Assert
		if !got {
			t.Fatalf("expected true for nil")
		}
	})
	t.Run("typed nil chan", func(t *testing.T) {
		// Arrange
		var ch chan int = nil
		// Act
		got := isNil(ch)
		// Assert
		if !got {
			t.Fatalf("expected true for nil chan")
		}
	})
	t.Run("non-nil value", func(t *testing.T) {
		// Arrange
		v := &fakeStateOnly{state: "open"}
		// Act
		got := isNil(v)
		// Assert
		if got {
			t.Fatalf("expected false for non-nil")
		}
	})
	t.Run("int not nil", func(t *testing.T) {
		// Arrange
		v := 42
		// Act
		got := isNil(v)
		// Assert
		if got {
			t.Fatalf("expected false for int")
		}
	})
}
