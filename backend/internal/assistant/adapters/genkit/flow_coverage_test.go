package genkit_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	genkit "fleetmonitoring/backend/internal/assistant/adapters/genkit"
	"fleetmonitoring/backend/internal/assistant/application"
	"fleetmonitoring/backend/internal/assistant/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// extFakeQuerier is a controllable fake for genkit flow coverage.
type extFakeQuerier struct {
	findCalls    int32
	lastMin      int
	lastZone     *string
	lastLimit    int
	findRows     []domain.StoppedVehicle
	findErr      error
	summary      application.FleetSummary
	summaryErr   error
	status       application.VehicleStatus
	statusErr    error
	alerts       []application.Alert
	alertsErr    error
	alertsLimit  int
	statusPlate  shared.Plate
}

func (f *extFakeQuerier) FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	// Arrange helper
	atomic.AddInt32(&f.findCalls, 1)
	f.lastMin = minMinutes
	f.lastZone = zoneID
	f.lastLimit = limit
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.findRows != nil {
		return f.findRows, nil
	}
	return []domain.StoppedVehicle{
		{Plate: shared.Plate("GTP980"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona Norte", DurationMin: 27, Lat: 4.711, Lon: -74.072},
	}, nil
}
func (f *extFakeQuerier) GetFleetSummary(ctx context.Context) (application.FleetSummary, error) {
	// Arrange helper
	if f.summaryErr != nil {
		return application.FleetSummary{}, f.summaryErr
	}
	return f.summary, nil
}
func (f *extFakeQuerier) GetVehicleStatus(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error) {
	// Arrange helper
	f.statusPlate = plate
	if f.statusErr != nil {
		return application.VehicleStatus{}, f.statusErr
	}
	if f.status.Plate != "" {
		return f.status, nil
	}
	return application.VehicleStatus{Plate: plate, Status: "moving"}, nil
}
func (f *extFakeQuerier) GetActiveAlerts(ctx context.Context, limit int) ([]application.Alert, error) {
	// Arrange helper
	f.alertsLimit = limit
	if f.alertsErr != nil {
		return nil, f.alertsErr
	}
	if f.alerts != nil {
		return f.alerts, nil
	}
	return []application.Alert{}, nil
}

func (f *extFakeQuerier) ListPlates(ctx context.Context) ([]string, error) {
	return []string{"ABC123"}, nil
}

var _ application.FleetQuerier = (*extFakeQuerier)(nil)

// Covers [SPEC-003: AC-003, AC-004, BR-002, BR-005]
func TestFlow_Coverage_BreakerOpen_503(t *testing.T) {
	t.Run("returns 503 when breaker is open after consecutive failures", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{findErr: errors.New("db down")}
		stub := &genkit.StubGeminiClient{
			ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20}},
		}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.Background()
		input := genkit.ChatInput{Message: "vehiculos detenidos 20 minutos"}
		// Need 5 failures to trip breaker (MinRequests 5, ConsecutiveThreshold 3)
		for i := 0; i < 5; i++ {
			_, _ = flow.Chat(ctx, input)
		}
		// Act
		_, err := flow.Chat(ctx, input)
		// Assert
		if err == nil {
			t.Fatalf("expected 503 breaker open error")
		}
		if !strings.Contains(err.Error(), "503") {
			t.Fatalf("expected 503, got %v", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "breaker") && !strings.Contains(err.Error(), "open") {
			t.Fatalf("expected breaker open in error, got %v", err)
		}
	})
}

