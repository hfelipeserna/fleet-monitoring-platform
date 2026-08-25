package genkit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/sony/gobreaker"
	"google.golang.org/genai"
)

type citationKey struct{}

type citationCollector struct {
	mu        sync.Mutex
	citations []Citation
}

type findStoppedInput struct {
	MinMinutes int     `json:"minMinutes" jsonschema_description:"Minimum minutes vehicle has been stopped"`
	ZoneID     *string `json:"zoneId,omitempty" jsonschema_description:"Zone ID filter optional UUID"`
	Limit      *int    `json:"limit,omitempty" jsonschema_description:"Maximum number of results"`
}

type fleetSummaryInput struct {
	ZoneID *string `json:"zoneId,omitempty" jsonschema_description:"Zone ID filter optional"`
}

type vehicleStatusInput struct {
	Plate string `json:"plate" jsonschema_description:"Vehicle plate identifier"`
}

type activeAlertsInput struct {
	Limit  *int    `json:"limit,omitempty" jsonschema_description:"Maximum number of results"`
	ZoneID *string `json:"zoneId,omitempty" jsonschema_description:"Zone ID filter optional"`
}

func (f *AssistantFlow) ensureGenkitTools() []ai.ToolRef {
	f.toolsMu.Lock()
	defer f.toolsMu.Unlock()
	if f.tools != nil && len(f.tools) == 5 {
		return f.tools
	}
	if f.g == nil {
		return nil
	}
	if t := genkit.LookupTool(f.g, ToolFindStopped); t != nil {
		existing := []ai.ToolRef{
			genkit.LookupTool(f.g, ToolFindStopped),
			genkit.LookupTool(f.g, ToolFleetSummary),
			genkit.LookupTool(f.g, ToolVehicleStatus),
			genkit.LookupTool(f.g, ToolActiveAlerts),
			genkit.LookupTool(f.g, ToolListPlates),
		}
		allFound := true
		for _, e := range existing {
			if e == nil {
				allFound = false
				break
			}
		}
		if allFound {
			f.tools = existing
			return f.tools
		}
	}
	collectorFromCtx := func(ctx *ai.ToolContext) *citationCollector {
		if v := ctx.Context.Value(citationKey{}); v != nil {
			if c, ok := v.(*citationCollector); ok {
				return c
			}
		}
		return nil
	}
	findTool := genkit.DefineTool(f.g, ToolFindStopped, "Find vehicles stopped in critical zones longer than minMinutes. Use for detenidos queries.",
		func(ctx *ai.ToolContext, input findStoppedInput) (string, error) {
			args := map[string]any{"minMinutes": input.MinMinutes}
			if input.ZoneID != nil && *input.ZoneID != "" {
				args["zoneId"] = *input.ZoneID
			}
			if input.Limit != nil {
				args["limit"] = *input.Limit
			}
			var out string
			var cit Citation
			var hErr error
			_, cbErr := f.breaker.Execute(func() (any, error) {
				out, cit, hErr = f.handleFindStopped(ctx.Context, args)
				return nil, hErr
			})
			if cbErr != nil {
				if cbErr == gobreaker.ErrOpenState {
					return "", fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
				}
				if hErr != nil {
					return "", hErr
				}
				return "", fmt.Errorf("breaker error: %w", cbErr)
			}
			if hErr != nil {
				return "", hErr
			}
			if c := collectorFromCtx(ctx); c != nil {
				c.mu.Lock()
				c.citations = append(c.citations, cit)
				c.mu.Unlock()
			}
			return out, nil
		})

	summaryTool := genkit.DefineTool(f.g, ToolFleetSummary, "Get fleet summary totals moving idle.",
		func(ctx *ai.ToolContext, input fleetSummaryInput) (string, error) {
			args := map[string]any{}
			if input.ZoneID != nil && *input.ZoneID != "" {
				args["zoneId"] = *input.ZoneID
			}
			var out string
			var cit Citation
			var hErr error
			_, cbErr := f.breaker.Execute(func() (any, error) {
				out, cit, hErr = f.handleFleetSummary(ctx.Context, args)
				return nil, hErr
			})
			if cbErr != nil {
				if cbErr == gobreaker.ErrOpenState {
					return "", fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
				}
				if hErr != nil {
					return "", hErr
				}
				return "", fmt.Errorf("breaker error: %w", cbErr)
			}
			if hErr != nil {
				return "", hErr
			}
			if c := collectorFromCtx(ctx); c != nil {
				c.mu.Lock()
				c.citations = append(c.citations, cit)
				c.mu.Unlock()
			}
			return out, nil
		})

	statusTool := genkit.DefineTool(f.g, ToolVehicleStatus, "Get vehicle status by plate.",
		func(ctx *ai.ToolContext, input vehicleStatusInput) (string, error) {
			args := map[string]any{"plate": input.Plate}
			var out string
			var cit Citation
			var hErr error
			_, cbErr := f.breaker.Execute(func() (any, error) {
				out, cit, hErr = f.handleVehicleStatus(ctx.Context, args)
				return nil, hErr
			})
			if cbErr != nil {
				if cbErr == gobreaker.ErrOpenState {
					return "", fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
				}
				if hErr != nil {
					return "", hErr
				}
				return "", fmt.Errorf("breaker error: %w", cbErr)
			}
			if hErr != nil {
				return "", hErr
			}
			if c := collectorFromCtx(ctx); c != nil {
				c.mu.Lock()
				c.citations = append(c.citations, cit)
				c.mu.Unlock()
			}
			return out, nil
		})

	alertsTool := genkit.DefineTool(f.g, ToolActiveAlerts, "Get active alerts list.",
		func(ctx *ai.ToolContext, input activeAlertsInput) (string, error) {
			args := map[string]any{}
			if input.Limit != nil {
				args["limit"] = *input.Limit
			}
			if input.ZoneID != nil && *input.ZoneID != "" {
				args["zoneId"] = *input.ZoneID
			}
			var out string
			var cit Citation
			var hErr error
			_, cbErr := f.breaker.Execute(func() (any, error) {
				out, cit, hErr = f.handleActiveAlerts(ctx.Context, args)
				return nil, hErr
			})
			if cbErr != nil {
				if cbErr == gobreaker.ErrOpenState {
					return "", fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
				}
				if hErr != nil {
					return "", hErr
				}
				return "", fmt.Errorf("breaker error: %w", cbErr)
			}
			if hErr != nil {
				return "", hErr
			}
			if c := collectorFromCtx(ctx); c != nil {
				c.mu.Lock()
				c.citations = append(c.citations, cit)
				c.mu.Unlock()
			}
			return out, nil
		})

	type listPlatesInput struct{}
	listPlatesTool := genkit.DefineTool(f.g, ToolListPlates, "List distinct plates registered in telemetry. Use for 'cuales placas hay registradas'.",
		func(ctx *ai.ToolContext, input listPlatesInput) (string, error) {
			var out string
			var cit Citation
			var hErr error
			_, cbErr := f.breaker.Execute(func() (any, error) {
				out, cit, hErr = f.handleListPlates(ctx.Context, map[string]any{})
				return nil, hErr
			})
			if cbErr != nil {
				if cbErr == gobreaker.ErrOpenState {
					return "", fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
				}
				if hErr != nil {
					return "", hErr
				}
				return "", fmt.Errorf("breaker error: %w", cbErr)
			}
			if hErr != nil {
				return "", hErr
			}
			if c := collectorFromCtx(ctx); c != nil {
				c.mu.Lock()
				c.citations = append(c.citations, cit)
				c.mu.Unlock()
			}
			return out, nil
		})

	f.tools = []ai.ToolRef{findTool, summaryTool, statusTool, alertsTool, listPlatesTool}
	return f.tools
}

