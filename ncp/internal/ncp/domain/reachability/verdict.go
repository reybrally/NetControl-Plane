package reachability

type Verdict string

const (
	Allowed Verdict = "allowed"
	Denied  Verdict = "denied"
	Unknown Verdict = "unknown"
)

type Result struct {
	Verdict  Verdict `json:"verdict"`
	Why      string  `json:"why"`
	Evidence []any   `json:"evidence,omitempty"`
}