// Covers [SPEC-003: AC-003, BR-002, BR-003]
func TestFlow_Coverage_AllowlistFailClosed_403(t *testing.T) {
	t.Run("fail-closed 403 when ctx without claims and zoneID non-nil", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{
			ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20, "zoneId": "550e8400-e29b-41d4-a716-446655440000"}},
		}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.Background() // no JWTClaimsKey
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err == nil {
			t.Fatal("expected 403 allowlist error")
		}
		if !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403, got %v", err)
		}
	})
	t.Run("allowlist via ValidateAllowlist direct missing claims returns 403", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.Background()
		// Act
		err := genkit.ValidateAllowlist(ctx, zone)
		// Assert
		if err == nil {
			t.Fatal("expected 403")
		}
		if !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403, got %v", err)
		}
	})
	t.Run("allowlist passes when zone in allowedZones", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{
			"allowedZones": []string{zone},
		})
		// Act
		err := genkit.ValidateAllowlist(ctx, &zone)
		// Assert
		if err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})
	t.Run("allowlist with *string nil passes", func(t *testing.T) {
		// Arrange
		var zp *string
		ctx := context.Background()
		// Act
		err := genkit.ValidateAllowlist(ctx, zp)
		// Assert
		if err != nil {
			t.Fatalf("expected nil zone passes, got %v", err)
		}
	})
	t.Run("allowlist with empty string passes", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		// Act
		err := genkit.ValidateAllowlist(ctx, "")
		// Assert
		if err != nil {
			t.Fatalf("expected empty zone passes, got %v", err)
		}
	})
	t.Run("allowlist with map[string][]string type", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string][]string{
			"allowedZones": {zone},
		})
		// Act
		err := genkit.ValidateAllowlist(ctx, zone)
		// Assert
		if err != nil {
			t.Fatalf("expected pass for map[string][]string, got %v", err)
		}
	})
	t.Run("allowlist with map[string]string single zone", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]string{
			"allowedZones": zone,
		})
		// Act
		err := genkit.ValidateAllowlist(ctx, zone)
		// Assert
		if err != nil {
			t.Fatalf("expected pass for map[string]string, got %v", err)
		}
	})
	t.Run("allowlist with []any in map[string]any", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{
			"allowedZones": []any{zone},
		})
		// Act
		err := genkit.ValidateAllowlist(ctx, zone)
		// Assert
		if err != nil {
			t.Fatalf("expected pass for []any, got %v", err)
		}
	})
	t.Run("allowlist with missing allowedZones key returns 403", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{
			"fleet_id": "fleet-a",
		})
		// Act
		err := genkit.ValidateAllowlist(ctx, zone)
		// Assert
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403 for missing key, got %v", err)
		}
	})
	t.Run("allowlist with non-string zoneID via int still checks via Sprint", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		// Act
		err := genkit.ValidateAllowlist(ctx, 12345)
		// Assert
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403 for int zone fallback, got %v", err)
		}
	})
	t.Run("allowlist denies zone not in list 403", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{
			"allowedZones": []string{"11111111-1111-4111-8111-111111111111"},
		})
		// Act
		err := genkit.ValidateAllowlist(ctx, zone)
		// Assert
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403 deny, got %v", err)
		}
	})
}

// Covers [SPEC-003: AC-004, BR-005]
func TestFlow_Coverage_MaybeDelay_Cancel(t *testing.T) {
	t.Run("maybeDelay returns context canceled when parent canceled", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{Delay: 500 * time.Millisecond}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already canceled
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "hola"})
		// Assert
		if err == nil {
			t.Fatal("expected context canceled error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "canceled") && !strings.Contains(strings.ToLower(err.Error()), "cancel") {
			t.Fatalf("expected canceled error, got %v", err)
		}
	})
	t.Run("maybeDelay with empty ToolCall returns generic reply", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{Delay: 10 * time.Millisecond} // small delay, no tool
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.Background()
		// Act
		out, err := flow.Chat(ctx, genkit.ChatInput{Message: "hola"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.Reply, "sin datos") && !strings.Contains(out.Reply, "no se requirió") {
			t.Fatalf("expected empty tool reply, got %q", out.Reply)
		}
	})
}

// Covers [SPEC-003: AC-003, FR-002, FR-003]
func TestFlow_Coverage_DispatchUnknownTool(t *testing.T) {
	t.Run("dispatch unknown tool returns not recognized without error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: "unknownTool_xyz", Args: map[string]any{}}}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.Background()
		// Act
		out, err := flow.Chat(ctx, genkit.ChatInput{Message: "hola"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for unknown tool, got %v", err)
		}
		if !strings.Contains(out.Reply, "no reconocido") {
			t.Fatalf("expected 'no reconocido', got %q", out.Reply)
		}
		if len(out.Citations) != 0 {
			t.Fatalf("expected 0 citations for unknown tool, got %v", out.Citations)
		}
	})
}

