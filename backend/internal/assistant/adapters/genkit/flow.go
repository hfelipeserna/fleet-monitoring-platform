package genkit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/sony/gobreaker"

	"fleetmonitoring/backend/internal/assistant/application"
	"fleetmonitoring/backend/internal/assistant/domain"
	"fleetmonitoring/backend/internal/assistant/infra/breaker"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

const (
	FlowTimeout     = 30 * time.Second
	MaxOutputTokens = 1024
	SemaphoreCap    = 20
)

type ToolCall struct {
	Name string
	Args map[string]any
}

type StubGeminiClient struct {
	ToolCall ToolCall
	Delay    time.Duration
}

type ChatInput struct {
	Message string
}

type Citation struct {
	Tool  string
	Count int
}

type ChatOutput struct {
	Reply     string
	Citations []Citation
}

type GenerateOptions struct {
	MaxOutputTokens int
	Temperature     float64
}

type AssistantFlow struct {
	querier  application.FleetQuerier
	gemini   *StubGeminiClient
	sem      chan struct{}
	breaker  *gobreaker.CircuitBreaker
	semCount atomic.Int32
	registry map[string]ToolHandler
	g        *genkit.Genkit
	model    string
	tools    []ai.ToolRef
	toolsMu  sync.Mutex
}

func NewAssistantFlow(q application.FleetQuerier, client *StubGeminiClient) *AssistantFlow {
	cb := gobreaker.NewCircuitBreaker(breaker.NewSettings(breaker.DefaultTimeout))
	f := &AssistantFlow{
		querier: q,
		gemini:  client,
		sem:     make(chan struct{}, SemaphoreCap),
		breaker: cb,
	}
	f.initRegistry()
	return f
}

func (f *AssistantFlow) initRegistry() {
	f.registry = map[string]ToolHandler{
		ToolFindStopped:   f.handleFindStopped,
		ToolFleetSummary:  f.handleFleetSummary,
		ToolVehicleStatus: f.handleVehicleStatus,
		ToolActiveAlerts:  f.handleActiveAlerts,
		ToolListPlates:    f.handleListPlates,
	}
}

func (f *AssistantFlow) ToolNames() []string {
	names := make([]string, len(DefineTools))
	for i, t := range DefineTools {
		names[i] = t.Name
	}
	return names
}

func (f *AssistantFlow) GenerateOptions() GenerateOptions {
	return GenerateOptions{MaxOutputTokens: MaxOutputTokens, Temperature: 0.2}
}

func (f *AssistantFlow) CurrentSemaphoreCount() int {
	return int(f.semCount.Load())
}

func (f *AssistantFlow) BreakerState() gobreaker.State {
	if f.breaker == nil {
		return gobreaker.StateClosed
	}
	return f.breaker.State()
}

func (f *AssistantFlow) acquire(timeoutCtx, parentCtx context.Context) error {
	select {
	case f.sem <- struct{}{}:
		f.semCount.Add(1)
		return nil
	case <-timeoutCtx.Done():
		return fmt.Errorf("semaphore timeout: %w", timeoutCtx.Err())
	case <-parentCtx.Done():
		return fmt.Errorf("context canceled: %w", parentCtx.Err())
	}
}

func (f *AssistantFlow) release() {
	<-f.sem
	f.semCount.Add(-1)
}

func (f *AssistantFlow) maybeDelay(timeoutCtx, parentCtx context.Context) (bool, error) {
	if f.gemini == nil || f.gemini.Delay == 0 {
		return false, nil
	}
	select {
	case <-time.After(f.gemini.Delay):
	case <-timeoutCtx.Done():
		return false, fmt.Errorf("deadline exceeded: %w", timeoutCtx.Err())
	case <-parentCtx.Done():
		return false, fmt.Errorf("context canceled: %w", parentCtx.Err())
	}
	if f.gemini.ToolCall.Name == "" {
		return true, nil
	}
	return false, nil
}

func (f *AssistantFlow) resolveToolCall(input ChatInput) *ToolCall {
	if f.gemini != nil && f.gemini.ToolCall.Name != "" {
		return &f.gemini.ToolCall
	}
	lower := strings.ToLower(input.Message)
	if strings.Contains(lower, "20") || strings.Contains(lower, "vehículos") || strings.Contains(lower, "vehiculos") || strings.Contains(lower, "detenid") {
		return &ToolCall{Name: ToolFindStopped, Args: map[string]any{"minMinutes": 20}}
	}
	return nil
}

