package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go/option"
	"github.com/sony/gobreaker"

	assistantGenkit "fleetmonitoring/backend/internal/assistant/adapters/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	asshttp "fleetmonitoring/backend/internal/assistant/adapters/http"
	"fleetmonitoring/backend/internal/assistant/application"
	"fleetmonitoring/backend/internal/assistant/domain"
	assistantbreaker "fleetmonitoring/backend/internal/assistant/infra/breaker"
	fleetpg "fleetmonitoring/backend/internal/fleet/adapters/pg"
	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/shared/idgen"
)

type agentOpsProvider struct {
	breaker *gobreaker.CircuitBreaker
	nc      *nats.Conn
	pool    *pgxpool.Pool
}

func (o *agentOpsProvider) BreakerState() string {
	if o.breaker == nil {
		return "closed"
	}
	switch o.breaker.State() {
	case gobreaker.StateOpen:
		return "open"
	case gobreaker.StateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

func (o *agentOpsProvider) DBPool() string {
	if o.pool == nil {
		return "unknown"
	}
	s := o.pool.Stat()
	return fmt.Sprintf("total=%d idle=%d", s.TotalConns(), s.IdleConns())
}

func (o *agentOpsProvider) GeminiState() string {
	if v := os.Getenv("OPENCODE_API_KEY"); v != "" {
		return "connected:opencode:" + os.Getenv("OPENCODE_MODEL")
	}
	if v := os.Getenv("GEMINI_API_KEY"); v == "" {
		return "missing-key"
	}
	return "connected:gemini:" + os.Getenv("GEMINI_MODEL")
}

func Bootstrap(ctx context.Context) (*Server, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	agentPort := os.Getenv("AGENT_PORT")
	if agentPort == "" {
		if v := os.Getenv("API_PORT"); v != "" {
			agentPort = v
		} else if v := os.Getenv("HTTP_PORT"); v != "" {
			agentPort = v
		} else {
			agentPort = "8080"
		}
	}
	if key := os.Getenv("GEMINI_API_KEY"); key == "" {
		slog.Warn("GEMINI_API_KEY not set — agent will answer healthz degraded but chat fallback 503")
	}

	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		slog.Warn("nats connect failed — agent degrades", "error", err)
		nc = nil
	} else {
		slog.Info("nats connected", "url", natsURL)
	}

	var pool *pgxpool.Pool
	if databaseURL != "" {
		p, err := pgxpool.New(ctx, databaseURL)
		if err != nil {
			slog.Warn("pgxpool failed — agent degrades", "error", err)
		} else {
			pool = p
			slog.Info("db pool ready")
		}
	} else {
		slog.Warn("DATABASE_URL not set — agent degrades")
	}

	cb := gobreaker.NewCircuitBreaker(assistantbreaker.NewSettings(assistantbreaker.DefaultTimeout))

	ops := &agentOpsProvider{breaker: cb, nc: nc, pool: pool}
	opsHandler := asshttp.NewOpsHandler(ops)

	var querier application.FleetQuerier = &agentQuerier{pool: pool}
	if pool != nil {
		adapter := fleetpg.NewPgxPoolAdapter(pool)
		reader := fleetpg.NewStoppedReader(adapter)
		querier = &agentQuerier{pool: pool, stopped: reader}
	}
	flow := assistantGenkit.NewAssistantFlow(querier, &assistantGenkit.StubGeminiClient{})
	if ocKey := os.Getenv("OPENCODE_API_KEY"); ocKey != "" {
		ocBase := os.Getenv("OPENCODE_BASE_URL")
		ocModel := os.Getenv("OPENCODE_MODEL")
		if ocModel == "" {
			ocModel = "muse-spark-1.2-contributor"
		}
		if ocBase == "" {
			ocBase = "https://api.opencode.ai/v1"
		}
		slog.Info("initializing genkit with opencode model", "model", ocModel, "base", ocBase)
		var opts []option.RequestOption
		if ocBase != "" {
			opts = append(opts, option.WithBaseURL(ocBase))
		}
		oai := &openai.OpenAI{APIKey: ocKey, Opts: opts}
		g := genkit.Init(ctx, genkit.WithPlugins(oai), genkit.WithDefaultModel("openai/"+ocModel))
		flow.SetGenkit(g, "openai/"+ocModel)
		slog.Info("genkit initialized with opencode")
	} else {
		apiKey := os.Getenv("GEMINI_API_KEY")
		modelEnv := os.Getenv("GEMINI_MODEL")
		if modelEnv == "" {
			modelEnv = "gemini-2.5-flash"
		}
		if apiKey != "" {
			slog.Info("initializing genkit with gemini model", "model", modelEnv)
			g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: apiKey, APIVersion: "v1"}), genkit.WithDefaultModel("googleai/"+modelEnv))
			flow.SetGenkit(g, modelEnv)
			slog.Info("genkit initialized")
		} else {
			slog.Warn("GEMINI_API_KEY not set — agent degraded")
		}
	}

	chatHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			http.Error(w, `{"error":"validation","message":"Content-Type must be application/json"}`, http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<14)
		var payload struct {
			Message *string `json:"message"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&payload); err != nil {
			http.Error(w, `{"error":"validation","message":"invalid json"}`, http.StatusBadRequest)
			return
		}
		if payload.Message == nil || strings.TrimSpace(*payload.Message) == "" {
			http.Error(w, `{"error":"validation","message":"message is required"}`, http.StatusBadRequest)
			return
		}
		if err := shared.ValidateMessage(*payload.Message); err != nil {
			http.Error(w, `{"error":"validation","message":"message must be 1..4000 runes"}`, http.StatusBadRequest)
			return
		}
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" || !shared.IsValidUUID(reqID) {
			reqID = idgen.GenerateUUID()
		}
		w.Header().Set("X-Request-ID", reqID)
		w.Header().Set("Content-Type", "application/json")
		ctx2 := shared.WithRequestID(r.Context(), reqID)
		if cb.State() == gobreaker.StateOpen {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "agente temporalmente no disponible"})
			return
		}
		if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("OPENCODE_API_KEY") == "" {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "503 service unavailable: no LLM API key configured"})
			return
		}
		out, err := flow.Chat(ctx2, assistantGenkit.ChatInput{Message: strings.TrimSpace(*payload.Message)})
		if err != nil {
			status := http.StatusBadGateway
			if strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "breaker open") {
				status = http.StatusServiceUnavailable
				w.Header().Set("Retry-After", "30")
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		resp := map[string]any{"reply": out.Reply, "request_id": reqID}
		if len(out.Citations) > 0 {
			resp["citations"] = out.Citations
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	addr := ":" + agentPort
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", chatHandler.ServeHTTP)
	mux.Handle("/", opsHandler)
	srv := NewServer(mux, addr, nc, pool)
	_ = time.Now
	return srv, nil
}

type agentQuerier struct {
	pool    *pgxpool.Pool
	stopped *fleetpg.StoppedReader
}

func (q *agentQuerier) FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	if q.stopped != nil {
		return q.stopped.FindStoppedInZones(ctx, minMinutes, zoneID, limit)
	}
	return []domain.StoppedVehicle{}, nil
}

func (q *agentQuerier) GetFleetSummary(ctx context.Context) (application.FleetSummary, error) {
	if q.pool == nil {
		return application.FleetSummary{}, nil
	}
	var total, moving, idle int
	_ = q.pool.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE received_at > now() - interval '5 minutes'`).Scan(&total)
	_ = q.pool.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE speed > 0 AND received_at > now() - interval '5 minutes'`).Scan(&moving)
	idle = total - moving
	if idle < 0 {
		idle = 0
	}
	return application.FleetSummary{Total: total, Moving: moving, Idle: idle}, nil
}

func (q *agentQuerier) GetVehicleStatus(ctx context.Context, plate shared.Plate) (application.VehicleStatus, error) {
	if q.pool == nil {
		return application.VehicleStatus{Plate: plate, Status: "unknown"}, nil
	}
	var lat, lon, speed float64
	var receivedAt time.Time
	err := q.pool.QueryRow(ctx, `SELECT lat, lon, speed, received_at FROM telemetry WHERE plate=$1 ORDER BY received_at DESC LIMIT 1`, string(plate)).Scan(&lat, &lon, &speed, &receivedAt)
	if err != nil {
		return application.VehicleStatus{Plate: plate, Status: "not_found"}, nil
	}
	status := "detenido"
	if speed > 5 {
		status = "en movimiento"
	}
	return application.VehicleStatus{Plate: plate, Lat: &lat, Lon: &lon, Speed: speed, ReceivedAt: receivedAt, Status: status}, nil
}

func (q *agentQuerier) GetActiveAlerts(ctx context.Context, limit int) ([]application.Alert, error) {
	return []application.Alert{}, nil
}

func (q *agentQuerier) ListPlates(ctx context.Context) ([]string, error) {
	if q.pool == nil {
		return []string{}, nil
	}
	rows, err := q.pool.Query(ctx, `SELECT DISTINCT plate FROM telemetry ORDER BY plate LIMIT 20`)
	if err != nil {
		return nil, fmt.Errorf("list plates failed: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan plate: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}
