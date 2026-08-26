package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/shared/idgen"
)

var ErrMissingZoneID = errors.New("missing zoneID")

const (
	dedupWindow          = 2 * time.Minute
	dedupCleanupInterval = 30 * time.Second
	dedupThreshold       = 10000
)

type Publisher interface {
	Publish(ctx context.Context, alert fleet.Alert) error
}

type ZoneResolver interface {
	IsInside(ctx context.Context, plate string, lat, lon float64) (*string, *string, bool, error)
}

type Option func(*AlertDetector)

func WithClock(fn func() time.Time) Option {
	return func(d *AlertDetector) {
		d.clock = fn
	}
}

type AlertDetector struct {
	pub          Publisher
	resolver     ZoneResolver
	clock        func() time.Time
	mu           sync.Mutex
	prevSpeed    map[string]int
	prevInside   map[string]bool
	prevZoneID   map[string]*string
	prevZoneName map[string]*string
	dedup        map[string]time.Time
}

func NewAlertDetector(pub Publisher, resolver ZoneResolver, opts ...Option) *AlertDetector {
	d := &AlertDetector{
		pub:          pub,
		resolver:     resolver,
		prevSpeed:    make(map[string]int),
		prevInside:   make(map[string]bool),
		prevZoneID:   make(map[string]*string),
		prevZoneName: make(map[string]*string),
		dedup:        make(map[string]time.Time),
	}
	for _, o := range opts {
		o(d)
	}
	go d.StartCleanup(context.Background())
	return d
}

func (d *AlertDetector) now() time.Time {
	if d.clock != nil {
		return d.clock()
	}
	return time.Now().UTC()
}

func (d *AlertDetector) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(dedupCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			d.evict(t.UTC())
		}
	}
}

func (d *AlertDetector) evict(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, v := range d.dedup {
		if now.Sub(v) >= dedupWindow {
			delete(d.dedup, k)
		}
	}
}

func (d *AlertDetector) isDupLocked(key string, now time.Time) bool {
	if last, ok := d.dedup[key]; ok && now.Sub(last) < dedupWindow {
		return true
	}
	if len(d.dedup) > dedupThreshold {
		for k, v := range d.dedup {
			if now.Sub(v) >= dedupWindow {
				delete(d.dedup, k)
			}
		}
	}
	return false
}

func (d *AlertDetector) Process(ctx context.Context, plate string, lat, lon *float64, speed int) error {
	now := d.now()
	d.mu.Lock()
	prevSpeed, hasPrevSpeed := d.prevSpeed[plate]
	prevInside, hasPrevInside := d.prevInside[plate]
	prevZone := d.prevZoneID[plate]
	var prevZoneCopy *string
	if prevZone != nil {
		cp := *prevZone
		prevZoneCopy = &cp
	}
	prevZoneNameVal := d.prevZoneName[plate]
	var prevZoneNameCopy *string
	if prevZoneNameVal != nil {
		cp := *prevZoneNameVal
		prevZoneNameCopy = &cp
	}
	d.mu.Unlock()

	var inside bool
	var zoneID *string
	var zoneName *string
	if lat != nil && lon != nil {
		zid, zname, ins, err := d.resolveZone(ctx, plate, *lat, *lon)
		if err != nil {
			return err
		}
		inside = ins
		zoneID = zid
		zoneName = zname
	}

	if err := d.handleSpeed(ctx, plate, lat, lon, speed, now, prevSpeed, hasPrevSpeed); err != nil {
		return err
	}
	if lat == nil || lon == nil {
		return nil
	}
	if err := d.handleZone(ctx, plate, lat, lon, speed, now, prevInside, hasPrevInside, prevZoneCopy, prevZoneNameCopy, inside, zoneID, zoneName); err != nil {
		return err
	}
	return nil
}

func (d *AlertDetector) resolveZone(ctx context.Context, plate string, lat, lon float64) (*string, *string, bool, error) {
	zid, zname, inside, err := d.resolver.IsInside(ctx, plate, lat, lon)
	if err != nil {
		return nil, nil, false, fmt.Errorf("zone resolver: %w", err)
	}
	if zid != nil {
		cp := *zid
		zid = &cp
	}
	if zname != nil {
		cp := *zname
		zname = &cp
	}
	return zid, zname, inside, nil
}

func speedTransition(prev int, hasPrev bool, curr int) (string, bool) {
	if !hasPrev {
		return "", false
	}
	was := prev > 80
	is := curr > 80
	if !was && is {
		return "speeding_on", true
	}
	if was && !is {
		return "speeding_off", true
	}
	return "", false
}

func zoneTransition(prevInside bool, hasPrev bool, inside bool, zoneID *string, prevZoneID *string) (string, *string, bool) {
	prev := false
	if hasPrev {
		prev = prevInside
	}
	if !prev && inside {
		var zid *string
		if zoneID != nil {
			cp := *zoneID
			zid = &cp
		}
		return "zone_enter", zid, true
	}
	if prev && !inside {
		var zid *string
		if prevZoneID != nil {
			cp := *prevZoneID
			zid = &cp
		} else if zoneID != nil {
			cp := *zoneID
			zid = &cp
		}
		return "zone_exit", zid, true
	}
	return "", nil, false
}

