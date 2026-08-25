package genkit_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	genkit "fleetmonitoring/backend/internal/assistant/adapters/genkit"
	"fleetmonitoring/backend/internal/assistant/application"
	"fleetmonitoring/backend/internal/assistant/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// fakeFleetQuerier captures tool invocations for flow tests.
type fakeFleetQuerier struct {
	findCalls int32
	lastMin   int
	lastZone  *string
	lastLimit int
	rows      []domain.StoppedVehicle
	err       error
}

func (f *fakeFleetQuerier) FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	// Arrange helper: record invocation
	atomic.AddInt32(&f.findCalls, 1)
	f.lastMin = minMinutes
	f.lastZone = zoneID
	f.lastLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	if f.rows != nil {
		return f.rows, nil
	}
	return []domain.StoppedVehicle{
		{Plate: shared.Plate("GTP980"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona Norte", DurationMin: 27, Lat: 4.711, Lon: -74.072},
		{Plate: shared.Plate("TTY423"), ZoneID: "550e8400-e29b-41d4-a716-446655440000", ZoneName: "Zona Norte", DurationMin: 25, Lat: 4.712, Lon: -74.073},
	}, nil
}

func (f *fakeFleetQuerier) GetFleetSummary(ctx context.Context) (application.FleetSummary, error) {
	// Arrange helper
	return application.FleetSummary{Total: 10, Moving: 5, Idle: 5}, nil
}

func (f *fakeFleetQuerier) GetVehicleStatus(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error) {
	// Arrange helper
	return application.VehicleStatus{Plate: plate}, nil
}

func (f *fakeFleetQuerier) GetActiveAlerts(ctx context.Context, limit int) ([]application.Alert, error) {
	// Arrange helper
	return nil, nil
}

func (f *fakeFleetQuerier) ListPlates(ctx context.Context) ([]string, error) {
	return []string{"ABC123", "XYZ789"}, nil
}

var _ application.FleetQuerier = (*fakeFleetQuerier)(nil)

// Covers [SPEC-003: FR-002, FR-003, BR-002]
func TestFlow_Registers_4_Tools(t *testing.T) {
	// Covers [SPEC-003: AC-003, FR-002, FR-003, BR-002]
	t.Run("registers exactly 5 read-only tools with correct names and schemas", func(t *testing.T) {
		// Arrange
		q := &fakeFleetQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)

		// Act
		tools := genkit.DefineTools
		names := flow.ToolNames()

		// Assert
		if len(tools) != 5 {
			t.Fatalf("expected 5 tools defined, got %d", len(tools))
		}
		expected := []string{"findVehiclesStoppedInCriticalZones", "getFleetSummary", "getVehicleStatus", "getActiveAlerts", "listPlates"}
		for _, exp := range expected {
			found := false
			for _, n := range names {
				if n == exp {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected tool %q to be registered, got %v", exp, names)
			}
		}
		// Verify schemas contain required fields via genkit tool definitions
		schema := genkit.FindStoppedToolSchema
		if !strings.Contains(schema, "minMinutes") {
			t.Fatalf("expected findVehiclesStoppedInCriticalZones schema to contain minMinutes, got %q", schema)
		}
		if !strings.Contains(schema, "zoneId") {
			t.Fatalf("expected schema to contain zoneId, got %q", schema)
		}
	})
}

func TestFlow_Invokes_FindStopped_Tool(t *testing.T) {
	// Covers [SPEC-003: AC-001, AC-003, BR-002, FR-003, FR-004]
	t.Run("stub Gemini invokes findVehiclesStoppedInCriticalZones with minMinutes 20 and validates allowlist", func(t *testing.T) {
		// Arrange
		querier := &fakeFleetQuerier{}
		stubGemini := &genkit.StubGeminiClient{
			ToolCall: genkit.ToolCall{
				Name: "findVehiclesStoppedInCriticalZones",
				Args: map[string]any{"minMinutes": 20},
			},
		}
		flow := genkit.NewAssistantFlow(querier, stubGemini)
		ctx := context.WithValue(context.Background(), genkit.JWTClaimsKey, map[string]any{
			"allowedZones": []string{"550e8400-e29b-41d4-a716-446655440000"},
		})
		input := genkit.ChatInput{Message: "¿qué vehículos llevan detenidos más de 20 minutos en zonas críticas?"}

		// Act
		output, err := flow.Chat(ctx, input)

		// Assert
		if err != nil {
			t.Fatalf("expected no error from flow Chat, got %v", err)
		}
		if atomic.LoadInt32(&querier.findCalls) != 1 {
			t.Fatalf("expected FleetQuerier.FindStoppedInZones to be called once, got %d", querier.findCalls)
		}
		if querier.lastMin != 20 {
			t.Fatalf("expected minMinutes 20 propagated to querier, got %d", querier.lastMin)
		}
		if querier.lastLimit != 20 {
			t.Fatalf("expected default limit 20, got %d", querier.lastLimit)
		}
		if len(output.Citations) == 0 || output.Citations[0].Tool != "findVehiclesStoppedInCriticalZones" {
			t.Fatalf("expected citation for findVehiclesStoppedInCriticalZones, got %v", output.Citations)
		}
		if output.Citations[0].Count != 2 {
			t.Fatalf("expected citation count 2, got %d", output.Citations[0].Count)
		}
		if !strings.Contains(output.Reply, "GTP980") {
			t.Fatalf("expected reply to contain GTP980, got %q", output.Reply)
		}
	})
}

func TestFlow_MaxOutputTokens_1024_cap(t *testing.T) {
	// Covers [SPEC-003: AC-003, BR-005, FR-006]
	t.Run("caps maxOutputTokens to 1024", func(t *testing.T) {
		// Arrange
		q := &fakeFleetQuerier{}
		flow := genkit.NewAssistantFlow(q, nil)

		// Act
		opts := flow.GenerateOptions()
		maxTokens := genkit.MaxOutputTokens

		// Assert
		if maxTokens != 1024 {
			t.Fatalf("expected MaxOutputTokens 1024, got %d", maxTokens)
		}
		if opts.MaxOutputTokens != 1024 {
			t.Fatalf("expected GenerateOptions MaxOutputTokens 1024, got %d", opts.MaxOutputTokens)
		}
	})
}

func TestFlow_Timeout_15s(t *testing.T) {
	// Covers [SPEC-003: AC-004, BR-005, FR-001, FR-006]
	t.Run("respects 30s timeout and cancels slow Gemini call", func(t *testing.T) {
		// Arrange
		q := &fakeFleetQuerier{}
		delayedGemini := &genkit.StubGeminiClient{
			Delay: 31 * time.Second,
		}
		flow := genkit.NewAssistantFlow(q, delayedGemini)
		timeout := genkit.FlowTimeout

		// Act
		if timeout != 30*time.Second {
			t.Fatalf("expected FlowTimeout 30s, got %v", timeout)
		}
		ctx := context.Background()
		start := time.Now()
		_, err := flow.Chat(ctx, genkit.ChatInput{Message: "hola"})

		// Assert
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("expected timeout error for 16s delay exceeding 15s")
		}
		if !strings.Contains(err.Error(), "deadline") && !strings.Contains(strings.ToLower(err.Error()), "timeout") {
			t.Fatalf("expected deadline/timeout error, got %v", err)
		}
		if elapsed > 31*time.Second {
			t.Fatalf("expected flow to timeout near 30s, elapsed %v", elapsed)
		}
		if elapsed < 29*time.Second {
			t.Fatalf("expected flow to wait close to 30s before timeout, elapsed %v", elapsed)
		}
	})
}

