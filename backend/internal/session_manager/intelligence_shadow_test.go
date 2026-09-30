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
	req ports.SpawnDecisionRequest
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
