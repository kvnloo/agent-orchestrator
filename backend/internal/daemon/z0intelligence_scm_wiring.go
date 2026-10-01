package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	z0intelligence "github.com/aoagents/agent-orchestrator/backend/internal/adapters/intelligence/z0intelligence"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

const (
	scmIntelligenceTimeout = 300 * time.Millisecond
	maxSCMOutcomePRs       = 8
)

type scmIntelligenceStore interface {
	GetSession(context.Context, domain.SessionID) (domain.SessionRecord, bool, error)
	ListPRFactsForSession(context.Context, domain.SessionID) ([]domain.PRFacts, error)
}

type scmIntelligenceSink struct {
	store   scmIntelligenceStore
	advisor ports.IntelligenceAdvisor
}

func newSCMIntelligenceSink(store scmIntelligenceStore, logger *slog.Logger) *scmIntelligenceSink {
	raw := strings.TrimSpace(os.Getenv("AO_Z0INTELLIGENCE_SHADOW_URL"))
	if raw == "" || store == nil {
		return nil
	}
	client, err := z0intelligence.New(raw, 250*time.Millisecond)
	if err != nil {
		logger.Warn("z0intelligence SCM evidence disabled", "error", err)
		return nil
	}
	return &scmIntelligenceSink{store: store, advisor: client}
}

func (s *scmIntelligenceSink) ObserveSCM(ctx context.Context, id domain.SessionID) error {
	if s == nil || s.store == nil || s.advisor == nil {
		return nil
	}
	evidenceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scmIntelligenceTimeout)
	defer cancel()

	rec, ok, err := s.store.GetSession(evidenceCtx, id)
	if err != nil {
		return fmt.Errorf("read session %s for intelligence evidence: %w", id, err)
	}
	if !ok {
		return nil
	}
	facts, err := s.store.ListPRFactsForSession(evidenceCtx, id)
	if err != nil {
		return fmt.Errorf("read PR facts for intelligence evidence %s: %w", id, err)
	}
	req := buildSCMIntelligenceOutcome(rec, facts)
	return s.advisor.ObserveOutcome(evidenceCtx, req)
}

func buildSCMIntelligenceOutcome(rec domain.SessionRecord, facts []domain.PRFacts) ports.SpawnOutcomeRequest {
	scmComplete := len(facts) <= maxSCMOutcomePRs
	if len(facts) > maxSCMOutcomePRs {
		facts = facts[:maxSCMOutcomePRs]
	}

	prs := make([]ports.SpawnOutcomePR, 0, len(facts))
	anyMerged := false
	anyCIFailed := false
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
		anyMerged = anyMerged || pr.Merged
		anyCIFailed = anyCIFailed || pr.CI == domain.CIFailing
	}

	outcome := ports.SpawnOutcome{Source: "agent-orchestrator"}
	if anyMerged {
		value := true
		outcome.PRMerged = &value
		outcome.VerificationSource = "ao-pr-merge"
	} else if anyCIFailed {
		value := true
		outcome.CIFailed = &value
		outcome.VerificationSource = "ao-ci"
	}

	evidence := ports.SpawnOutcomeEvidence{
		ProjectID:   string(rec.ProjectID),
		Kind:        string(rec.Kind),
		Harness:     string(rec.Harness),
		Mode:        string(domain.NormalizeSessionMode(rec.Mode)),
		Model:       rec.Metadata.Model,
		Activity:    string(rec.Activity.State),
		Disposition: "observed",
		Terminated:  rec.IsTerminated,
		SCMComplete: scmComplete,
		PRs:         prs,
	}

	return ports.SpawnOutcomeRequest{
		Schema:    ports.SpawnOutcomeSchema,
		TraceID:   "ao-spawn-" + string(rec.ID),
		OutcomeID: scmOutcomeID(rec.ID, evidence),
		SessionID: string(rec.ID),
		Outcome:   outcome,
		Evidence:  evidence,
	}
}

func scmOutcomeID(id domain.SessionID, evidence ports.SpawnOutcomeEvidence) string {
	body, err := json.Marshal(evidence)
	if err != nil {
		// The evidence shape is composed only of JSON-native primitives, so this
		// is defensive. Preserve deterministic identity if that invariant ever
		// changes rather than introducing a random replay key.
		body = []byte(fmt.Sprintf("%#v", evidence))
	}
	sum := sha256.Sum256(body)
	return "ao-outcome-" + string(id) + "-scm-" + hex.EncodeToString(sum[:8])
}
