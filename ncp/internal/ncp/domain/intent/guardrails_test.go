package intent

import "testing"

func baseSpec() Spec {
	return Spec{
		Envs: []string{"dev"},
		Owner: Owner{
			Team: "payments",
		},
		Subject: WorkloadRef{
			Cluster:   "c1",
			Namespace: "payments",
			Selector:  map[string]string{"app": "svc-a"},
		},
		Destinations: []Destination{{
			Type: "service",
			Service: &ServiceRef{
				Cluster:   "c1",
				Namespace: "payments",
				Name:      "svc-b",
			},
		}},
		Rules: []Rule{{
			Direction: DirEgress,
			Protocol:  ProtoTCP,
			Ports:     []int{443},
		}},
	}
}

func TestGuardrails_AllowsSafeSpec(t *testing.T) {
	s := baseSpec()
	if err := s.ValidateGuardrails(); err != nil {
		t.Fatalf("expected no guardrail error, got: %v", err)
	}
}

func TestGuardrails_DeniesInternetSSH(t *testing.T) {
	s := baseSpec()
	s.Destinations = []Destination{{Type: "cidr", CIDR: "0.0.0.0/0"}}
	s.Rules = []Rule{{Direction: DirIngress, Protocol: ProtoTCP, Ports: []int{22}}}

	err := s.ValidateGuardrails()
	if err == nil {
		t.Fatalf("expected guardrail error")
	}
	ge, ok := err.(GuardrailError)
	if !ok {
		t.Fatalf("expected GuardrailError, got %T", err)
	}
	if len(ge.Violations) == 0 {
		t.Fatalf("expected at least 1 violation")
	}
	if ge.Violations[0].ID != "INTERNET_SSH_RDP" {
		t.Fatalf("unexpected violation id: %s", ge.Violations[0].ID)
	}
}

func TestGuardrails_ProdInternetEgressRequiresApproval(t *testing.T) {
	s := baseSpec()
	s.Envs = []string{"prod"}
	s.Destinations = []Destination{{Type: "cidr", CIDR: "0.0.0.0/0"}}
	s.Rules = []Rule{{Direction: DirEgress, Protocol: ProtoTCP, Ports: []int{443}}}
	s.Constraints.ApprovalRequired = false

	err := s.ValidateGuardrails()
	if err == nil {
		t.Fatalf("expected guardrail error")
	}
	ge := err.(GuardrailError)
	found := false
	for _, v := range ge.Violations {
		if v.ID == "PROD_INTERNET_EGRESS_REQUIRES_APPROVAL" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PROD_INTERNET_EGRESS_REQUIRES_APPROVAL violation, got %+v", ge.Violations)
	}
}