func parseIntArg(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		switch iv := v.(type) {
		case int:
			return iv
		case int32:
			return int(iv)
		case int64:
			return int(iv)
		case float64:
			return int(iv)
		}
	}
	return def
}

func extractZoneID(args map[string]any) *string {
	if v, ok := args["zoneId"]; ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return &s
		}
	}
	if v, ok := args["zoneID"]; ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return &s
		}
	}
	return nil
}

func (f *AssistantFlow) parseFindStoppedArgs(args map[string]any) (int, *string, int, error) {
	minMinutes := parseIntArg(args, "minMinutes", shared.StoppedMinMinutesDefault)
	zoneID := extractZoneID(args)
	limit := parseIntArg(args, "limit", shared.StoppedLimitDefault)
	clamped, err := shared.ValidateStoppedParams(minMinutes, limit, zoneID)
	if err != nil {
		return 0, nil, 0, err
	}
	return minMinutes, zoneID, clamped, nil
}

func (f *AssistantFlow) handleFindStopped(ctx context.Context, args map[string]any) (string, Citation, error) {
	minMinutes, zoneID, limit, err := f.parseFindStoppedArgs(args)
	if err != nil {
		return "", Citation{}, fmt.Errorf("validation: %w", err)
	}
	if err := ValidateAllowlist(ctx, zoneID); err != nil {
		return "", Citation{}, err
	}
	rows, err := f.querier.FindStoppedInZones(ctx, minMinutes, zoneID, limit)
	if err != nil {
		return "", Citation{}, fmt.Errorf("find stopped failed: %w", err)
	}
	for i := range rows {
		if err := rows[i].Validate(); err != nil {
			return "", Citation{}, fmt.Errorf("stopped row %d invalid: %w", i, err)
		}
		rows[i] = rows[i].Normalized()
	}
	reply := FilterOutput(buildStoppedReply(rows))
	return reply, Citation{Tool: ToolFindStopped, Count: len(rows)}, nil
}

func (f *AssistantFlow) handleFleetSummary(ctx context.Context, args map[string]any) (string, Citation, error) {
	if zoneID := extractZoneID(args); zoneID != nil {
		if err := ValidateAllowlist(ctx, *zoneID); err != nil {
			return "", Citation{}, err
		}
	}
	s, err := f.querier.GetFleetSummary(ctx)
	if err != nil {
		return "", Citation{}, fmt.Errorf("get fleet summary failed: %w", err)
	}
	reply := FilterOutput(fmt.Sprintf("Resumen flota total %d en movimiento %d detenidos %d", s.Total, s.Moving, s.Idle))
	return reply, Citation{Tool: ToolFleetSummary, Count: 1}, nil
}

func (f *AssistantFlow) handleVehicleStatus(ctx context.Context, args map[string]any) (string, Citation, error) {
	raw, ok := args["plate"]
	if !ok {
		return "", Citation{}, fmt.Errorf("validation: plate required: %w", shared.ErrValidation)
	}
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", Citation{}, fmt.Errorf("validation: plate required: %w", shared.ErrValidation)
	}
	plate, err := shared.ParsePlate(s)
	if err != nil {
		return "", Citation{}, fmt.Errorf("plate %q invalid: %w", s, err)
	}
	st, err := f.querier.GetVehicleStatus(ctx, plate)
	if err != nil {
		return "", Citation{}, fmt.Errorf("get vehicle status failed: %w", err)
	}
	reply := FilterOutput(fmt.Sprintf("Vehículo %s estado %s", st.Plate, st.Status))
	return reply, Citation{Tool: ToolVehicleStatus, Count: 1}, nil
}

func (f *AssistantFlow) handleActiveAlerts(ctx context.Context, args map[string]any) (string, Citation, error) {
	limit := parseIntArg(args, "limit", shared.StoppedLimitDefault)
	clamped, err := shared.ValidateStoppedParams(shared.StoppedMinMinutesDefault, limit, nil)
	if err != nil {
		return "", Citation{}, fmt.Errorf("validation: %w", err)
	}
	limit = clamped
	if zoneID := extractZoneID(args); zoneID != nil {
		if err := ValidateAllowlist(ctx, *zoneID); err != nil {
			return "", Citation{}, err
		}
	}
	alerts, err := f.querier.GetActiveAlerts(ctx, limit)
	if err != nil {
		return "", Citation{}, fmt.Errorf("get active alerts failed: %w", err)
	}
	reply := FilterOutput(fmt.Sprintf("Alertas activas %d", len(alerts)))
	return reply, Citation{Tool: ToolActiveAlerts, Count: len(alerts)}, nil
}

