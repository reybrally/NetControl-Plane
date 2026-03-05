package drift

import "github.com/google/uuid"

type Status string

const (
	StatusDetected Status = "DETECTED"
	StatusResolved Status = "RESOLVED"
)

type Type string

const (
	TypeMissing  Type = "MISSING"
	TypeModified Type = "MODIFIED"
	TypeExtra    Type = "EXTRA"
)

type Drift struct {
	ID         uuid.UUID
	Provider   string
	Kind       string
	Namespace  string
	Name       string
	Type       Type
	Expected   string
	Actual     string
	Status     Status
	DetectedAt int64
}
