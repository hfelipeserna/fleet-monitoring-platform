package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/sony/gobreaker"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/shared/idgen"
)

type zoneService interface {
	Create(ctx context.Context, name string, coords [][]float64) (fleet.Zone, error)
	List(ctx context.Context) ([]fleet.Zone, error)
	Update(ctx context.Context, id string, name string, coords [][]float64) (fleet.Zone, error)
	Delete(ctx context.Context, id string) error
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

type ZoneHandler struct {
	svc       zoneService
	limiters  sync.Map
	sweepOnce sync.Once
	mux       *http.ServeMux
}

func NewZoneHandler(svc zoneService) http.Handler {
	h := &ZoneHandler{svc: svc, mux: http.NewServeMux()}
	h.mux.HandleFunc("/api/zones", h.handleZones)
	h.mux.HandleFunc("/api/zones/", h.handleZoneByID)
	return h
}

func (h *ZoneHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *ZoneHandler) startSweep() {
	h.sweepOnce.Do(func() {
		go func() {
			for {
				time.Sleep(10 * time.Minute)
				now := time.Now().UnixNano()
				h.limiters.Range(func(k, v any) bool {
					e := v.(*limiterEntry)
					if now-e.lastSeen.Load() > int64(10*time.Minute) {
						h.limiters.Delete(k)
					}
					return true
				})
			}
		}()
	})
}

func (h *ZoneHandler) getLimiter(ip string) *rate.Limiter {
	if v, ok := h.limiters.Load(ip); ok {
		e := v.(*limiterEntry)
		e.lastSeen.Store(time.Now().UnixNano())
		return e.limiter
	}
	h.startSweep()
	e := &limiterEntry{limiter: rate.NewLimiter(rate.Every(time.Minute/10), 10)}
	e.lastSeen.Store(time.Now().UnixNano())
	actual, loaded := h.limiters.LoadOrStore(ip, e)
	if loaded {
		ae := actual.(*limiterEntry)
		ae.lastSeen.Store(time.Now().UnixNano())
		return ae.limiter
	}
	return e.limiter
}

func isTrustedProxy(remoteAddr string) bool {
	host := remoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

func getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" && isTrustedProxy(r.RemoteAddr) {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		ip := host[:idx]
		ip = strings.Trim(ip, "[]")
		if net.ParseIP(ip) != nil {
			return ip
		}
		return ip
	}
	if net.ParseIP(host) != nil {
		return host
	}
	return host
}

func (h *ZoneHandler) allowWrite(w http.ResponseWriter, r *http.Request) bool {
	ip := getIP(r)
	lim := h.getLimiter(ip)
	if !lim.Allow() {
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited", "message": "too many requests"})
		return false
	}
	return true
}

