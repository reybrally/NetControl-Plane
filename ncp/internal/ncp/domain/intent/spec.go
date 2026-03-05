package intent

type Direction string

const (
	DirIngress Direction = "ingress"
	DirEgress  Direction = "egress"
	DirBoth    Direction = "both"
)

type Protocol string

const (
	ProtoTCP Protocol = "TCP"
	ProtoUDP Protocol = "UDP"
)

type WorkloadRef struct {
	Cluster   string            `json:"cluster"`
	Namespace string            `json:"namespace"`
	Selector  map[string]string `json:"selector"`
}

type ServiceRef struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type Destination struct {
	Type    string      `json:"type"`
	Service *ServiceRef `json:"service,omitempty"`
	CIDR    string      `json:"cidr,omitempty"`
	FQDN    string      `json:"fqdn,omitempty"`
}

type Rule struct {
	Direction Direction `json:"direction"`
	Protocol  Protocol  `json:"protocol"`
	Ports     []int     `json:"ports"`
}

type Constraints struct {
	TTLSeconds       *int `json:"ttlSeconds,omitempty"`
	ApprovalRequired bool `json:"approvalRequired"`
}

type Owner struct {
	Team          string `json:"team"`
	CreatedBy     string `json:"createdBy"`
	TicketRef     string `json:"ticketRef"`
	Justification string `json:"justification"`
}

type Spec struct {
	Envs         []string          `json:"envs"`
	Owner        Owner             `json:"owner"`
	Subject      WorkloadRef       `json:"subject"`
	Destinations []Destination     `json:"destinations"`
	Rules        []Rule            `json:"rules"`
	Constraints  Constraints       `json:"constraints"`
	Labels       map[string]string `json:"labels,omitempty"`
}
