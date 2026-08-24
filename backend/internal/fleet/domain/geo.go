package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

var ErrValidation = shared.ErrValidation

var (
	ErrInvalidUUID       = errors.New("invalid uuid")
	ErrInvalidName       = errors.New("invalid name")
	ErrNotClosed         = errors.New("polygon not closed")
	ErrCoordCount        = errors.New("invalid coordinate count")
	ErrZeroArea          = errors.New("zero area")
	ErrSelfIntersection  = errors.New("self intersection")
	ErrNegativeSpeed     = errors.New("negative speed")
	ErrLatOutOfRange     = errors.New("latitude out of range")
	ErrLonOutOfRange     = errors.New("longitude out of range")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrZeroTime          = errors.New("zero time")
	ErrMissingZone       = errors.New("missing zone")
	ErrUnexpectedZone    = errors.New("unexpected zone")
	ErrInvalidAlertType  = errors.New("invalid alert type")
	ErrNotFound          = errors.New("not found")
	ErrInvalidPolygon    = errors.New("invalid polygon")
	ErrDuplicateZoneName = errors.New("duplicate zone name")
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func roundCoords(coords [][]float64) {
	for i := range coords {
		for j := range coords[i] {
			coords[i][j] = round6(coords[i][j])
		}
	}
}

func validateUUID(s string) error {
	if !uuidRegex.MatchString(s) {
		return fmt.Errorf("invalid uuid %q: %w", s, errors.Join(ErrInvalidUUID, ErrValidation))
	}
	return nil
}

func ValidateUUID(s string) error {
	return validateUUID(s)
}

func validateZoneName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("invalid name blank: %w", errors.Join(ErrInvalidName, ErrValidation))
	}
	n := utf8.RuneCountInString(trimmed)
	if n < 1 || n > 100 {
		return fmt.Errorf("invalid name length %d: %w", n, errors.Join(ErrInvalidName, ErrValidation))
	}
	return nil
}

func validateCoordinatesCountClosure(coords [][]float64) error {
	n := len(coords)
	if n < 4 || n > 101 {
		return fmt.Errorf("coordinates count %d invalid: %w", n, errors.Join(ErrCoordCount, ErrValidation))
	}
	if len(coords[0]) < 2 || len(coords[n-1]) < 2 {
		return fmt.Errorf("invalid coordinate dimension: %w", errors.Join(ErrCoordCount, ErrValidation))
	}
	if coords[0][0] != coords[n-1][0] || coords[0][1] != coords[n-1][1] {
		return fmt.Errorf("polygon not closed: %w", errors.Join(ErrNotClosed, ErrValidation))
	}
	return nil
}

func validateLonLat(lon, lat float64) error {
	if math.IsNaN(lon) || math.IsInf(lon, 0) {
		return fmt.Errorf("longitude invalid %v: %w", lon, errors.Join(ErrLonOutOfRange, ErrValidation))
	}
	if math.IsNaN(lat) || math.IsInf(lat, 0) {
		return fmt.Errorf("latitude invalid %v: %w", lat, errors.Join(ErrLatOutOfRange, ErrValidation))
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("longitude %v out of range: %w", lon, errors.Join(ErrLonOutOfRange, ErrValidation))
	}
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude %v out of range: %w", lat, errors.Join(ErrLatOutOfRange, ErrValidation))
	}
	return nil
}

