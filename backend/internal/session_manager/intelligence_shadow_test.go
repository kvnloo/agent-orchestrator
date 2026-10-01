package sessionmanager

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

type captureIntelligenceAdvisor struct {
	req          ports.SpawnDecisionRequest
	outcome      ports.SpawnOutcomeRequest
	outcomeCalls int
}

func (a *captureIntelligenceAdvisor) AdviseSpawn(_ context.Context, req ports.SpawnDecisionRequest) (ports.SpawnDecision, error) {
	a.req = req
	return ports.SpawnDecision{
		Schema:         ports.SpawnDecisionSchema,
		DecisionID:     req.TraceID,
		Action:         "abstain",
		Recommendation: req.Current,
		PolicyRevision: "test-v1",
		ReceiptID:      req.TraceID,
	}, nil
}

func (a *captureIntelligenceAdvisor) ObserveOutcome(_ context.Context, req ports.SpawnOutcomeRequest) error {
	a.outcome = req
	a.outcomeCalls++
	return nil
}

type outcomeIntelligenceStore struct {
	*fakeStore
	prs map[domain.SessionID][]domain.PRFacts
}

func (s *outcomeIntelligenceStore) ListPRFactsForSession(_ context.Context, id domain.SessionID) ([]domain.PRFacts, error) {
	return append([]domain.PRFacts(nil), s.prs[id]...), nil
}

func TestObserveSpawnDecisionCarriesResolvedChoiceAndExplicitConstraints(t *testing.T) {
	advisor := &captureIntelligenceAdvisor{}
	m := &Manager{
		intelligence: advisor,
		logger:       slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}
	m.observeSpawnDecision(
		context.Background(),
		domain.SessionRecord{ID: "proj-7"},
		ports.SpawnConfig{
			ProjectID: "proj",
			Kind:      domain.KindWorker,
			Harness:   "codex",
			Prompt:    "fix retry race",
		},
		ports.AgentConfig{Model: "gpt-5", Permissions: ports.PermissionModeDefault},
		domain.SessionModeChat,
		false,
		true,
		false,
	)

	if advisor.req.TraceID != "ao-spawn-proj-7" || advisor.req.SessionID != "proj-7" {
		t.Fatalf("identity = %+v", advisor.req)
	}
	if advisor.req.Current.Harness != "codex" || advisor.req.Current.Model != "gpt-5" || advisor.req.Current.Mode != "chat" {
		t.Fatalf("current = %+v", advisor.req.Current)
	}
	if advisor.req.Constraints.ExplicitHarness || !advisor.req.Constraints.ExplicitModel || advisor.req.Constraints.ExplicitMode {
		t.Fatalf("constraints = %+v", advisor.req.Constraints)
	}
}


func TestObserveTerminalOutcomeJoinsLifecycleAndSCMEvidence(t *testing.T) {
	base := newFakeStore()
	base.sessions["proj-7"] = domain.SessionRecord{
		ID:           "proj-7",
		ProjectID:    "proj",
		Kind:         domain.KindWorker,
		Harness:      "codex",
		Mode:         domain.SessionModeTUI,
		Activity:     domain.Activity{State: domain.ActivityIdle},
		IsTerminated: true,
		Metadata:     domain.SessionMetadata{Model: "gpt-5"},
	}
	store := &outcomeIntelligenceStore{
		fakeStore: base,
		prs: map[domain.SessionID][]domain.PRFacts{
			"proj-7": {{
				URL: "https://github.com/example/repo/pull/7", Number: 7,
				Merged: true, CI: domain.CIPassing, Review: domain.ReviewApproved,
				Mergeability: domain.MergeMergeable, ExternalApproved: true,
				HeadSHA: "abc123",
			}},
		},
	}
	advisor := &captureIntelligenceAdvisor{}
	m := &Manager{
		store:        store,
		intelligence: advisor,
		logger:       slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}

	m.observeTerminalOutcome(context.Background(), "proj-7")

	if advisor.outcomeCalls != 1 {
		t.Fatalf("outcome calls = %d, want 1", advisor.outcomeCalls)
	}
	got := advisor.outcome
	if got.Schema != ports.SpawnOutcomeSchema || got.TraceID != "ao-spawn-proj-7" ||
		got.OutcomeID != "ao-outcome-proj-7-terminated" || got.SessionID != "proj-7" {
		t.Fatalf("identity = %+v", got)
	}
	if !got.Terminated || !got.SCMComplete || got.Harness != "codex" || got.Model != "gpt-5" || got.Mode != "tui" {
		t.Fatalf("lifecycle = %+v", got)
	}
	if len(got.PRs) != 1 || !got.PRs[0].Merged || got.PRs[0].CI != "passing" ||
		got.PRs[0].Review != "approved" || !got.PRs[0].ExternalApproved {
		t.Fatalf("prs = %+v", got.PRs)
	}
}

func TestObserveTerminalOutcomeIgnoresLiveSession(t *testing.T) {
	base := newFakeStore()
	base.sessions["proj-8"] = domain.SessionRecord{ID: "proj-8", ProjectID: "proj", Kind: domain.KindWorker}
	advisor := &captureIntelligenceAdvisor{}
	m := &Manager{
		store:        base,
		intelligence: advisor,
		logger:       slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}

	m.observeTerminalOutcome(context.Background(), "proj-8")

	if advisor.outcomeCalls != 0 {
		t.Fatalf("outcome calls = %d, want 0", advisor.outcomeCalls)
	}
}