type geoJSON struct {
	Type        string          `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

type zoneRequest struct {
	Name     string   `json:"name"`
	GeoJSON  *geoJSON `json:"geojson"`
	Geometry *geoJSON `json:"geometry"`
}

func isMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return true
	}
	return strings.Contains(err.Error(), "request body too large")
}

func decodeZoneRequest(r *http.Request) (zoneRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return zoneRequest{}, fmt.Errorf("read body failed: %w", err)
	}
	var req zoneRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return zoneRequest{}, fmt.Errorf("invalid json: %w", errors.Join(shared.ErrValidation, err))
	}
	return req, nil
}

func extractCoords(req zoneRequest) ([][]float64, error) {
	g := req.GeoJSON
	if g == nil {
		g = req.Geometry
	}
	if g == nil {
		return nil, fmt.Errorf("missing geojson: %w", shared.ErrValidation)
	}
	if g.Type != "Polygon" {
		return nil, fmt.Errorf("invalid geojson type: %w", shared.ErrValidation)
	}
	if len(g.Coordinates) == 0 {
		return nil, fmt.Errorf("missing coordinates: %w", shared.ErrValidation)
	}
	return g.Coordinates[0], nil
}

func validateZoneCoords(name string, coords [][]float64) error {
	dummyID := idgen.GenerateUUID()
	z := fleet.Zone{ID: dummyID, Name: name, Coordinates: coords}
	if err := z.Validate(); err != nil {
		return fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, err))
	}
	return nil
}

func parseZoneRequest(r *http.Request) (string, [][]float64, error) {
	req, err := decodeZoneRequest(r)
	if err != nil {
		return "", nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 100 {
		return "", nil, fmt.Errorf("invalid name: %w", errors.Join(fleet.ErrInvalidName, shared.ErrValidation))
	}
	coords, err := extractCoords(req)
	if err != nil {
		return "", nil, err
	}
	if err := validateZoneCoords(name, coords); err != nil {
		return "", nil, err
	}
	return name, coords, nil
}

func validateZoneID(id string) error {
	if err := fleet.ValidateUUID(id); err != nil {
		return fmt.Errorf("invalid uuid: %w", errors.Join(shared.ErrValidation, err))
	}
	return nil
}

func mapZoneError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, fleet.ErrDuplicateZoneName) {
		return http.StatusConflict
	}
	if errors.Is(err, fleet.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, shared.ErrValidation) || errors.Is(err, fleet.ErrValidation) {
		return http.StatusBadRequest
	}
	if errors.Is(err, gobreaker.ErrOpenState) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func handleZoneError(w http.ResponseWriter, err error) bool {
	code := mapZoneError(err)
	switch code {
	case http.StatusConflict:
		writeJSON(w, code, map[string]string{"error": "zone name already exists"})
	case http.StatusServiceUnavailable:
		w.Header().Set("Retry-After", "5")
		writeJSON(w, code, map[string]string{"error": "unavailable", "message": "service unavailable"})
	case http.StatusNotFound:
		writeNotFound(w, err)
	case http.StatusBadRequest:
		writeValidationErr(w, err)
	default:
		writeInternalZone(w, err)
	}
	return true
}

func (h *ZoneHandler) handleZones(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleCreate(w, r)
	case http.MethodGet:
		h.handleList(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}

func (h *ZoneHandler) handleZoneByID(w http.ResponseWriter, r *http.Request) {
	trim := strings.TrimPrefix(r.URL.Path, "/api/zones/")
	parts := strings.SplitN(trim, "/", 2)
	id := parts[0]
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.handleUpdate(w, r, id)
	case http.MethodDelete:
		h.handleDelete(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}

func (h *ZoneHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !h.allowWrite(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	name, coords, err := parseZoneRequest(r)
	if err != nil {
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
			return
		}
		writeValidationErr(w, err)
		return
	}
	created, err := h.svc.Create(r.Context(), name, coords)
	if err != nil {
		handleZoneError(w, err)
		return
	}
	resp := map[string]any{
		"id":   created.ID,
		"name": created.Name,
		"geometry": map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{created.Coordinates},
		},
		"geojson": map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{created.Coordinates},
		},
	}
	writeJSON(w, http.StatusCreated, resp)
}

type Feature struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties"`
	Geometry   map[string]any `json:"geometry"`
}

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

func (h *ZoneHandler) handleList(w http.ResponseWriter, r *http.Request) {
	zones, err := h.svc.List(r.Context())
	if err != nil {
		handleZoneError(w, err)
		return
	}
	if zones == nil {
		zones = []fleet.Zone{}
	}
	features := make([]Feature, 0, len(zones))
	for _, z := range zones {
		f := Feature{
			Type: "Feature",
			ID:   z.ID,
			Properties: map[string]any{
				"name": z.Name,
			},
			Geometry: map[string]any{
				"type":        "Polygon",
				"coordinates": [][][]float64{z.Coordinates},
			},
		}
		features = append(features, f)
	}
	resp := FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ZoneHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	if !h.allowWrite(w, r) {
		return
	}
	if err := validateZoneID(id); err != nil {
		writeValidationErr(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	name, coords, err := parseZoneRequest(r)
	if err != nil {
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
			return
		}
		writeValidationErr(w, err)
		return
	}
	updated, err := h.svc.Update(r.Context(), id, name, coords)
	if err != nil {
		handleZoneError(w, err)
		return
	}
	resp := map[string]any{
		"id":   updated.ID,
		"name": updated.Name,
		"geometry": map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{updated.Coordinates},
		},
		"geojson": map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{updated.Coordinates},
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ZoneHandler) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	if !h.allowWrite(w, r) {
		return
	}
	if err := validateZoneID(id); err != nil {
		writeValidationErr(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		handleZoneError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeValidationErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validation", "message": err.Error()})
}

func writeNotFound(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": err.Error()})
}

func writeInternalZone(w http.ResponseWriter, err error) {
	slog.Error("internal zone error", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
}
