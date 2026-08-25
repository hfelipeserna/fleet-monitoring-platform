package http

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fleetmonitoring/backend/internal/telemetry/application"
)

func decodeSingleEvent(m map[string]json.RawMessage) (application.RawEvent, error) {
	plate, speedInt, err := decodeRequiredFields(m)
	if err != nil {
		return application.RawEvent{}, err
	}
	latPtr, err := parseOptionalFloat(m, "lat")
	if err != nil {
		return application.RawEvent{}, err
	}
	lonPtr, err := parseOptionalFloat(m, "lon")
	if err != nil {
		return application.RawEvent{}, err
	}
	cid, err := parseOptionalString(m, "client_event_id")
	if err != nil {
		return application.RawEvent{}, err
	}
	occ, err := parseOccurredAt(m)
	if err != nil {
		return application.RawEvent{}, err
	}
	return application.RawEvent{
		Plate:         plate,
		Speed:         &speedInt,
		Lat:           latPtr,
		Lon:           lonPtr,
		ClientEventID: cid,
		OccurredAt:    occ,
	}, nil
}

func decodeRequiredFields(m map[string]json.RawMessage) (string, int, error) {
	plateRaw, err := getRequiredRaw(m, "plate")
	if err != nil {
		return "", 0, err
	}
	var plate string
	if err := json.Unmarshal(plateRaw, &plate); err != nil {
		return "", 0, fmt.Errorf("invalid plate: %w", err)
	}
	speedRaw, err := getRequiredRaw(m, "speed")
	if err != nil {
		return "", 0, err
	}
	speedInt, err := parseSpeedInt(speedRaw)
	if err != nil {
		return "", 0, err
	}
	return plate, speedInt, nil
}

func getRequiredRaw(m map[string]json.RawMessage, key string) (json.RawMessage, error) {
	raw, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("missing %s: %w", key, application.ErrValidation)
	}
	return raw, nil
}

func parseOptionalFloat(m map[string]json.RawMessage, key string) (*float64, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	return &f, nil
}

func parseOptionalString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", nil
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "null" || trim == "" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("invalid %s: %w", key, err)
	}
	return s, nil
}

func parseOccurredAt(m map[string]json.RawMessage) (*time.Time, error) {
	raw, ok := m["occurred_at"]
	if !ok {
		return nil, nil
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "null" || trim == "" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("invalid occurred_at: %w", err)
	}
	tm, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		tm2, err2 := time.Parse(time.RFC3339, s)
		if err2 != nil {
			return nil, fmt.Errorf("invalid occurred_at format: %w", err)
		}
		tm = tm2
	}
	return &tm, nil
}

func decodeBatch(body []byte) ([]application.RawEvent, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	eventsRaw, ok := top["events"]
	if !ok {
		return nil, fmt.Errorf("missing events: %w", application.ErrValidation)
	}
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(eventsRaw, &rawMessages); err != nil {
		return nil, fmt.Errorf("invalid events: %w", err)
	}
	if len(rawMessages) == 0 || len(rawMessages) > application.MaxBatchSize {
		return nil, fmt.Errorf("invalid batch size %d: %w", len(rawMessages), application.ErrValidation)
	}
	raws := make([]application.RawEvent, 0, len(rawMessages))
	for i, eb := range rawMessages {
		var mm map[string]json.RawMessage
		if err := json.Unmarshal(eb, &mm); err != nil {
			return nil, fmt.Errorf("invalid event %d: %w", i, err)
		}
		raw, err := decodeSingleEvent(mm)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", i, err)
		}
		raws = append(raws, raw)
	}
	return raws, nil
}

func parseSpeedInt(raw json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, fmt.Errorf("missing speed")
	}
	if strings.ContainsAny(s, ".eE") {
		return 0, fmt.Errorf("speed must be integer")
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("speed must be integer: %w", err)
	}
	return v, nil
}