func (d *AlertDetector) handleSpeed(ctx context.Context, plate string, lat, lon *float64, speed int, now time.Time, prev int, hasPrev bool) error {
	alertType, emit := speedTransition(prev, hasPrev, speed)
	if !emit {
		d.mu.Lock()
		d.prevSpeed[plate] = speed
		d.mu.Unlock()
		return nil
	}
	bucket := fleet.BucketFor(alertType, now)
	key := fmt.Sprintf("%s:%s:%d", plate, alertType, bucket)
	d.mu.Lock()
	dup := d.isDupLocked(key, now)
	if dup {
		d.prevSpeed[plate] = speed
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	rlat, rlon := roundLatLon(lat, lon)
	a := fleet.Alert{
		EventID:   idgen.GenerateUUID(),
		Plate:     plate,
		AlertType: alertType,
		ZoneID:    nil,
		Lat:       rlat,
		Lon:       rlon,
		Speed:     speed,
		CreatedAt: now,
	}
	if err := a.Validate(); err != nil {
		d.mu.Lock()
		d.prevSpeed[plate] = speed
		d.mu.Unlock()
		return fmt.Errorf("alert validate: %w", err)
	}
	if err := d.pub.Publish(ctx, a); err != nil {
		return fmt.Errorf("publish alert: %w", err)
	}
	d.mu.Lock()
	d.dedup[key] = now
	d.prevSpeed[plate] = speed
	d.mu.Unlock()
	return nil
}

func (d *AlertDetector) handleZone(ctx context.Context, plate string, lat, lon *float64, speed int, now time.Time, prevInside bool, hasPrev bool, prevZoneID *string, prevZoneName *string, inside bool, zoneID *string, zoneName *string) error {
	alertType, zoneOut, emit := zoneTransition(prevInside, hasPrev, inside, zoneID, prevZoneID)
	if !emit {
		d.mu.Lock()
		d.storeZoneStateLocked(plate, inside, zoneID, zoneName)
		d.mu.Unlock()
		return nil
	}
	if zoneOut == nil {
		d.mu.Lock()
		d.storeZoneStateLocked(plate, inside, zoneID, zoneName)
		d.mu.Unlock()
		return fmt.Errorf("zone_enter missing zoneID: %w", errors.Join(shared.ErrValidation, ErrMissingZoneID, fmt.Errorf("missing zone")))
	}
	var zoneNameOut *string
	if alertType == "zone_enter" {
		zoneNameOut = zoneName
	} else if alertType == "zone_exit" {
		if prevZoneName != nil {
			cp := *prevZoneName
			zoneNameOut = &cp
		} else if zoneName != nil {
			cp := *zoneName
			zoneNameOut = &cp
		} else if zoneOut != nil {
			zoneNameOut = nil
		}
	} else {
		zoneNameOut = zoneName
	}
	bucket := fleet.BucketFor(alertType, now)
	key := fmt.Sprintf("%s:%s:%s:%d", plate, alertType, *zoneOut, bucket)
	d.mu.Lock()
	dup := d.isDupLocked(key, now)
	if dup {
		d.storeZoneStateLocked(plate, inside, zoneID, zoneName)
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()
	if err := d.publishZoneAlert(ctx, plate, lat, lon, speed, now, alertType, zoneOut, zoneNameOut); err != nil {
		d.mu.Lock()
		d.storeZoneStateLocked(plate, inside, zoneID, zoneName)
		d.mu.Unlock()
		return err
	}
	d.mu.Lock()
	d.dedup[key] = now
	d.storeZoneStateLocked(plate, inside, zoneID, zoneName)
	d.mu.Unlock()
	return nil
}

func (d *AlertDetector) publishZoneAlert(ctx context.Context, plate string, lat, lon *float64, speed int, now time.Time, alertType string, zoneID *string, zoneName *string) error {
	rlat, rlon := roundLatLon(lat, lon)
	cp := *zoneID
	zid := &cp
	var zname *string
	if zoneName != nil {
		cp2 := *zoneName
		zname = &cp2
	}
	a := fleet.Alert{
		EventID:   idgen.GenerateUUID(),
		Plate:     plate,
		AlertType: alertType,
		ZoneID:    zid,
		ZoneName:  zname,
		Lat:       rlat,
		Lon:       rlon,
		Speed:     speed,
		CreatedAt: now,
	}
	if err := a.Validate(); err != nil {
		return fmt.Errorf("alert validate: %w", err)
	}
	if err := d.pub.Publish(ctx, a); err != nil {
		return fmt.Errorf("publish alert: %w", err)
	}
	return nil
}

func (d *AlertDetector) storeZoneStateLocked(plate string, inside bool, zoneID *string, zoneName *string) {
	d.prevInside[plate] = inside
	if inside && zoneID != nil {
		cp := *zoneID
		d.prevZoneID[plate] = &cp
		if zoneName != nil {
			cp2 := *zoneName
			d.prevZoneName[plate] = &cp2
		} else {
			d.prevZoneName[plate] = nil
		}
		return
	}
	if !inside {
		d.prevZoneID[plate] = nil
		d.prevZoneName[plate] = nil
	}
}

func roundLatLon(lat, lon *float64) (*float64, *float64) {
	var rlat, rlon *float64
	if lat != nil {
		v := shared.Round6(*lat)
		rlat = &v
	}
	if lon != nil {
		v := shared.Round6(*lon)
		rlon = &v
	}
	return rlat, rlon
}
