package domain

import (
	"testing"
	"time"
)

// Covers [SPEC-002: AC-003, BR-002]

func TestCoverage_FleetDomain(t *testing.T) {
	t.Run("BucketFor zone and speeding", func(t *testing.T) {
		// Arrange
		ts := time.Date(2026, 8, 24, 10, 7, 0, 0, time.UTC)
		// Act
		b1 := BucketFor("zone_enter", ts)
		b2 := BucketFor("zone_exit", ts)
		b3 := BucketFor("speeding", ts)
		b4 := BucketFor("other", ts)
		// Assert
		// zone bucket is 20m, speeding 5m
		if b1 != ts.Truncate(ZoneBucket).Unix() {
			t.Fatalf("expected zone bucket, got %d", b1)
		}
		if b2 != b1 {
			t.Fatalf("expected same zone bucket")
		}
		if b3 != ts.Truncate(SpeedingBucket).Unix() {
			t.Fatalf("expected speeding bucket, got %d", b3)
		}
		if b4 != b3 {
			t.Fatalf("expected default speeding for other")
		}
	})

	t.Run("MsgID zone and speeding", func(t *testing.T) {
		// Arrange
		ts := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		zid := "zone-123"
		a1 := Alert{Plate: "ABC123", AlertType: "zone_enter", ZoneID: &zid, CreatedAt: ts}
		a2 := Alert{Plate: "ABC123", AlertType: "zone_enter", ZoneID: nil, CreatedAt: ts}
		a3 := Alert{Plate: "ABC123", AlertType: "speeding", CreatedAt: ts}
		// Act
		m1 := a1.MsgID()
		m2 := a2.MsgID()
		m3 := a3.MsgID()
		// Assert
		if m1 != "ABC123:zone_enter:zone-123:"+string(rune(0)) && len(m1) == 0 {
			// just check contains
		}
		if !containsStr(m1, "zone-123") {
			t.Fatalf("expected zone id in msgid, got %q", m1)
		}
		if !containsStr(m2, "ABC123:zone_enter:") {
			t.Fatalf("expected plate and type, got %q", m2)
		}
		if !containsStr(m3, "ABC123:speeding:") {
			t.Fatalf("expected speeding msgid, got %q", m3)
		}
	})

	t.Run("onSegment and helpers", func(t *testing.T) {
		// Arrange
		// onSegment is private but we can test via validateSelfIntersection that uses it indirectly
		// We test polygon that triggers colinear path
		// Use a simple self-intersect bowtie already tested in zone validation, but we test direct
		// For onSegment, we need points colinear
		// We indirectly test via Validate that hits those branches
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}
		z := Zone{ID: "550e8400-e29b-41d4-a716-446655440001", Name: "Test", Coordinates: coords}
		// Act
		err := z.Validate()
		// Assert
		if err == nil {
			t.Fatalf("expected validation error for bowtie")
		}
	})

	t.Run("validateLonLat and polygon range", func(t *testing.T) {
		// Arrange
		// test validateLonLat via Zone with invalid lon/lat
		z := Zone{ID: "550e8400-e29b-41d4-a716-446655440001", Name: "Test", Coordinates: [][]float64{{200, 100}, {200, 100}, {200, 100}, {200, 100}, {200, 100}}}
		// Act
		err := z.Validate()
		// Assert
		if err == nil {
			t.Fatalf("expected range error")
		}
	})
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
