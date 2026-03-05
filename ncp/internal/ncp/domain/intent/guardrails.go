package intent

import (
	"encoding/json"
	"fmt"
	"strings"
)

type GuardrailSeverity string

const (
	SeverityLow    GuardrailSeverity = "LOW"
	SeverityMedium GuardrailSeverity = "MEDIUM"
	SeverityHigh   GuardrailSeverity = "HIGH"
)

type GuardrailViolation struct {
	ID       string            `json:"id"`
	Severity GuardrailSeverity `json:"severity"`
	Message  string            `json:"message"`
	Path     string            `json:"path,omitempty"`
}

type GuardrailError struct {
	Violations []GuardrailViolation `json:"violations"`
}

func (e GuardrailError) Error() string {

	return fmt.Sprintf("guardrails denied: %d violation(s)", len(e.Violations))
}

func (e GuardrailError) AsMeta() map[string]any {
	b, _ := json.Marshal(e)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func EvaluateGuardrails(s Spec) GuardrailError {
	var out GuardrailError

	isProd := false
	for _, env := range s.Envs {
		if strings.EqualFold(env, "prod") || strings.EqualFold(env, "production") {
			isProd = true
			break
		}
	}

	hasInternetDest := false
	for _, d := range s.Destinations {
		if strings.EqualFold(d.Type, "cidr") && strings.TrimSpace(d.CIDR) == "0.0.0.0/0" {
			hasInternetDest = true
			break
		}
	}

	if hasInternetDest {
		for i, r := range s.Rules {
			if !strings.EqualFold(string(r.Protocol), string(ProtoTCP)) {
				continue
			}
			for _, p := range r.Ports {
				if p == 22 || p == 3389 {
					out.Violations = append(out.Violations, GuardrailViolation{
						ID:       "INTERNET_SSH_RDP",
						Severity: SeverityHigh,
						Message:  "internet CIDR (0.0.0.0/0) must not be used with SSH (22) or RDP (3389)",
						Path:     fmt.Sprintf("rules[%d].ports", i),
					})
				}
			}
		}
	}

	if isProd && hasInternetDest {
		internetEgress := false
		for _, r := range s.Rules {
			if r.Direction == DirEgress || r.Direction == DirBoth {
				internetEgress = true
				break
			}
		}

		if internetEgress && !s.Constraints.ApprovalRequired {
			out.Violations = append(out.Violations, GuardrailViolation{
				ID:       "PROD_INTERNET_EGRESS_REQUIRES_APPROVAL",
				Severity: SeverityHigh,
				Message:  "prod internet egress requires constraints.approvalRequired=true",
				Path:     "constraints.approvalRequired",
			})
		}
	}

	return out
}
