package ports

import "context"

// SpawnDecisionSchema is the versioned AO-to-intelligence spawn opportunity contract.
const (
	SpawnDecisionSchema = "ao.z0int.spawn.v1"
	SpawnOutcomeSchema  = "ao.z0int.outcome.v1"
)

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

// SpawnOutcomePR is the bounded SCM evidence attached to a terminal outcome.
// It deliberately excludes review bodies, check logs, credentials, and other
// high-cardinality/private provider payloads.
type SpawnOutcomePR struct {
	URL                      string `json:"url"`
	Number                   int    `json:"number"`
	Draft                    bool   `json:"draft"`
	Merged                   bool   `json:"merged"`
	Closed                   bool   `json:"closed"`
	CI                       string `json:"ci"`
	Review                   string `json:"review"`
	Mergeability             string `json:"mergeability"`
	ReviewComments           bool   `json:"review_comments"`
	ExternalApproved         bool   `json:"external_approved"`
	ExternalChangesRequested bool   `json:"external_changes_requested"`
	ExternalComments         bool   `json:"external_comments"`
	HeadSHA                  string `json:"head_sha,omitempty"`
}

// SpawnOutcome carries only generic training/verifier signals understood by
// z0intelligence's canonical Outcome model. Unknown/negative facts stay absent
// rather than being guessed from session termination.
type SpawnOutcome struct {
	ExecutionCompleted *bool  `json:"execution_completed,omitempty"`
	PRMerged           *bool  `json:"pr_merged,omitempty"`
	CIFailed           *bool  `json:"ci_failed,omitempty"`
	Source             string `json:"source,omitempty"`
	VerificationSource string `json:"verification_source,omitempty"`
}

// SpawnOutcomeEvidence preserves AO-specific durable evidence separately from
// the generic Outcome signal. z0intelligence can use it for attribution and
// calibration without pretending that CI/PR metadata is itself execution truth.
type SpawnOutcomeEvidence struct {
	ProjectID   string           `json:"project_id"`
	Kind        string           `json:"kind"`
	Harness     string           `json:"harness"`
	Mode        string           `json:"mode"`
	Model       string           `json:"model,omitempty"`
	Activity    string           `json:"activity"`
	Disposition string           `json:"disposition"`
	Terminated  bool             `json:"terminated"`
	SCMComplete bool             `json:"scm_complete"`
	PRs         []SpawnOutcomePR `json:"prs"`
}

// SpawnOutcomeRequest joins AO's durable terminal/lifecycle facts back to the
// spawn opportunity through the same stable trace id. OutcomeID is deterministic
// so a receiver can make replay idempotent without AO persisting sidecar state.
type SpawnOutcomeRequest struct {
	Schema    string               `json:"schema"`
	TraceID   string               `json:"trace_id"`
	OutcomeID string               `json:"outcome_id"`
	SessionID string               `json:"session_id"`
	Outcome   SpawnOutcome         `json:"outcome"`
	Evidence  SpawnOutcomeEvidence `json:"evidence"`
}

// IntelligenceAdvisor is deliberately orthogonal to AgentResolver. An
// intelligence plane may recommend policy and receive bounded evidence, but it
// is not an agent harness and cannot own AO session lifecycle or canonical SCM
// state.
type IntelligenceAdvisor interface {
	AdviseSpawn(context.Context, SpawnDecisionRequest) (SpawnDecision, error)
	ObserveOutcome(context.Context, SpawnOutcomeRequest) error
}
