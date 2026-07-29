package event

import "time"

// Event is the transport-neutral representation produced by collectors.
type Event struct {
	SchemaVersion string         `json:"schema_version"`
	ID            string         `json:"event_id"`
	RouterID      string         `json:"router_id"`
	Category      string         `json:"category"`
	Type          string         `json:"type"`
	Source        string         `json:"source"`
	Timestamp     time.Time      `json:"timestamp"`
	Data          map[string]any `json:"data"`
}
