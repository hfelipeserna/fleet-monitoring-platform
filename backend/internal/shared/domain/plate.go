package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var plateRegex = regexp.MustCompile(`^[A-Z]{3}[0-9]{3}$`)

type Plate string

func ParsePlate(s string) (Plate, error) {
	normalized := strings.ToUpper(s)
	if !plateRegex.MatchString(normalized) {
		return "", fmt.Errorf("plate %q invalid: must match ^[A-Z]{3}[0-9]{3}$: %w", s, fmt.Errorf("invalid plate format"))
	}
	return Plate(normalized), nil
}

func (p Plate) String() string {
	return string(p)
}

func (p Plate) IsValid() bool {
	return plateRegex.MatchString(string(p))
}
