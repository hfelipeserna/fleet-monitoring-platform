package domain_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/assistant/domain"
)

func strPtr(s string) *string { return &s }

func platePtr(s string) *shared.Plate {
	p := shared.Plate(s)
	return &p
}

func validChatRequest() domain.ChatRequest {
	return domain.ChatRequest{
		Message:    "hola",
		Limit:      domain.LimitMax,
		MinMinutes: 20,
	}
}

// Covers [SPEC-003: AC-009, BR-009, FR-003]
// TEST-009 — Step 1 Domain validation (TDD RED)
func TestChatRequest_Validate(t *testing.T) {
	t.Run("accepts message 1 char", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Message = "a"

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected 1 char message to be valid, got %v", err)
		}
	})

	t.Run("accepts message 4000 chars boundary", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Message = strings.Repeat("a", 4000)
		if utf8.RuneCountInString(req.Message) != 4000 {
			t.Fatalf("precondition rune count must be 4000")
		}

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected 4000 chars to be valid, got %v", err)
		}
	})

	t.Run("accepts message 4000 runes UTF-8 multibyte", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009] — UTF-8 rune count not byte length
		// Arrange
		req := validChatRequest()
		req.Message = strings.Repeat("🔥", 4000)

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected 4000 multibyte runes to be valid, got %v", err)
		}
	})

	t.Run("rejects empty message 0 chars", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Message = ""

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for empty message")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects message 4001 chars", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Message = strings.Repeat("a", 4001)

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for 4001 chars")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for 4001, got %v", err)
		}
	})

	t.Run("rejects message 4001 runes UTF-8", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Message = strings.Repeat("🔥", 4001)

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for 4001 runes multibyte")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for 4001 runes, got %v", err)
		}
	})

	t.Run("documents spaces count as chars", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Spec: message 1..4000 chars UTF-8, spaces count. Future BFF may trim, but domain validates raw length.
		// Currently a single space should be considered 1 char and pass; empty fails. This test documents behavior.
		// Arrange
		req := validChatRequest()
		req.Message = " "

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected single space to count as 1 char and be valid per current spec, got %v", err)
		}
	})

	t.Run("plate nil is OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Plate = nil

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected nil plate to be valid, got %v", err)
		}
	})

	t.Run("plate GTP980 valid", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Plate = platePtr("GTP980")

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected plate GTP980 to be valid, got %v", err)
		}
	})

	t.Run("plate too short GTP98 rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009] — regex ^[A-Z]{3}[0-9]{3}$
		// Arrange
		req := validChatRequest()
		req.Plate = platePtr("GTP98")

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for plate GTP98 (5 chars)")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for invalid plate, got %v", err)
		}
	})

	t.Run("plate lowercase gtp980 normalized via ParsePlate", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Uses shared.ParsePlate which normalizes to upper; Validate should accept lowercase by normalizing.
		// Arrange
		req := validChatRequest()
		req.Plate = platePtr("gtp980")

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected lowercase gtp980 to be normalized and valid via ParsePlate, got %v", err)
		}
		if string(*req.Plate) != "GTP980" {
			t.Fatalf("expected plate normalized to GTP980, got %v", *req.Plate)
		}
	})

	t.Run("plate invalid symbols rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		invalid := []string{"GTP-98", "ABC12", "123ABC", "GT 980", "AB1234"}
		for _, p := range invalid {
			p := p
			t.Run(p, func(t *testing.T) {
				// Arrange
				req := validChatRequest()
				req.Plate = platePtr(p)

				// Act
				err := req.Validate()

				// Assert
				if err == nil {
					t.Fatalf("expected error for plate %q", p)
				}
				if !errors.Is(err, shared.ErrValidation) {
					t.Fatalf("expected ErrValidation for %q, got %v", p, err)
				}
			})
		}
	})

	t.Run("zoneID nil is OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.ZoneID = nil

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected nil zoneID to be valid, got %v", err)
		}
	})

	t.Run("zoneID valid UUID v4 OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.ZoneID = strPtr("550e8400-e29b-41d4-a716-446655440000")

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected valid UUID to pass, got %v", err)
		}
	})

	t.Run("zoneID valid UUID upper case OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.ZoneID = strPtr("550E8400-E29B-41D4-A716-446655440000")

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected uppercase UUID to be valid, got %v", err)
		}
	})

	t.Run("zoneID bad UUID rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		bad := []string{"not-a-uuid", "550e8400-e29b-41d4-a716-44665544", "550e8400-e29b-41d4-a716-44665544000Z", "", "   "}
		for _, id := range bad {
			id := id
			t.Run(id, func(t *testing.T) {
				// Arrange
				req := validChatRequest()
				req.ZoneID = strPtr(id)

				// Act
				err := req.Validate()

				// Assert
				if err == nil {
					t.Fatalf("expected error for bad uuid %q", id)
				}
				if !errors.Is(err, shared.ErrValidation) {
					t.Fatalf("expected ErrValidation for bad uuid %q, got %v", id, err)
				}
			})
		}
	})

	t.Run("limit 0 rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Limit = 0

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for limit 0")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for limit 0, got %v", err)
		}
	})

	t.Run("limit 1 OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Limit = 1

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected limit 1 to be valid, got %v", err)
		}
	})

	t.Run("limit 20 OK boundary", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Limit = 20

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected limit 20 to be valid, got %v", err)
		}
	})

	t.Run("limit 21 rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.Limit = 21

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for limit 21")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for limit 21, got %v", err)
		}
	})

	t.Run("minMinutes 0 rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.MinMinutes = 0

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for minMinutes 0")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for minMinutes 0, got %v", err)
		}
	})

	t.Run("minMinutes 1 OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.MinMinutes = 1

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected minMinutes 1 to be valid, got %v", err)
		}
	})

	t.Run("minMinutes 1440 OK boundary", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.MinMinutes = 1440

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected minMinutes 1440 to be valid, got %v", err)
		}
	})

	t.Run("minMinutes 1441 rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.MinMinutes = 1441

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for minMinutes 1441")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for 1441, got %v", err)
		}
	})

	t.Run("sessionID nil OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.SessionID = nil

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected nil sessionID to be valid, got %v", err)
		}
	})

	t.Run("sessionID valid UUID OK", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.SessionID = strPtr("123e4567-e89b-12d3-a456-426614174000")

		// Act
		err := req.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected valid sessionID uuid to pass, got %v", err)
		}
	})

	t.Run("sessionID bad rejects", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		req := validChatRequest()
		req.SessionID = strPtr("not-a-uuid")

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for bad sessionID")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for bad sessionID, got %v", err)
		}
	})

	t.Run("multiple validation errors joined", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009] — errors must be joined with ErrValidation
		// Arrange
		req := validChatRequest()
		req.Message = ""
		req.Limit = 0
		req.MinMinutes = 0

		// Act
		err := req.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for multiple invalid fields")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation even with multiple errors, got %v", err)
		}
	})
}
