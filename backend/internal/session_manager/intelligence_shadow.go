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


type intelligencePRFactReader interface {
	ListPRFactsForSession(context.Context, domain.SessionID) ([]domain.PRFacts, error)
}

// observeTerminalOutcome projects AO's canonical terminal + SCM facts back to
// the optional intelligence plane. This path is evidence-only: a response
// cannot mutate AO state, and any failure is logged and ignored.
func (m *Manager) observeTerminalOutcome(ctx context.Context, id domain.SessionID) {
	if m.intelligence == nil || m.store == nil {
		return
	}
	outcomeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), intelligenceShadowTimeout)
	defer cancel()

	rec, ok, err := m.store.GetSession(outcomeCtx, id)
	if err != nil {
		m.logger.Warn("z0intelligence outcome session read failed; continuing unchanged",
			"sessionID", id, "error", err)
		return
	}
	if !ok || !rec.IsTerminated {
		return
	}

	prs := make([]ports.SpawnOutcomePR, 0)
	scmComplete := false
	if reader, ok := m.store.(intelligencePRFactReader); ok {
		facts, readErr := reader.ListPRFactsForSession(outcomeCtx, id)
		if readErr != nil {
			m.logger.Warn("z0intelligence outcome SCM read failed; reporting incomplete evidence",
				"sessionID", id, "error", readErr)
		} else {
			scmComplete = true
			prs = make([]ports.SpawnOutcomePR, 0, len(facts))
			for _, pr := range facts {
				prs = append(prs, ports.SpawnOutcomePR{
					URL:                      pr.URL,
					Number:                   pr.Number,
					Draft:                    pr.Draft,
					Merged:                   pr.Merged,
					Closed:                   pr.Closed,
					CI:                       string(pr.CI),
					Review:                   string(pr.Review),
					Mergeability:             string(pr.Mergeability),
					ReviewComments:           pr.ReviewComments,
					ExternalApproved:         pr.ExternalApproved,
					ExternalChangesRequested: pr.ExternalChangesRequested,
					ExternalComments:         pr.ExternalComments,
					HeadSHA:                  pr.HeadSHA,
				})
			}
		}
	}

	req := ports.SpawnOutcomeRequest{
		Schema:      ports.SpawnOutcomeSchema,
		TraceID:     "ao-spawn-" + string(rec.ID),
		OutcomeID:   "ao-outcome-" + string(rec.ID) + "-terminated",
		SessionID:   string(rec.ID),
		ProjectID:   string(rec.ProjectID),
		Kind:        string(rec.Kind),
		Harness:     string(rec.Harness),
		Mode:        string(domain.NormalizeSessionMode(rec.Mode)),
		Model:       rec.Metadata.Model,
		Activity:    string(rec.Activity.State),
		Terminated:  true,
		SCMComplete: scmComplete,
		PRs:         prs,
	}
	if err := m.intelligence.ObserveOutcome(outcomeCtx, req); err != nil {
		m.logger.Warn("z0intelligence outcome unavailable; continuing unchanged",
			"sessionID", rec.ID, "outcomeID", req.OutcomeID, "error", err)
		return
	}
	m.logger.Info("z0intelligence terminal outcome reported",
		"sessionID", rec.ID,
		"outcomeID", req.OutcomeID,
		"scmComplete", scmComplete,
		"prCount", len(prs),
	)
}