func (f *AssistantFlow) handleListPlates(ctx context.Context, args map[string]any) (string, Citation, error) {
	plates, err := f.querier.ListPlates(ctx)
	if err != nil {
		return "", Citation{}, fmt.Errorf("list plates failed: %w", err)
	}
	if len(plates) == 0 {
		return FilterOutput("No hay placas registradas en la base de datos."), Citation{Tool: ToolListPlates, Count: 0}, nil
	}
	reply := FilterOutput("Placas registradas: " + strings.Join(plates, ", "))
	return reply, Citation{Tool: ToolListPlates, Count: len(plates)}, nil
}

func (f *AssistantFlow) dispatch(ctx context.Context, tc *ToolCall) (string, Citation, error) {
	h, ok := f.registry[tc.Name]
	if !ok {
		return FilterOutput(fmt.Sprintf("tool %s no reconocido", tc.Name)), Citation{}, nil
	}
	var out string
	var cit Citation
	var hErr error
	_, cbErr := f.breaker.Execute(func() (any, error) { out, cit, hErr = h(ctx, tc.Args); return nil, hErr })
	if cbErr != nil {
		if cbErr == gobreaker.ErrOpenState {
			return "", Citation{}, fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
		}
		if hErr != nil {
			return "", Citation{}, hErr
		}
		return "", Citation{}, fmt.Errorf("breaker error: %w", cbErr)
	}
	return out, cit, hErr
}

func (f *AssistantFlow) SetGenkit(g *genkit.Genkit, model string) {
	f.toolsMu.Lock()
	defer f.toolsMu.Unlock()
	f.g = g
	f.model = model
	f.tools = nil
}

func (f *AssistantFlow) Chat(ctx context.Context, input ChatInput) (ChatOutput, error) {
	if err := shared.ValidateMessage(input.Message); err != nil {
		return ChatOutput{}, fmt.Errorf("chat validation: %w", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, FlowTimeout)
	defer cancel()
	if err := f.acquire(timeoutCtx, ctx); err != nil {
		return ChatOutput{}, err
	}
	defer f.release()
	if empty, err := f.maybeDelay(timeoutCtx, ctx); err != nil {
		return ChatOutput{}, err
	} else if empty {
		return ChatOutput{Reply: FilterOutput("Respuesta operativa: sin datos adicionales solicitados.")}, nil
	}
	if f.g != nil {
		if v := os.Getenv("GEMINI_API_KEY"); v == "" {
			return ChatOutput{}, fmt.Errorf("503 service unavailable: GEMINI_API_KEY missing: %w", shared.ErrValidation)
		}
		out, err := f.chatWithGenkit(timeoutCtx, input)
		if err == nil {
			return out, nil
		}
		if strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "UNAVAILABLE") || strings.Contains(err.Error(), "high demand") || strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "NOT_FOUND") || strings.Contains(err.Error(), "context deadline") || strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "Quota exceeded") {
			return ChatOutput{}, fmt.Errorf("503 service unavailable: modelo Gemini temporalmente no disponible o saturado, intente más tarde: %w", err)
		}
		return ChatOutput{}, err
	}
	tc := f.resolveToolCall(input)
	if tc == nil {
		return ChatOutput{Reply: FilterOutput("Respuesta operativa: no se requirió tool adicional.")}, nil
	}
	out, cit, err := f.dispatch(timeoutCtx, tc)
	if err != nil {
		return ChatOutput{}, err
	}
	if cit.Tool == "" {
		return ChatOutput{Reply: out}, nil
	}
	return ChatOutput{Reply: out, Citations: []Citation{cit}}, nil
}

func (f *AssistantFlow) listPlates(ctx context.Context) ([]string, error) {
	if f.querier == nil {
		return nil, fmt.Errorf("querier not set: %w", shared.ErrValidation)
	}
	return f.querier.ListPlates(ctx)
}

func buildStoppedReply(rows []domain.StoppedVehicle) string {
	if len(rows) == 0 {
		return "No hay vehículos detenidos en zonas críticas con ese criterio."
	}
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, fmt.Sprintf("%s lleva %dm en %s", r.Plate, r.DurationMin, r.ZoneName))
	}
	return strings.Join(parts, ", ")
}