func validatePolygonRange(coords [][]float64) error {
	for _, c := range coords {
		if len(c) < 2 {
			return fmt.Errorf("invalid coordinate dimension %v: %w", c, errors.Join(ErrCoordCount, ErrValidation))
		}
		if err := validateLonLat(c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func validateArea(coords [][]float64) error {
	area := polygonArea(coords)
	if math.Abs(area) < 1e-9 {
		return fmt.Errorf("zero area: %w", errors.Join(ErrZeroArea, ErrValidation))
	}
	return nil
}

func polygonArea(coords [][]float64) float64 {
	sum := 0.0
	n := len(coords)
	for i := 0; i < n-1; i++ {
		sum += coords[i][0]*coords[i+1][1] - coords[i+1][0]*coords[i][1]
	}
	return sum / 2
}

func validateSelfIntersection(coords [][]float64) error {
	n := len(coords)
	if n < 4 {
		return nil
	}
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n-1; j++ {
			if segmentsAdjacent(i, j, n) {
				continue
			}
			if segmentsIntersect(coords[i], coords[i+1], coords[j], coords[j+1]) {
				return fmt.Errorf("self intersection between %d and %d: %w", i, j, errors.Join(ErrSelfIntersection, ErrValidation))
			}
		}
	}
	return nil
}

func segmentsAdjacent(i, j, n int) bool {
	if j == i+1 {
		return true
	}
	if i == 0 && j == n-2 {
		return true
	}
	return false
}

func segmentsIntersect(p1, p2, p3, p4 []float64) bool {
	if len(p1) < 2 || len(p2) < 2 || len(p3) < 2 || len(p4) < 2 {
		return false
	}
	o1 := orientation(p1, p2, p3)
	o2 := orientation(p1, p2, p4)
	o3 := orientation(p3, p4, p1)
	o4 := orientation(p3, p4, p2)
	if o1 != o2 && o3 != o4 {
		return true
	}
	return segmentsIntersectColinear(o1, o2, o3, o4, p1, p2, p3, p4)
}

func segmentsIntersectColinear(o1, o2, o3, o4 int, p1, p2, p3, p4 []float64) bool {
	if o1 == 0 && onSegment(p1, p3, p2) {
		return true
	}
	if o2 == 0 && onSegment(p1, p4, p2) {
		return true
	}
	if o3 == 0 && onSegment(p3, p1, p4) {
		return true
	}
	if o4 == 0 && onSegment(p3, p2, p4) {
		return true
	}
	return false
}

func orientation(p, q, r []float64) int {
	v := (q[1]-p[1])*(r[0]-q[0]) - (q[0]-p[0])*(r[1]-q[1])
	if math.Abs(v) < 1e-9 {
		return 0
	}
	if v > 0 {
		return 1
	}
	return 2
}

func onSegment(p, q, r []float64) bool {
	if q[0] <= math.Max(p[0], r[0]) && q[0] >= math.Min(p[0], r[0]) && q[1] <= math.Max(p[1], r[1]) && q[1] >= math.Min(p[1], r[1]) {
		return true
	}
	return false
}

func validateSpeed(speed int) error {
	if speed < 0 {
		return fmt.Errorf("negative speed %d: %w", speed, errors.Join(ErrNegativeSpeed, ErrValidation))
	}
	return nil
}

func validateLat(lat *float64) error {
	if lat == nil {
		return nil
	}
	if math.IsNaN(*lat) || math.IsInf(*lat, 0) {
		return fmt.Errorf("latitude NaN/Inf %v: %w", *lat, errors.Join(ErrLatOutOfRange, ErrValidation))
	}
	rounded := round6(*lat)
	*lat = rounded
	if rounded < -90 || rounded > 90 {
		return fmt.Errorf("latitude %v out of range: %w", rounded, errors.Join(ErrLatOutOfRange, ErrValidation))
	}
	return nil
}

func validateLon(lon *float64) error {
	if lon == nil {
		return nil
	}
	if math.IsNaN(*lon) || math.IsInf(*lon, 0) {
		return fmt.Errorf("longitude NaN/Inf %v: %w", *lon, errors.Join(ErrLonOutOfRange, ErrValidation))
	}
	rounded := round6(*lon)
	*lon = rounded
	if rounded < -180 || rounded > 180 {
		return fmt.Errorf("longitude %v out of range: %w", rounded, errors.Join(ErrLonOutOfRange, ErrValidation))
	}
	return nil
}

func validateStatus(s string) error {
	if s == "" {
		return nil
	}
	if s == "moving" || s == "idle" || s == "alert" {
		return nil
	}
	return fmt.Errorf("invalid status %q: %w", s, errors.Join(ErrInvalidStatus, ErrValidation))
}

func validateAlertType(t string) error {
	if t == "zone_enter" || t == "zone_exit" || t == "speeding_on" || t == "speeding_off" {
		return nil
	}
	return fmt.Errorf("invalid alert type %q: %w", t, errors.Join(ErrInvalidAlertType, ErrValidation))
}

func validateAlertZoneID(alertType string, zoneID *string) error {
	isZone := alertType == "zone_enter" || alertType == "zone_exit"
	if isZone {
		if zoneID == nil || strings.TrimSpace(*zoneID) == "" {
			return fmt.Errorf("missing zone for %q: %w", alertType, errors.Join(ErrMissingZone, ErrValidation))
		}
		if err := validateUUID(*zoneID); err != nil {
			return err
		}
		return nil
	}
	if zoneID != nil {
		return fmt.Errorf("unexpected zone for %q: %w", alertType, errors.Join(ErrUnexpectedZone, ErrValidation))
	}
	return nil
}

func validateAlertLatLon(lat, lon *float64) error {
	if err := validateLat(lat); err != nil {
		return err
	}
	if err := validateLon(lon); err != nil {
		return err
	}
	return nil
}

func validateVehicleLatLon(lat, lon *float64) error {
	if err := validateLat(lat); err != nil {
		return err
	}
	if err := validateLon(lon); err != nil {
		return err
	}
	return nil
}

func validateTimeNonZero(t time.Time) error {
	if t.IsZero() {
		return fmt.Errorf("zero time: %w", errors.Join(ErrZeroTime, ErrValidation))
	}
	return nil
}

func validateReceivedAt(t time.Time) error { return validateTimeNonZero(t) }

func validateCreatedAt(t time.Time) error { return validateTimeNonZero(t) }
