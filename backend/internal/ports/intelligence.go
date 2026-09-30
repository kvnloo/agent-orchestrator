package ports

import "context"

// SpawnDecisionSchema is the versioned AO-to-intelligence spawn opportunity contract.
const SpawnDecisionSchema = "ao.z0int.spawn.v1"

// SpawnDecisionCurrent is AO's already-resolved choice. Intelligence may
// inspect it, but shadow mode never mutates it.
type SpawnDecisionCurrent struct {
	Harness    string `json:"harness"`
	Model      string `json:"model"`
	Mode       string `json:"mode"`
	Permission string `json:"permission"`
}

// SpawnDecisionConstraints distinguishes a caller's explicit choices from AO
// defaults. A future promoted policy must never silently override explicit
// human constraints.
type SpawnDecisionConstraints struct {
	ExplicitHarness bool `json:"explicit_harness"`
	ExplicitModel   bool `json:"explicit_model"`
	ExplicitMode    bool `json:"explicit_mode"`
}

// SpawnDecisionRequest is the bounded, secret-free decision opportunity AO
// exposes to an optional intelligence plane.
type SpawnDecisionRequest struct {
	Schema      string                   `json:"schema"`
	TraceID     string                   `json:"trace_id"`
	SessionID   string                   `json:"session_id"`
	ProjectID   string                   `json:"project_id"`
	Kind        string                   `json:"kind"`
	Task        string                   `json:"task"`
	Current     SpawnDecisionCurrent     `json:"current"`
	Constraints SpawnDecisionConstraints `json:"constraints"`
}

// SpawnDecision is non-authoritative advice. AO owns execution and permission
// truth; this contract exists so a sidecar can be measured before promotion.
type SpawnDecision struct {
	Schema                string               `json:"schema"`
	DecisionID            string               `json:"decision_id"`
	Action                string               `json:"action"`
	Recommendation        SpawnDecisionCurrent `json:"recommendation"`
	Confidence            *float64             `json:"confidence"`
	Reason                string               `json:"reason"`
	PolicyRevision        string               `json:"policy_revision"`
	RequestedVerification []string             `json:"requested_verification"`
	ReceiptID             string               `json:"receipt_id"`
	Replayed              bool                 `json:"replayed"`
}

// IntelligenceAdvisor is deliberately orthogonal to AgentResolver. An
// intelligence plane may recommend policy, but it is not an agent harness and
// cannot own AO session lifecycle.
type IntelligenceAdvisor interface {
	AdviseSpawn(context.Context, SpawnDecisionRequest) (SpawnDecision, error)
}