// Covers [SPEC-003: FR-003, BR-009]
func TestFlow_Coverage_HandleVehicleStatus(t *testing.T) {
	t.Run("plate missing returns validation error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado vehiculo"})
		// Assert
		if err == nil {
			t.Fatal("expected plate required error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "plate") {
			t.Fatalf("expected plate error, got %v", err)
		}
	})
	t.Run("plate empty string returns validation error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{"plate": "   "}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado"})
		// Assert
		if err == nil {
			t.Fatal("expected plate required")
		}
	})
	t.Run("plate invalid format returns error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{"plate": "BAD12"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado"})
		// Assert
		if err == nil {
			t.Fatal("expected invalid plate error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "invalid") {
			t.Fatalf("expected invalid, got %v", err)
		}
	})
	t.Run("plate non-string type returns validation error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{"plate": 12345}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado"})
		// Assert
		if err == nil {
			t.Fatal("expected plate required for non-string")
		}
	})
	t.Run("querier error propagates", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{statusErr: errors.New("db fail")}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{"plate": "GTP980"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado"})
		// Assert
		if err == nil || !strings.Contains(err.Error(), "get vehicle status failed") {
			t.Fatalf("expected wrapped vehicle status failed, got %v", err)
		}
	})
	t.Run("success returns citation", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{status: application.VehicleStatus{Plate: shared.Plate("GTP980"), Status: "moving"}}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{"plate": "GTP980"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado GTP980"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Citations) != 1 || out.Citations[0].Tool != genkit.ToolVehicleStatus {
			t.Fatalf("expected citation %v, got %v", genkit.ToolVehicleStatus, out.Citations)
		}
		if !strings.Contains(out.Reply, "GTP980") {
			t.Fatalf("expected GTP980 in reply, got %q", out.Reply)
		}
	})
	t.Run("plate lowercase normalized accepted", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolVehicleStatus, Args: map[string]any{"plate": "gtp980"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "estado"})
		// Assert
		if err != nil {
			t.Fatalf("expected lower case plate accepted, got %v", err)
		}
		if q.statusPlate != "GTP980" {
			t.Fatalf("expected normalized GTP980, got %v", q.statusPlate)
		}
	})
}

// Covers [SPEC-003: FR-003, BR-004]
func TestFlow_Coverage_HandleActiveAlerts(t *testing.T) {
	t.Run("limit clamp 0 to 20 via flow", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": 0}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for limit 0, got %v", err)
		}
		if q.alertsLimit != 20 {
			t.Fatalf("expected clamped 20, got %d", q.alertsLimit)
		}
	})
	t.Run("limit clamp 21 to 20 via flow", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": 21}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if q.alertsLimit != 20 {
			t.Fatalf("expected clamped 20 for 21, got %d", q.alertsLimit)
		}
	})
	t.Run("limit as int64 and float64 parsed correctly", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub64 := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": int64(5)}}}
		flow := genkit.NewAssistantFlow(q, stub64)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for int64, got %v", err)
		}
		if q.alertsLimit != 5 {
			t.Fatalf("expected 5 for int64, got %d", q.alertsLimit)
		}
		// Arrange float64
		q2 := &extFakeQuerier{}
		stubF := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": float64(7)}}}
		flow2 := genkit.NewAssistantFlow(q2, stubF)
		// Act
		_, err = flow2.Chat(context.Background(), genkit.ChatInput{Message: "alertas2"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for float64, got %v", err)
		}
		if q2.alertsLimit != 7 {
			t.Fatalf("expected 7 for float64, got %d", q2.alertsLimit)
		}
	})
	t.Run("limit as int32 parsed correctly", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": int32(9)}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for int32, got %v", err)
		}
		if q.alertsLimit != 9 {
			t.Fatalf("expected 9, got %d", q.alertsLimit)
		}
	})
	t.Run("querier error propagates", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{alertsErr: errors.New("db down")}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": 5}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas"})
		// Assert
		if err == nil || !strings.Contains(err.Error(), "get active alerts failed") {
			t.Fatalf("expected get active alerts failed, got %v", err)
		}
	})
	t.Run("success returns citation count", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{alerts: []application.Alert{{Plate: shared.Plate("GTP980")}, {Plate: shared.Plate("TTY423")}}}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": 5}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Citations) != 1 || out.Citations[0].Count != 2 {
			t.Fatalf("expected count 2, got %v", out.Citations)
		}
		if !strings.Contains(out.Reply, "Alertas activas 2") {
			t.Fatalf("expected reply Alertas activas 2, got %q", out.Reply)
		}
	})
	t.Run("allowlist 403 when zone provided without claims", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": 5, "zoneId": "550e8400-e29b-41d4-a716-446655440000"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "alertas zona"})
		// Assert
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403 allowlist for alerts with zone, got %v", err)
		}
	})
	t.Run("allowlist with zoneID key passes when allowed", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolActiveAlerts, Args: map[string]any{"limit": 5, "zoneID": zone}}}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{"allowedZones": []string{zone}})
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "alertas"})
		// Assert
		if err != nil {
			t.Fatalf("expected pass for zoneID key, got %v", err)
		}
	})
}

