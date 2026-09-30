package sessionmanager

import (
	"context"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

const intelligenceShadowTimeout = 300 * time.Millisecond

// observeSpawnDecision is intentionally post-seed in P0. The durable AO
// session id gives the shadow receipt a stable idempotency identity; moving
// policy authority before seed creation requires a first-class spawn request
// idempotency key rather than inventing one from task content.
func (m *Manager) observeSpawnDecision(
	ctx context.Context,
	rec domain.SessionRecord,
	cfg ports.SpawnConfig,
	agentConfig ports.AgentConfig,
	mode domain.SessionMode,
	explicitHarness bool,
	explicitModel bool,
	explicitMode bool,
) {
	if m.intelligence == nil {
		return
	}
	decisionCtx, cancel := context.WithTimeout(ctx, intelligenceShadowTimeout)
	defer cancel()

	req := ports.SpawnDecisionRequest{
		Schema:    ports.SpawnDecisionSchema,
		TraceID:   "ao-spawn-" + string(rec.ID),
		SessionID: string(rec.ID),
		ProjectID: string(cfg.ProjectID),
		Kind:      string(cfg.Kind),
		Task:      cfg.Prompt,
		Current: ports.SpawnDecisionCurrent{
			Harness:    string(cfg.Harness),
			Model:      agentConfig.Model,
			Mode:       string(mode),
			Permission: string(agentConfig.Permissions),
		},
		Constraints: ports.SpawnDecisionConstraints{
			ExplicitHarness: explicitHarness,
			ExplicitModel:   explicitModel,
			ExplicitMode:    explicitMode,
		},
	}
	decision, err := m.intelligence.AdviseSpawn(decisionCtx, req)
	if err != nil {
		m.logger.Warn("z0intelligence shadow decision unavailable; continuing unchanged",
			"sessionID", rec.ID, "error", err)
		return
	}
	m.logger.Info("z0intelligence shadow spawn decision",
		"sessionID", rec.ID,
		"decisionID", decision.DecisionID,
		"action", decision.Action,
		"policyRevision", decision.PolicyRevision,
		"replayed", decision.Replayed,
	)
}
