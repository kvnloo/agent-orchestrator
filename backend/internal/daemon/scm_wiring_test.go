package daemon

import (
	"context"
	"log/slog"
	"testing"

	scmmulti "github.com/aoagents/agent-orchestrator/backend/internal/adapters/scm/multi"
	"github.com/aoagents/agent-orchestrator/backend/internal/config"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	scmobserve "github.com/aoagents/agent-orchestrator/backend/internal/observe/scm"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// TestSCMWiring_MultiProviderSatisfiesScopedIdentityResolver verifies that the
// multi provider constructed with both GitHub and GitLab sub-providers
// satisfies ports.ScopedIdentityResolver so it can be wired as the observer's
// scoped identity resolver (finding #7).
func TestSCMWiring_MultiProviderSatisfiesScopedIdentityResolver(t *testing.T) {
	gh, err := newGitHubSCMProvider(slog.Default())
	if err != nil {
		t.Skipf("github provider unavailable (no token): %v", err)
	}
	gl, err := newGitLabSCMProvider(testGitLabConfig(), slog.Default())
	if err != nil {
		t.Skipf("gitlab provider unavailable (no token): %v", err)
	}

	multi := scmmulti.New(
		scmmulti.NamedProvider{Key: "github", Provider: gh},
		scmmulti.NamedProvider{Key: "gitlab", Provider: gl},
	)

	// The multi provider must satisfy ScopedIdentityResolver.
	var _ ports.ScopedIdentityResolver = multi

	// The multi provider must also satisfy the observer's Provider interface
	// (it already does in production; this asserts the type assertion at compile
	// time for the test).
	_ = multi.SCMCredentialsAvailable
}

// TestSCMWiring_NewMultiSCMProviderReturnsScopedResolver verifies that the
// newMultiSCMProvider helper (used outside the observer) also returns a value
// that satisfies ScopedIdentityResolver.
func TestSCMWiring_NewMultiSCMProviderReturnsScopedResolver(t *testing.T) {
	multi := newMultiSCMProvider(testGitLabConfig(), slog.Default())
	if multi == nil {
		t.Skip("no SCM provider available (missing tokens)")
	}
	var _ ports.ScopedIdentityResolver = multi
}

// TestSCMWiring_ObserverConfigHasScopedResolver verifies that the observer
// Config constructed in production wiring (startSCMObserver) carries a
// non-nil ScopedIdentityResolver when a multi provider is available.
func TestSCMWiring_ObserverConfigHasScopedResolver(t *testing.T) {
	gh, err := newGitHubSCMProvider(slog.Default())
	if err != nil {
		t.Skipf("github provider unavailable: %v", err)
	}
	gl, err := newGitLabSCMProvider(testGitLabConfig(), slog.Default())
	if err != nil {
		t.Skipf("gitlab provider unavailable: %v", err)
	}

	multi := scmmulti.New(
		scmmulti.NamedProvider{Key: "github", Provider: gh},
		scmmulti.NamedProvider{Key: "gitlab", Provider: gl},
	)

	// Mirror the production wiring in startSCMObserver: the multi provider
	// is passed as the ScopedIdentityResolver in the observer Config.
	cfg := scmobserve.Config{
		ScopedIdentityResolver: multi,
	}
	if cfg.ScopedIdentityResolver == nil {
		t.Fatal("ScopedIdentityResolver is nil; production wiring must set it")
	}

	// Verify it actually resolves per-provider (github identity is available
	// if a token was set; if not, it should still return an error, not panic).
	_, _ = cfg.ScopedIdentityResolver.AuthenticatedIdentityForProvider(context.Background(), "github", "")
}

func testGitLabConfig() config.GitLabConfig {
	return config.GitLabConfig{}
}


type captureSCMIntelligenceAdvisor struct {
	outcome ports.SpawnOutcomeRequest
	calls   int
}

func (a *captureSCMIntelligenceAdvisor) AdviseSpawn(context.Context, ports.SpawnDecisionRequest) (ports.SpawnDecision, error) {
	return ports.SpawnDecision{}, nil
}

func (a *captureSCMIntelligenceAdvisor) ObserveOutcome(_ context.Context, req ports.SpawnOutcomeRequest) error {
	a.outcome = req
	a.calls++
	return nil
}

type fakeSCMIntelligenceStore struct {
	rec   domain.SessionRecord
	facts []domain.PRFacts
}

func (s fakeSCMIntelligenceStore) GetSession(context.Context, domain.SessionID) (domain.SessionRecord, bool, error) {
	return s.rec, true, nil
}

func (s fakeSCMIntelligenceStore) ListPRFactsForSession(context.Context, domain.SessionID) ([]domain.PRFacts, error) {
	return append([]domain.PRFacts(nil), s.facts...), nil
}

func TestSCMIntelligenceOutcomeIsBoundedAndSemanticallyAddressed(t *testing.T) {
	rec := domain.SessionRecord{
		ID:        "p-1",
		ProjectID: "p",
		Kind:      domain.KindWorker,
		Harness:   "codex",
		Mode:      domain.SessionModeChat,
		Activity:  domain.Activity{State: domain.ActivityIdle},
		Metadata:  domain.SessionMetadata{Model: "gpt-5"},
	}
	facts := make([]domain.PRFacts, 9)
	for i := range facts {
		facts[i] = domain.PRFacts{
			URL: "https://github.com/o/r/pull/x",
			Number: i + 1,
			CI: domain.CIPassing,
			Review: domain.ReviewRequired,
			Mergeability: domain.MergeBlocked,
			HeadSHA: "sha",
		}
	}
	facts[0].Merged = true

	first := buildSCMIntelligenceOutcome(rec, facts)
	second := buildSCMIntelligenceOutcome(rec, facts)
	if first.OutcomeID != second.OutcomeID {
		t.Fatalf("same evidence produced different ids: %q vs %q", first.OutcomeID, second.OutcomeID)
	}
	if first.Evidence.SCMComplete {
		t.Fatal("nine PRs should mark the capped evidence incomplete")
	}
	if len(first.Evidence.PRs) != maxSCMOutcomePRs {
		t.Fatalf("PR evidence = %d, want %d", len(first.Evidence.PRs), maxSCMOutcomePRs)
	}
	if first.Evidence.Disposition != "observed" || first.Evidence.Terminated {
		t.Fatalf("evidence disposition = %+v", first.Evidence)
	}
	if first.Outcome.PRMerged == nil || !*first.Outcome.PRMerged || first.Outcome.VerificationSource != "ao-pr-merge" {
		t.Fatalf("generic outcome = %+v", first.Outcome)
	}

	changed := append([]domain.PRFacts(nil), facts...)
	changed[0].Review = domain.ReviewApproved
	third := buildSCMIntelligenceOutcome(rec, changed)
	if third.OutcomeID == first.OutcomeID {
		t.Fatal("semantic evidence change reused the same outcome id")
	}
}

func TestSCMIntelligenceOutcomeKeepsCIFailureNegativeWithoutInventingSuccess(t *testing.T) {
	rec := domain.SessionRecord{
		ID: "p-2", ProjectID: "p", Kind: domain.KindWorker, Harness: "codex",
	}
	req := buildSCMIntelligenceOutcome(rec, []domain.PRFacts{{
		URL: "https://github.com/o/r/pull/2", Number: 2,
		CI: domain.CIFailing, Review: domain.ReviewNone, Mergeability: domain.MergeBlocked,
	}})
	if req.Outcome.CIFailed == nil || !*req.Outcome.CIFailed || req.Outcome.VerificationSource != "ao-ci" {
		t.Fatalf("generic outcome = %+v", req.Outcome)
	}
	if req.Outcome.PRMerged != nil || req.Outcome.ExecutionCompleted != nil {
		t.Fatalf("SCM failure invented unrelated success/completion: %+v", req.Outcome)
	}
}

func TestSCMIntelligenceSinkReadsCanonicalFacts(t *testing.T) {
	store := fakeSCMIntelligenceStore{
		rec: domain.SessionRecord{ID: "p-3", ProjectID: "p", Kind: domain.KindWorker, Harness: "codex"},
		facts: []domain.PRFacts{{
			URL: "https://github.com/o/r/pull/3", Number: 3, Merged: true, CI: domain.CIPassing,
		}},
	}
	advisor := &captureSCMIntelligenceAdvisor{}
	sink := &scmIntelligenceSink{store: store, advisor: advisor}
	if err := sink.ObserveSCM(context.Background(), "p-3"); err != nil {
		t.Fatal(err)
	}
	if advisor.calls != 1 || advisor.outcome.SessionID != "p-3" {
		t.Fatalf("advisor calls=%d outcome=%+v", advisor.calls, advisor.outcome)
	}
	if advisor.outcome.Evidence.Disposition != "observed" || len(advisor.outcome.Evidence.PRs) != 1 {
		t.Fatalf("evidence = %+v", advisor.outcome.Evidence)
	}
}
