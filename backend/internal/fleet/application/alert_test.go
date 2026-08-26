package application_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"fleetmonitoring/backend/internal/fleet/application"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
)

// Covers [SPEC-002: AC-005, BR-004/011]
// Step4 RED: detector de 4 tipos zone_enter/zone_exit/speeding_on/speeding_off
// con bucket 20m zone y 5m speeding, MsgId plate:alert_type:bucket y dedup 2m.
// Esta suite FALLA intencionalmente hasta que exista application/alert.go
// con NewAlertDetector, Publisher, ZoneResolver y Process (TDD RED).

// ---- fakes de consumidor ----

type fakePublisher struct {
	published []fleet.Alert
	// opcional: capturar MsgId si el publisher lo expone; si solo Publish(alert) se verifica vía conteo+tipo
	msgIDs []string
	err    error
}

func (f *fakePublisher) Publish(ctx context.Context, alert fleet.Alert) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, alert)
	// MsgId esperado plate:alert_type:bucket se deriva del alert y el clock del detector;
	// el fake no lo calcula, solo registra el alert. El dedup se verifica por conteo.
	return nil
}

// publishWithMsgID es variante si la implementación expone MsgID como segundo arg.
// Lo dejamos como helper para que el reviewer vea la expectativa de Nats-Msg-Id.
func (f *fakePublisher) PublishWithMsgID(ctx context.Context, alert fleet.Alert, msgID string) error {
	f.published = append(f.published, alert)
	f.msgIDs = append(f.msgIDs, msgID)
	return nil
}

type stubZoneResolver struct {
	// secuencia de respuestas para llamadas sucesivas
	responses []struct {
		zoneID   *string
		zoneName *string
		inside   bool
		err      error
	}
	calls int
	fn    func(ctx context.Context, plate string, lat, lon float64) (*string, *string, bool, error)
}

func (s *stubZoneResolver) IsInside(ctx context.Context, plate string, lat, lon float64) (*string, *string, bool, error) {
	if s.fn != nil {
		return s.fn(ctx, plate, lat, lon)
	}
	if s.calls < len(s.responses) {
		r := s.responses[s.calls]
		s.calls++
		return r.zoneID, r.zoneName, r.inside, r.err
	}
	return nil, nil, false, nil
}

func alertStrPtr(s string) *string { return &s }
func alertF64Ptr(v float64) *float64 { return &v }

const testZoneID = "550e8400-e29b-41d4-a716-446655440002"
var testZoneName = "Zona Norte"
const testPlate = "GTP980"

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// newTestDetector documenta la API esperada en GREEN.
// Expected GREEN signature:
//   type Publisher interface { Publish(ctx context.Context, alert fleet.Alert) error }
//   type ZoneResolver interface { IsInside(ctx context.Context, plate string, lat, lon float64) (*string, bool, error) }
//   func NewAlertDetector(pub Publisher, resolver ZoneResolver, opts ...Option) *AlertDetector
//   func WithClock(func() time.Time) Option
//   func (d *AlertDetector) Process(ctx context.Context, plate string, lat, lon *float64, speed int) error
// Si tu implementación usa otra firma (ej. ProcessTelemetry con fleet.VehiclePos), adapta este helper
// pero mantén el contrato de bucket y dedup.
func newTestDetector(pub *fakePublisher, resolver *stubZoneResolver, clock func() time.Time) *application.AlertDetector {
	// Arrange helper: construye detector con clock determinístico para bucket.
	// RED: esta llamada falla hasta que application/alert.go exista.
	if clock == nil {
		clock = time.Now
	}
	// Se espera WithClock option; si tu implementación recibe clock como 3er arg, cambia aquí.
	return application.NewAlertDetector(pub, resolver, application.WithClock(clock))
}

