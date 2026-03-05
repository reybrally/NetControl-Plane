package usecases

import "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"

type IntentService struct {
	Tx      outbound.Tx
	Intents outbound.IntentRepo
	Plans   outbound.PlanRepo
	Jobs    outbound.JobQueue
	Audit   outbound.AuditRepo
}