func (f *AssistantFlow) chatWithGenkit(ctx context.Context, input ChatInput) (ChatOutput, error) {
	if f.breaker.State() == gobreaker.StateOpen {
		return ChatOutput{}, fmt.Errorf("503 service unavailable: breaker open: %w", gobreaker.ErrOpenState)
	}
	collector := &citationCollector{}
	ctxWithCollector := context.WithValue(ctx, citationKey{}, collector)
	tools := f.ensureGenkitTools()
	modelName := f.model
	if modelName == "" {
		modelName = os.Getenv("GEMINI_MODEL")
		if modelName == "" {
			modelName = "gemini-2.5-flash"
		}
	}
	if !strings.HasPrefix(modelName, "googleai/") && !strings.HasPrefix(modelName, "google/") && !strings.Contains(modelName, "/") {
		modelName = "googleai/" + modelName
	}
	var resp *ai.ModelResponse
	var genErr error
	isGoogle := strings.HasPrefix(modelName, "googleai/") || strings.HasPrefix(modelName, "google/")
	var genOpts []ai.GenerateOption
	genOpts = append(genOpts, ai.WithSystem(SystemPrompt), ai.WithPrompt("[USER]: "+input.Message), ai.WithModelName(modelName), ai.WithTools(tools...))
	if isGoogle {
		cfg := &genai.GenerateContentConfig{
			Temperature:     genai.Ptr[float32](0.2),
			MaxOutputTokens: int32(MaxOutputTokens),
		}
		genOpts = append(genOpts, ai.WithConfig(cfg))
	}
	_, cbErr := f.breaker.Execute(func() (any, error) {
		var err error
		resp, err = genkit.Generate(ctxWithCollector, f.g, genOpts...)
		genErr = err
		return nil, err
	})
	if cbErr != nil {
		if cbErr == gobreaker.ErrOpenState {
			return ChatOutput{}, fmt.Errorf("503 service unavailable: breaker open: %w", cbErr)
		}
		if genErr != nil {
			return ChatOutput{}, genErr
		}
		return ChatOutput{}, fmt.Errorf("breaker error: %w", cbErr)
	}
	if genErr != nil {
		return ChatOutput{}, fmt.Errorf("gemini generate failed: %w", genErr)
	}
	if resp == nil || resp.Message == nil {
		return ChatOutput{Reply: FilterOutput("Respuesta operativa: sin datos adicionales solicitados.")}, nil
	}
	txt := resp.Text()
	if strings.TrimSpace(txt) == "" {
		txt = "Respuesta operativa: sin datos adicionales solicitados."
	}
	txt = FilterOutput(txt)
	collector.mu.Lock()
	cits := make([]Citation, len(collector.citations))
	copy(cits, collector.citations)
	collector.mu.Unlock()
	return ChatOutput{Reply: txt, Citations: cits}, nil
}
