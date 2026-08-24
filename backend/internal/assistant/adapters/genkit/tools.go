package genkit

import "context"

type Tool struct {
	Name   string
	Schema string
}

const FindStoppedToolSchema = `{"type":"object","properties":{"minMinutes":{"type":"integer"},"zoneId":{"type":"string"}},"required":["minMinutes"]}`

const (
	ToolFindStopped  = "findVehiclesStoppedInCriticalZones"
	ToolFleetSummary = "getFleetSummary"
	ToolVehicleStatus = "getVehicleStatus"
	ToolActiveAlerts = "getActiveAlerts"
)

type ToolHandler func(ctx context.Context, args map[string]any) (string, Citation, error)

var DefineTools = []Tool{
	{Name: ToolFindStopped, Schema: FindStoppedToolSchema},
	{Name: ToolFleetSummary, Schema: `{"type":"object","properties":{}}`},
	{Name: ToolVehicleStatus, Schema: `{"type":"object","properties":{"plate":{"type":"string"}}}`},
	{Name: ToolActiveAlerts, Schema: `{"type":"object","properties":{"limit":{"type":"integer"}}}`},
}
