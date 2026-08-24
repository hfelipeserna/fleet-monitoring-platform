package domain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

func Round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func Round3(v float64) float64 {
	return math.Round(v*1e3) / 1e3
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

func EncodeCursor(plate string, t time.Time) string {
	raw := fmt.Sprintf("%s|%s", plate, t.Format(time.RFC3339Nano))
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(cursor string) (Plate, time.Time, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cursor base64 decode failed: %w", errors.Join(ErrValidation, err))
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return "", time.Time{}, fmt.Errorf("cursor format invalid: %w", ErrValidation)
	}
	p, err := ParsePlate(parts[0])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cursor plate invalid: %w", errors.Join(ErrValidation, err))
	}
	tt, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cursor time invalid: %w", errors.Join(ErrValidation, err))
	}
	return p, tt, nil
}