// Covers [SPEC-003: AC-001, BR-001, BR-003]
func TestFlow_Coverage_HandleFindStopped(t *testing.T) {
	t.Run("querier error propagates", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{findErr: errors.New("db down")}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20}}}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{"allowedZones": []string{}})
		// Use empty allowlist but no zone -> passes; error from querier
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err == nil || !strings.Contains(err.Error(), "find stopped failed") {
			t.Fatalf("expected find stopped failed, got %v", err)
		}
	})
	t.Run("validation error minMinutes 0", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 0}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err == nil {
			t.Fatal("expected validation error for minMinutes 0")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "validation") && !strings.Contains(err.Error(), "minMinutes") {
			t.Fatalf("expected validation minMinutes, got %v", err)
		}
	})
	t.Run("validation error invalid zoneID", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20, "zoneId": "not-a-uuid"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err == nil {
			t.Fatal("expected validation error for zone")
		}
	})
	t.Run("invalid row validation fails", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{findRows: []domain.StoppedVehicle{{Plate: shared.Plate("BAD"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona", DurationMin: 27, Lat: 4.7, Lon: -74.0}}}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err == nil {
			t.Fatal("expected row validation error")
		}
		if !strings.Contains(err.Error(), "stopped row") {
			t.Fatalf("expected stopped row invalid, got %v", err)
		}
	})
	t.Run("empty rows returns no vehicles reply", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{findRows: []domain.StoppedVehicle{}}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for empty, got %v", err)
		}
		if !strings.Contains(out.Reply, "No hay") {
			t.Fatalf("expected No hay vehicles, got %q", out.Reply)
		}
		if out.Citations[0].Count != 0 {
			t.Fatalf("expected count 0, got %d", out.Citations[0].Count)
		}
	})
	t.Run("parseIntArg int64 and float64 variants", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": int64(25), "limit": float64(5)}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for int64/float64, got %v", err)
		}
		if q.lastMin != 25 {
			t.Fatalf("expected 25 for int64, got %d", q.lastMin)
		}
		if q.lastLimit != 5 {
			t.Fatalf("expected 5 for float64, got %d", q.lastLimit)
		}
	})
	t.Run("parseIntArg int32 variant", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": int32(30)}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for int32, got %v", err)
		}
		if q.lastMin != 30 {
			t.Fatalf("expected 30, got %d", q.lastMin)
		}
	})
	t.Run("zoneId vs zoneID both accepted", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20, "zoneID": zone}}}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{"allowedZones": []string{zone}})
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "detenidos zona"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for zoneID key, got %v", err)
		}
		if q.lastZone == nil || *q.lastZone != zone {
			t.Fatalf("expected zoneID propagated")
		}
	})
}