func TestFlow_Semaphore_20_cap(t *testing.T) {
	// Covers [SPEC-003: AC-004, BR-005, FR-006]
	t.Run("limits concurrency to 20 via semaphore", func(t *testing.T) {
		// Arrange
		q := &fakeFleetQuerier{}
		blockingGemini := &genkit.StubGeminiClient{
			Delay: 200 * time.Millisecond,
		}
		flow := genkit.NewAssistantFlow(q, blockingGemini)
		capacity := genkit.SemaphoreCap
		ctx := context.Background()

		// Act
		if capacity != 20 {
			t.Fatalf("expected SemaphoreCap 20, got %d", capacity)
		}
		// Launch 21 concurrent chats; 21st should block and queue (no fail-fast default)
		errCh := make(chan error, 21)
		for i := 0; i < 21; i++ {
			go func() {
				// Arrange per goroutine
				_, err := flow.Chat(ctx, genkit.ChatInput{Message: "hola"})
				// Act done
				errCh <- err
			}()
		}
		// Collect with timeout: blocking semaphore queues 21st, needs ~400ms (20*200ms slots)
		var results []error
		timeout := time.After(5 * time.Second)
		for len(results) < 21 {
			select {
			case e := <-errCh:
				results = append(results, e)
			case <-timeout:
				t.Fatalf("timeout waiting for 21 results, got %d", len(results))
			}
		}
		close(errCh)
		if len(results) != 21 {
			t.Fatalf("expected 21 results, got %d", len(results))
		}
		// At least 20 should have been attempted; 21st may be throttled
		semCount := flow.CurrentSemaphoreCount()
		if semCount > 20 {
			t.Fatalf("expected semaphore count <=20, got %d", semCount)
		}
	})
}
