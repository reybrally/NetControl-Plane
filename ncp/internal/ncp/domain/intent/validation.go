package intent

import (
	"errors"
	"fmt"
)

func (s Spec) ValidateBasic() error {
	if len(s.Envs) == 0 {
		return fmt.Errorf("envs empty")
	}
	if s.Owner.Team == "" {
		return fmt.Errorf("owner.team empty")
	}
	if s.Subject.Cluster == "" || s.Subject.Namespace == "" {
		return fmt.Errorf("subject scope empty")
	}
	if len(s.Subject.Selector) == 0 {
		return fmt.Errorf("subject.selector empty")
	}
	if len(s.Destinations) == 0 {
		return fmt.Errorf("destinations empty")
	}
	if len(s.Rules) == 0 {
		return fmt.Errorf("rules empty")
	}
	for _, r := range s.Rules {
		if len(r.Ports) == 0 {
			return errors.New("ports are required for each rule")
		}
		if r.Protocol == "" {
			return errors.New("protocol is required for each rule")
		}
		if r.Direction == "" {
			return errors.New("direction is required for each rule")
		}
	}
	return nil
}

func (s Spec) ValidateGuardrails() error {
	vr := EvaluateGuardrails(s)
	if len(vr.Violations) == 0 {
		return nil
	}
	return vr
}