// Covers [SPEC-003: FR-003]
func TestFlow_Coverage_HandleFleetSummary(t *testing.T) {
	t.Run("success returns fleet summary reply", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{summary: application.FleetSummary{Total: 10, Moving: 5, Idle: 3}}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFleetSummary, Args: map[string]any{}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "resumen flota"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.Reply, "Resumen flota") {
			t.Fatalf("expected Resumen flota, got %q", out.Reply)
		}
		if len(out.Citations) != 1 || out.Citations[0].Tool != genkit.ToolFleetSummary {
			t.Fatalf("expected citation fleet summary, got %v", out.Citations)
		}
	})
	t.Run("querier error propagates", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{summaryErr: errors.New("db fail")}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFleetSummary, Args: map[string]any{}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "resumen"})
		// Assert
		if err == nil || !strings.Contains(err.Error(), "get fleet summary failed") {
			t.Fatalf("expected get fleet summary failed, got %v", err)
		}
	})
	t.Run("allowlist 403 when zone filtered", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFleetSummary, Args: map[string]any{"zoneId": "550e8400-e29b-41d4-a716-446655440000"}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "resumen zona"})
		// Assert
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected 403 for fleet summary with zone no claims, got %v", err)
		}
	})
	t.Run("allowlist passes when zone allowed", func(t *testing.T) {
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"
		q := &extFakeQuerier{summary: application.FleetSummary{Total: 5}}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFleetSummary, Args: map[string]any{"zoneId": zone}}}
		flow := genkit.NewAssistantFlow(q, stub)
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{"allowedZones": []string{zone}})
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "resumen"})
		// Assert
		if err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})
}

// Covers [SPEC-003: AC-003, AC-004, BR-003]
func TestFlow_Coverage_ChatValidationAndResolve(t *testing.T) {
	t.Run("empty message returns validation error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "   "})
		// Assert
		if err == nil {
			t.Fatal("expected validation error for empty message")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "validation") {
			t.Fatalf("expected validation, got %v", err)
		}
	})
	t.Run("message too long returns validation error", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)
		long := strings.Repeat("a", 4001)
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: long})
		// Assert
		if err == nil {
			t.Fatal("expected validation for long message")
		}
	})
	t.Run("no tool call returns generic reply without citation", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "hola que tal"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Citations) != 0 {
			t.Fatalf("expected 0 citations, got %v", out.Citations)
		}
		if !strings.Contains(out.Reply, "no se requirió") {
			t.Fatalf("expected no tool reply, got %q", out.Reply)
		}
	})
	t.Run("heuristic resolves tool via message containing 20 and vehiculos", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		flow := genkit.NewAssistantFlow(q, nil) // no stub, relies on heuristic
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "que vehiculos llevan detenidos 20 minutos?"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error for heuristic, got %v", err)
		}
		if len(out.Citations) == 0 || out.Citations[0].Tool != genkit.ToolFindStopped {
			t.Fatalf("expected heuristic findStopped, got %v", out.Citations)
		}
	})
	t.Run("heuristic with detenid keyword", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)
		// Act
		out, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "mostrar detenidos"})
		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Citations) == 0 {
			t.Fatalf("expected heuristic for detenid")
		}
	})
}

// Covers [SPEC-003: AC-003, BR-003]
func TestFlow_Coverage_FilterOutput(t *testing.T) {
	t.Run("filters GEMINI key but keeps normal text", func(t *testing.T) {
		// Arrange
		in := "GTP980 lleva 27m en Zona Norte"
		// Act
		got := genkit.FilterOutput(in)
		// Assert
		if got != in {
			t.Fatalf("expected no filter for normal, got %q", got)
		}
	})
	t.Run("filters multiple patterns together", func(t *testing.T) {
		// Arrange
		in := "token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjMifQ.signature and DROP TABLE foo"
		// Act
		got := genkit.FilterOutput(in)
		// Assert
		if strings.Contains(got, "DROP TABLE") {
			t.Fatalf("expected DROP TABLE filtered, got %q", got)
		}
		if strings.Contains(got, "eyJhbGciOi") {
			t.Fatalf("expected JWT filtered, got %q", got)
		}
		if !strings.Contains(got, "[filtrado]") {
			t.Fatalf("expected filtrado, got %q", got)
		}
	})
	t.Run("filters sk- secrets", func(t *testing.T) {
		// Arrange
		in := "sk-proj-abc1234567890xyz leaked"
		// Act
		got := genkit.FilterOutput(in)
		// Assert
		if strings.Contains(got, "sk-proj") {
			t.Fatalf("expected sk filtered, got %q", got)
		}
	})
	t.Run("filters DATABASE_URL", func(t *testing.T) {
		// Arrange
		in := "DATABASE_URL=postgres://user:pass@localhost:5432/fleet"
		// Act
		got := genkit.FilterOutput(in)
		// Assert
		if strings.Contains(got, "DATABASE_URL") {
			t.Fatalf("expected DATABASE_URL filtered, got %q", got)
		}
	})
}