// Covers [SPEC-002: AC-005, BR-004/011]
func TestAlertDetector(t *testing.T) {
	t.Run("speeding_on publish prev 70 -> curr 85 single Publish", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-011]
		// Arrange
		now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{} // ST_Within false (fuera de zona) para aislar speeding
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		lat := alertF64Ptr(4.711)
		lon := alertF64Ptr(-74.072)
		// primera telemetría 70 <=80 no debe publicar, deja prevSpeed=70
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat, lon, 85)

		// Assert
		// AC-005: speeding_on cuando speed>80 tras <=80
		if err != nil {
			t.Fatalf("expected no error for speeding_on, got %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 publish for speeding_on 70->85, got %d", len(pub.published)) // AC-005
		}
		got := pub.published[0]
		if got.AlertType != "speeding_on" {
			t.Fatalf("expected alert_type speeding_on, got %q", got.AlertType) // AC-005
		}
		if got.Plate != testPlate {
			t.Fatalf("expected plate %q, got %q", testPlate, got.Plate)
		}
		if got.Speed != 85 {
			t.Fatalf("expected speed 85, got %d", got.Speed)
		}
		if got.ZoneID != nil {
			t.Fatalf("expected ZoneID nil for speeding_on, got %v", *got.ZoneID) // BR-004
		}
		// Nats-Msg-Id plate:speeding_on:bucket (bucket 5m) se verifica implícito vía conteo y dedup;
		// GREEN debe generar MsgId fmt.Sprintf("%s:%s:%d", plate, alertType, bucket5m)
		expectedBucket := now.Truncate(5 * time.Minute).Unix() / 300
		_ = fmt.Sprintf("%s:%s:%d", testPlate, "speeding_on", expectedBucket) // documenta formato esperado
	})

	t.Run("speeding_on no duplicate prev 85 -> curr 90", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-011, BR-004]
		// Arrange
		now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		lat := alertF64Ptr(4.711)
		lon := alertF64Ptr(-74.072)
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		_ = det.Process(ctx, testPlate, lat, lon, 85) // deja prev=85 y publica speeding_on
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat, lon, 90)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected 0 publish for 85->90 (ya >80), got %d", len(pub.published)) // AC-005 BR-011
		}
	})

	t.Run("speeding_off publish prev 85 -> curr 70", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-011]
		// Arrange
		now := time.Date(2026, 8, 24, 10, 5, 0, 0, time.UTC)
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		lat := alertF64Ptr(4.711)
		lon := alertF64Ptr(-74.072)
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		_ = det.Process(ctx, testPlate, lat, lon, 85) // prev 85
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat, lon, 70)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for speeding_off, got %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 publish for speeding_off 85->70, got %d", len(pub.published)) // AC-005
		}
		if pub.published[0].AlertType != "speeding_off" {
			t.Fatalf("expected speeding_off, got %q", pub.published[0].AlertType) // AC-005 BR-011
		}
		if pub.published[0].Plate != testPlate {
			t.Fatalf("expected plate %q, got %q", testPlate, pub.published[0].Plate)
		}
	})

	t.Run("speeding_off no duplicate prev 70 -> curr 60", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-011]
		// Arrange
		now := time.Date(2026, 8, 24, 10, 10, 0, 0, time.UTC)
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		lat := alertF64Ptr(4.711)
		lon := alertF64Ptr(-74.072)
		_ = det.Process(ctx, testPlate, lat, lon, 70) // prev 70
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat, lon, 60)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected 0 publish for 70->60 (ya <=80), got %d", len(pub.published)) // AC-005
		}
	})

	t.Run("zone_enter ST_Within false->true publish with zone_id", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004]
		// Arrange
		now := time.Date(2026, 8, 24, 10, 20, 0, 0, time.UTC)
		pub := &fakePublisher{}
		zid := testZoneID
		resolver := &stubZoneResolver{
			responses: []struct {
				zoneID   *string
				zoneName *string
				inside   bool
				err      error
			}{
				{zoneID: nil, inside: false},
				{zoneID: &zid, zoneName: &testZoneName, inside: true},
			},
		}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		speed := 50 // <=80 para no mezclar speeding
		lat1 := alertF64Ptr(4.70)
		lon1 := alertF64Ptr(-74.07)
		lat2 := alertF64Ptr(4.72)
		lon2 := alertF64Ptr(-74.06)
		_ = det.Process(ctx, testPlate, lat1, lon1, speed) // false, prevInside=false
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat2, lon2, speed)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for zone_enter, got %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 publish for zone_enter false->true, got %d", len(pub.published)) // AC-005 BR-004
		}
		got := pub.published[0]
		if got.AlertType != "zone_enter" {
			t.Fatalf("expected zone_enter, got %q", got.AlertType) // AC-005
		}
		if got.ZoneID == nil || *got.ZoneID != testZoneID {
			t.Fatalf("expected ZoneID %q, got %v", testZoneID, got.ZoneID) // BR-004 zone_enter con zone_id
		}
		if got.Plate != testPlate {
			t.Fatalf("expected plate %q, got %q", testPlate, got.Plate)
		}
		// Nats-Msg-Id plate:zone_enter:bucket con bucket 20m (zone)
		expectedBucket := now.Truncate(20 * time.Minute).Unix() / 1200
		_ = fmt.Sprintf("%s:%s:%s:%d", testPlate, "zone_enter", testZoneID, expectedBucket)
	})

	t.Run("zone_exit true->false publish with zone_id", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004]
		// Arrange
		now := time.Date(2026, 8, 24, 10, 40, 0, 0, time.UTC)
		pub := &fakePublisher{}
		zid := testZoneID
		resolver := &stubZoneResolver{
			responses: []struct {
				zoneID   *string
				zoneName *string
				inside   bool
				err    error
			}{
				{zoneID: &zid, zoneName: &testZoneName, inside: true},
				{zoneID: nil, inside: false},
			},
		}
		// start clock at now, but we need prevInside=true. We achieve via first Process with inside=true
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		speed := 50
		latInside := alertF64Ptr(4.72)
		lonInside := alertF64Ptr(-74.06)
		latOutside := alertF64Ptr(4.70)
		lonOutside := alertF64Ptr(-74.07)
		// first call inside true -> prevInside true, but first transition nil->true would have published zone_enter;
		// we reset publish to isolate exit
		_ = det.Process(ctx, testPlate, latInside, lonInside, speed)
		pub.published = nil
		// reset resolver to return false for next call (already queued as second response)
		// Act: outside
		err := det.Process(ctx, testPlate, latOutside, lonOutside, speed)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for zone_exit, got %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 publish for zone_exit true->false, got %d", len(pub.published)) // AC-005
		}
		if pub.published[0].AlertType != "zone_exit" {
			t.Fatalf("expected zone_exit, got %q", pub.published[0].AlertType) // AC-005
		}
		if pub.published[0].ZoneID == nil || *pub.published[0].ZoneID != testZoneID {
			t.Fatalf("expected ZoneID %q for zone_exit, got %v", testZoneID, pub.published[0].ZoneID)
		}
	})

	t.Run("zone no publish si sin cambio false->false", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004]
		// Arrange
		now := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{
			responses: []struct {
				zoneID   *string
				zoneName *string
				inside   bool
				err    error
			}{
				{zoneID: nil, inside: false},
				{zoneID: nil, inside: false},
			},
		}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		speed := 30
		lat := alertF64Ptr(4.70)
		lon := alertF64Ptr(-74.07)
		_ = det.Process(ctx, testPlate, lat, lon, speed) // first false
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat, lon, speed)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected 0 publish for false->false, got %d", len(pub.published)) // AC-005 no evento si sin cambio
		}
	})

	t.Run("zone no publish si sin cambio true->true", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004]
		// Arrange
		now := time.Date(2026, 8, 24, 11, 20, 0, 0, time.UTC)
		pub := &fakePublisher{}
		zid := testZoneID
		resolver := &stubZoneResolver{
			responses: []struct {
				zoneID   *string
				zoneName *string
				inside   bool
				err    error
			}{
				{zoneID: &zid, zoneName: &testZoneName, inside: true},
				{zoneID: &zid, zoneName: &testZoneName, inside: true},
			},
		}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		speed := 30
		lat := alertF64Ptr(4.72)
		lon := alertF64Ptr(-74.06)
		_ = det.Process(ctx, testPlate, lat, lon, speed) // first true -> publishes enter, reset
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, lat, lon, speed)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected 0 publish for true->true, got %d", len(pub.published)) // AC-005
		}
	})

	t.Run("dedup MsgId 2m mismo plate:alert_type:bucket segundo Process no publica", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004]
		// Arrange
		now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		pub := &fakePublisher{}
		zid := testZoneID
		// Resolver que permite re-disparar zone_enter con mismo bucket:
		// secuencia false->true (publish), true->false (publish exit pero diferente tipo), false->true (intenta re-publish mismo zone_enter mismo bucket -> dedup)
		// Para aislar speeding, usamos speeding_on dedup: 70->85 (publish), luego forzamos prev a 70 y 70->85 de nuevo mismo bucket -> dedup
		// Como prevSpeed es privado, simulamos re-disparo bajando y subiendo dentro de la misma ventana 2m.
		resolver := &stubZoneResolver{} // fuera de zona siempre
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		lat := alertF64Ptr(4.711)
		lon := alertF64Ptr(-74.072)
		// priming: 70->85 publish speeding_on
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		_ = det.Process(ctx, testPlate, lat, lon, 85)
		if len(pub.published) != 1 {
			t.Fatalf("arrange expected 1 speeding_on, got %d", len(pub.published))
		}
		// speeding_off para volver a <=80 (diferente alert_type, no afecta dedup de speeding_on)
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		// ahora pub tiene 2 (on + off). Guardamos conteo de speeding_on
		speedingOnCount := 0
		for _, a := range pub.published {
			if a.AlertType == "speeding_on" {
				speedingOnCount++
			}
		}
		if speedingOnCount != 1 {
			t.Fatalf("expected 1 speeding_on before dedup, got %d", speedingOnCount)
		}
		// Act: intenta segundo speeding_on mismo bucket 5m (mismo plate:speeding_on:bucket)
		err := det.Process(ctx, testPlate, lat, lon, 85)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// AC-005 BR-004: Nats-Msg-Id duplicate window 2m no duplica
		speedingOnCountAfter := 0
		for _, a := range pub.published {
			if a.AlertType == "speeding_on" {
				speedingOnCountAfter++
			}
		}
		if speedingOnCountAfter != 1 {
			t.Fatalf("expected dedup: second speeding_on same bucket 5m dentro de 2m no debe publicar, got %d speeding_on total %d published", speedingOnCountAfter, len(pub.published)) // AC-005 BR-004
		}
		// adicional: si se expone MsgID, debe ser plate:speeding_on:bucket
		_ = zid
	})

	t.Run("speeding bucket 5m distinta bucket => nuevo publish", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004/011]
		// Arrange
		t0 := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		t1 := time.Date(2026, 8, 24, 10, 6, 0, 0, time.UTC) // +6m cruza bucket 5m
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{}
		// detector con clock mutable
		current := t0
		clock := func() time.Time { return current }
		det := newTestDetector(pub, resolver, clock)
		ctx := context.Background()
		lat := alertF64Ptr(4.711)
		lon := alertF64Ptr(-74.072)
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		_ = det.Process(ctx, testPlate, lat, lon, 85) // bucket t0 -> publish 1
		if len(pub.published) != 1 {
			t.Fatalf("arrange expected 1 publish at t0, got %d", len(pub.published))
		}
		// volver a <=80 para permitir nuevo on en siguiente bucket
		_ = det.Process(ctx, testPlate, lat, lon, 70) // speeding_off bucket t0
		// mover clock a t1 (nuevo bucket 5m)
		current = t1

		// Act
		err := det.Process(ctx, testPlate, lat, lon, 85)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// AC-005: misma hora bucket distinta => nuevo publish (no dedup)
		speedingOnCount := 0
		for _, a := range pub.published {
			if a.AlertType == "speeding_on" {
				speedingOnCount++
			}
		}
		if speedingOnCount != 2 {
			t.Fatalf("expected 2 speeding_on en buckets 5m distintos, got %d total %d", speedingOnCount, len(pub.published)) // AC-005 BR-004 BR-011
		}
	})

	t.Run("zone bucket 20m distinta bucket => nuevo publish zone_enter", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004]
		// Arrange
		t0 := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		t1 := time.Date(2026, 8, 24, 10, 21, 0, 0, time.UTC) // +21m cruza bucket 20m
		pub := &fakePublisher{}
		zid := testZoneID
		// secuencia: t0 false->true publish, luego true->false, luego false->true en t1 (nuevo bucket)
		responses := []struct {
			zoneID   *string
			zoneName *string
			inside   bool
			err    error
		}{
			{zoneID: nil, inside: false},    // priming false
			{zoneID: &zid, zoneName: &testZoneName, inside: true},    // t0 enter
			{zoneID: nil, inside: false},    // exit
			{zoneID: &zid, zoneName: &testZoneName, inside: true},    // t1 enter nuevo bucket
		}
		current := t0
		clock := func() time.Time { return current }
		resolver := &stubZoneResolver{responses: responses}
		det := newTestDetector(pub, resolver, clock)
		ctx := context.Background()
		speed := 40
		latOut := alertF64Ptr(4.70)
		lonOut := alertF64Ptr(-74.07)
		latIn := alertF64Ptr(4.72)
		lonIn := alertF64Ptr(-74.06)
		_ = det.Process(ctx, testPlate, latOut, lonOut, speed) // false
		_ = det.Process(ctx, testPlate, latIn, lonIn, speed)   // true -> publish 1
		if len(pub.published) != 1 {
			t.Fatalf("arrange expected 1 zone_enter at t0, got %d", len(pub.published))
		}
		_ = det.Process(ctx, testPlate, latOut, lonOut, speed) // exit -> publish zone_exit (2)
		if len(pub.published) != 2 {
			t.Fatalf("arrange expected 2 after exit, got %d", len(pub.published))
		}
		current = t1

		// Act
		err := det.Process(ctx, testPlate, latIn, lonIn, speed)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(pub.published) != 3 {
			t.Fatalf("expected 3 total (second zone_enter en nuevo bucket 20m), got %d", len(pub.published)) // AC-005 BR-004
		}
		if pub.published[2].AlertType != "zone_enter" {
			t.Fatalf("expected zone_enter in new bucket, got %q", pub.published[2].AlertType)
		}
	})

	t.Run("lat lon round6 PII 6dec", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-010]
		// Arrange
		now := time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)
		pub := &fakePublisher{}
		resolver := &stubZoneResolver{}
		det := newTestDetector(pub, resolver, fixedClock(now))
		ctx := context.Background()
		// coords con 7+ decimales
		latRaw := 4.71111119
		lonRaw := -74.07222229
		lat := alertF64Ptr(latRaw)
		lon := alertF64Ptr(lonRaw)
		_ = det.Process(ctx, testPlate, lat, lon, 70)
		pub.published = nil

		// Act
		err := det.Process(ctx, testPlate, alertF64Ptr(latRaw), alertF64Ptr(lonRaw), 85)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected 1 publish, got %d", len(pub.published))
		}
		got := pub.published[0]
		if got.Lat == nil || math.Abs(*got.Lat-4.711111) > 1e-9 {
			t.Fatalf("expected lat rounded to 4.711111, got %v", got.Lat) // BR-010 6dec
		}
		if got.Lon == nil || math.Abs(*got.Lon-(-74.072222)) > 1e-9 {
			t.Fatalf("expected lon rounded to -74.072222, got %v", got.Lon) // BR-010
		}
		// Validate también debe pasar con datos redondeados
		if err := got.Validate(); err != nil {
			t.Fatalf("expected published alert to be valid after round6, got %v", err)
		}
	})
}
