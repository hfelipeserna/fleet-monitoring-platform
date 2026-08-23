package domain_test

import (
	"testing"

	"fleetmonitoring/backend/internal/shared/domain"
)

// Covers [SPEC-001: AC-004, FR-004, BR-001]
// TEST-004 — Plate validation: regex ^[A-Z]{3}[0-9]{3}$, normalized uppercase.
// Valid: GTP890, TTY423. Invalid: GTP89, 123, empty, wrong length, symbols.

func TestPlate(t *testing.T) {
	t.Run("accepts valid plates", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-001, BR-001]
		valid := []string{"GTP890", "TTY423", "ABC123", "ZZZ999", "AAA000"}
		for _, input := range valid {
			input := input
			t.Run(input, func(t *testing.T) {
				// Arrange
				// Act
				plate, err := domain.ParsePlate(input)

				// Assert
				if err != nil {
					t.Fatalf("expected valid plate %q to parse without error, got %v", input, err)
				}
				if string(plate) != input {
					t.Fatalf("expected plate %q got %q", input, string(plate))
				}
				if !plate.IsValid() {
					t.Fatalf("expected IsValid true for %q", input)
				}
			})
		}
	})

	t.Run("normalizes lowercase to uppercase", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, BR-001]
		cases := []struct {
			input string
			want  string
		}{
			{"gtp890", "GTP890"},
			{"tty423", "TTY423"},
			{"abc123", "ABC123"},
			{"Gtp890", "GTP890"},
			{"gTp890", "GTP890"},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.input, func(t *testing.T) {
				// Arrange
				// Act
				plate, err := domain.ParsePlate(tc.input)

				// Assert
				if err != nil {
					t.Fatalf("expected %q to normalize and be valid, got error %v", tc.input, err)
				}
				if string(plate) != tc.want {
					t.Fatalf("expected normalized %q got %q", tc.want, string(plate))
				}
			})
		}
	})

	t.Run("normalizes and validates trimmed input", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, BR-001]
		// Arrange
		input := " gtp890 "

		// Act
		plate, err := domain.ParsePlate(input)

		// Assert
		if err == nil {
			t.Fatalf("expected error for plate with spaces %q, got plate %q", input, plate)
		}
	})

	t.Run("rejects invalid plates", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-001]
		invalid := []struct {
			name  string
			input string
		}{
			{"too short GTP89", "GTP89"},
			{"numeric only 123", "123"},
			{"too long GTP8901", "GTP8901"},
			{"too long 4 letters", "ABCD123"},
			{"too short 2 letters", "AB1234"},
			{"lower invalid length", "ab123"},
			{"with dash", "GT-890"},
			{"with space", "GTP 890"},
			{"empty", ""},
			{"whitespace", "   "},
			{"letters only", "ABCDEF"},
			{"digits only 6", "123456"},
			{"mix wrong pattern ABC12", "ABC12"},
			{"mix wrong pattern 123ABC", "123ABC"},
			{"special char", "GTP89@ "},
			{"lower too short gtp89", "gtp89"},
		}
		for _, tc := range invalid {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				// Act
				plate, err := domain.ParsePlate(tc.input)

				// Assert
				if err == nil {
					t.Fatalf("expected error for invalid plate %q, got plate %q", tc.input, plate)
				}
			})
		}
	})

	t.Run("String returns normalized form", func(t *testing.T) {
		// Covers [SPEC-001: BR-001]
		// Arrange
		input := "gtp890"

		// Act
		plate, err := domain.ParsePlate(input)

		// Assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plate.String() != "GTP890" {
			t.Fatalf("expected String() GTP890 got %q", plate.String())
		}
	})

	t.Run("IsValid matches ParsePlate validity", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, BR-001]
		// Arrange
		validPlate := domain.Plate("GTP890")
		invalidPlate := domain.Plate("GTP89")

		// Act
		valid := validPlate.IsValid()
		invalid := invalidPlate.IsValid()

		// Assert
		if !valid {
			t.Fatalf("expected IsValid true for GTP890")
		}
		if invalid {
			t.Fatalf("expected IsValid false for GTP89")
		}
	})
}