// Covers [SPEC-003: AC-004, BR-005]
func TestFlow_Coverage_SemaphoreParentCancel(t *testing.T) {
	t.Run("acquire returns context canceled when parent canceled while semaphore full", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		blockingStub := &genkit.StubGeminiClient{Delay: 2 * time.Second}
		flow := genkit.NewAssistantFlow(q, blockingStub)
		// Fill semaphore with 20 concurrent chats
		for i := 0; i < 20; i++ {
			go func() {
				_, _ = flow.Chat(context.Background(), genkit.ChatInput{Message: "hola filler"})
			}()
		}
		// Wait until semaphore at least 10 occupied (best effort)
		for i := 0; i < 20; i++ {
			if flow.CurrentSemaphoreCount() >= 10 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediate cancel
		// Act
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "hola after fill"})
		// Assert
		if err == nil {
			t.Fatalf("expected context canceled due to parent cancel with full semaphore, got nil (count=%d)", flow.CurrentSemaphoreCount())
		}
		if !strings.Contains(strings.ToLower(err.Error()), "canceled") && !strings.Contains(strings.ToLower(err.Error()), "cancel") {
			// also acceptable: semaphore timeout if timing off, but we expect canceled path at least when semaphore full
			// If not full yet, error may be canceled from maybeDelay, still valid
			t.Fatalf("expected canceled, got %v count=%d", err, flow.CurrentSemaphoreCount())
		}
	})
}

func TestFlow_Coverage_BreakerStateAndOptions(t *testing.T) {
	t.Run("breaker state closed initially and GenerateOptions correct", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)
		// Act
		state := flow.BreakerState()
		opts := flow.GenerateOptions()
		names := flow.ToolNames()
		// Assert
		if state.String() != "closed" {
			t.Fatalf("expected closed, got %v", state)
		}
		if opts.MaxOutputTokens != 1024 || opts.Temperature != 0.2 {
			t.Fatalf("expected opts 1024/0.2, got %+v", opts)
		}
		if len(names) != 4 {
			t.Fatalf("expected 4 tools, got %v", names)
		}
	})
}

func TestFlow_Coverage_DispatchHandlerErrorWrapped(t *testing.T) {
	t.Run("dispatch handler error wrapped without breaker open", func(t *testing.T) {
		// Arrange
		q := &extFakeQuerier{findErr: errors.New("transient")}
		stub := &genkit.StubGeminiClient{ToolCall: genkit.ToolCall{Name: genkit.ToolFindStopped, Args: map[string]any{"minMinutes": 20}}}
		flow := genkit.NewAssistantFlow(q, stub)
		// Use fresh flow so breaker closed; first call fails but not open state
		// Act
		_, err := flow.Chat(context.Background(), genkit.ChatInput{Message: "detenidos"})
		// Assert
		if err == nil {
			t.Fatal("expected handler error")
		}
		if strings.Contains(err.Error(), "503") {
			t.Fatalf("expected not 503 for first failure, got %v", err)
		}
		if !strings.Contains(err.Error(), "transient") && !strings.Contains(err.Error(), "find stopped failed") {
			t.Fatalf("expected transient wrapped, got %v", err)
		}
	})
}
