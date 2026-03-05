package drift

import "github.com/google/uuid"

const (
	StatusOK      Status = "ok"
	StatusDrift   Status = "drift"
	StatusUnknown Status = "unknown"
)

type Snapshot struct {
	ID           uuid.UUID      `json:"id"`
	AtUnix       int64          `json:"atUnix"`
	Scope        string         `json:"scope"`
	Status       Status         `json:"status"`
	DesiredHash  string         `json:"desiredHash"`
	ObservedHash string         `json:"observedHash"`
	Details      map[string]any `json:"details"`
}
