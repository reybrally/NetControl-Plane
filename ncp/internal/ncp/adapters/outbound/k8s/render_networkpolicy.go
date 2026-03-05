package k8s

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func RenderNetworkPolicy(obj map[string]any) (*netv1.NetworkPolicy, string, error) {
	kind, _ := obj["kind"].(string)
	if kind != "NetworkPolicy" {
		return nil, "", fmt.Errorf("k8s artifact kind must be NetworkPolicy, got %q", kind)
	}

	ns, _ := obj["namespace"].(string)
	if ns == "" {
		return nil, "", fmt.Errorf("namespace required")
	}

	name := "ncp-policy"
	if sel, ok := obj["podSelector"].(map[string]any); ok {
		if v, ok := sel["app"].(string); ok && v != "" {
			name = "ncp-" + sanitizeName(v)
		}
	}

	podSel := metav1.LabelSelector{MatchLabels: toStringMap(obj["podSelector"])}

	rules, _ := obj["rules"].([]any)
	dests, _ := obj["destinations"].([]any)

	ingressRules, egressRules, policyTypes, err := buildPolicyRules(rules, dests)
	if err != nil {
		return nil, "", err
	}

	np := &netv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"managed-by": "ncp",
			},
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: podSel,
			PolicyTypes: policyTypes,
			Ingress:     ingressRules,
			Egress:      egressRules,
		},
	}

	return np, name, nil
}

func buildPolicyRules(rules []any, dests []any) ([]netv1.NetworkPolicyIngressRule, []netv1.NetworkPolicyEgressRule, []netv1.PolicyType, error) {
	var (
		ingress     []netv1.NetworkPolicyIngressRule
		egress      []netv1.NetworkPolicyEgressRule
		wantIngress bool
		wantEgress  bool
	)

	ports, err := portsFromRules(rules)
	if err != nil {
		return nil, nil, nil, err
	}
	peers, err := peersFromDestinations(dests)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, r := range rules {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		dir, _ := rm["direction"].(string)
		dir = strings.ToLower(dir)

		switch dir {
		case "ingress":
			wantIngress = true

			if len(peers) == 0 && len(ports) == 0 {
				continue
			}
			ingress = append(ingress, netv1.NetworkPolicyIngressRule{From: peers, Ports: ports})
		case "egress":
			wantEgress = true
			if len(peers) == 0 && len(ports) == 0 {
				continue
			}
			egress = append(egress, netv1.NetworkPolicyEgressRule{To: peers, Ports: ports})
		case "both":
			wantIngress, wantEgress = true, true
			if !(len(peers) == 0 && len(ports) == 0) {
				ingress = append(ingress, netv1.NetworkPolicyIngressRule{From: peers, Ports: ports})
				egress = append(egress, netv1.NetworkPolicyEgressRule{To: peers, Ports: ports})
			}
		}
	}

	var policyTypes []netv1.PolicyType
	if wantIngress {
		policyTypes = append(policyTypes, netv1.PolicyTypeIngress)
	}
	if wantEgress {
		policyTypes = append(policyTypes, netv1.PolicyTypeEgress)
	}
	if !wantIngress && !wantEgress {

		policyTypes = []netv1.PolicyType{netv1.PolicyTypeEgress}
	}

	return ingress, egress, policyTypes, nil
}

func portsFromRules(rules []any) ([]netv1.NetworkPolicyPort, error) {
	var out []netv1.NetworkPolicyPort

	for _, r := range rules {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		protoStr, _ := rm["protocol"].(string)
		protoStr = strings.ToUpper(protoStr)
		var proto corev1.Protocol
		switch protoStr {
		case "UDP":
			proto = corev1.ProtocolUDP
		default:
			proto = corev1.ProtocolTCP
		}

		portsAny, _ := rm["ports"].([]any)
		for _, p := range portsAny {
			portNum, ok := toInt32(p)
			if !ok || portNum <= 0 {
				continue
			}
			v := intstr.FromInt32(portNum)
			out = append(out, netv1.NetworkPolicyPort{
				Protocol: &proto,
				Port:     &v,
			})
		}
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func peersFromDestinations(dests []any) ([]netv1.NetworkPolicyPeer, error) {
	if len(dests) == 0 {
		return nil, nil
	}

	var peers []netv1.NetworkPolicyPeer
	for _, d := range dests {
		dm, ok := d.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := dm["type"].(string)
		typ = strings.ToLower(typ)

		switch typ {
		case "service":
			svcAny, _ := dm["service"].(map[string]any)
			ns, _ := svcAny["namespace"].(string)
			if ns == "" {
				continue
			}
			peers = append(peers, netv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": ns,
					},
				},
			})
		case "cidr":
			c, _ := dm["cidr"].(string)
			if c == "" {
				continue
			}
			peers = append(peers, netv1.NetworkPolicyPeer{
				IPBlock: &netv1.IPBlock{CIDR: c},
			})
		}
	}

	if len(peers) == 0 {
		return nil, nil
	}
	return peers, nil
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}

func toStringMap(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		if ms, ok := v.(map[string]string); ok {
			return ms
		}
		return out
	}
	for k, vv := range m {
		if s, ok := vv.(string); ok {
			out[k] = s
		}
	}
	return out
}

func toInt32(v any) (int32, bool) {
	switch t := v.(type) {
	case int:
		return int32(t), true
	case int32:
		return t, true
	case int64:
		return int32(t), true
	case float64:
		return int32(t), true
	case string:
		n, err := strconv.Atoi(t)
		if err != nil {
			return 0, false
		}
		return int32(n), true
	default:
		return 0, false
	}
}
