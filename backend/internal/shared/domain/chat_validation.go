package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	MessageMinRunes = 1
	MessageMaxRunes = 4000
)

func ValidateMessage(msg string) error {
	trimmed := strings.TrimSpace(msg)
	n := utf8.RuneCountInString(trimmed)
	if n < MessageMinRunes || n > MessageMaxRunes {
		return fmt.Errorf("message length %d invalid: must be %d..%d runes: %w", n, MessageMinRunes, MessageMaxRunes, ErrValidation)
	}
	return nil
}
