package lifecycle

import (
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func providerMergeabilityObservation(state domain.Mergeability, providerMergeable, providerMergeState string) ports.SCMObservation {
	return ports.SCMObservation{
		Fetched: true,
		PR: ports.SCMPRObservation{
			URL:                      "pr1",
			ProviderMergeable:        providerMergeable,
			ProviderMergeStateStatus: providerMergeState,
		},
		Mergeability: ports.SCMMergeabilityObservation{State: string(state)},
	}
}

func TestSCMObservation_UnknownProviderMergeabilityDoesNotRearmBlockedConflict(t *testing.T) {
	m, st, msg := newManager()
	st.sessions["mer-1"] = working("mer-1")

	for _, obs := range []ports.SCMObservation{
		providerMergeabilityObservation(domain.MergeConflicting, "CONFLICTING", "DIRTY"),
		providerMergeabilityObservation(domain.MergeBlocked, "UNKNOWN", "BLOCKED"),
		providerMergeabilityObservation(domain.MergeConflicting, "CONFLICTING", "DIRTY"),
	} {
		if err := m.ApplySCMObservation(ctx, "mer-1", obs); err != nil {
			t.Fatal(err)
		}
	}
	if len(msg.msgs) != 1 {
		t.Fatalf("unknown provider mergeability must keep an unchanged conflict deduplicated, got %d nudges: %v", len(msg.msgs), msg.msgs)
	}
}

func TestSCMObservation_ContradictoryDirtyProviderStateDoesNotRearmConflict(t *testing.T) {
	m, st, msg := newManager()
	st.sessions["mer-1"] = working("mer-1")

	for _, obs := range []ports.SCMObservation{
		providerMergeabilityObservation(domain.MergeConflicting, "CONFLICTING", "DIRTY"),
		providerMergeabilityObservation(domain.MergeBlocked, "MERGEABLE", "DIRTY"),
		providerMergeabilityObservation(domain.MergeConflicting, "CONFLICTING", "DIRTY"),
	} {
		if err := m.ApplySCMObservation(ctx, "mer-1", obs); err != nil {
			t.Fatal(err)
		}
	}
	if len(msg.msgs) != 1 {
		t.Fatalf("a contradictory DIRTY provider state must not re-arm, got %d nudges: %v", len(msg.msgs), msg.msgs)
	}
}

func TestSCMObservation_ProviderMergeableBlockedRearmSurvivesManagerRestart(t *testing.T) {
	m, st, msg := newManager()
	st.sessions["mer-1"] = working("mer-1")

	if err := m.ApplySCMObservation(ctx, "mer-1", providerMergeabilityObservation(domain.MergeConflicting, "CONFLICTING", "DIRTY")); err != nil {
		t.Fatal(err)
	}
	if len(msg.msgs) != 1 {
		t.Fatalf("first conflict should nudge once, got %v", msg.msgs)
	}

	// A fresh manager proves the re-arm is loaded from and written back to the
	// persisted reaction payload rather than succeeding only in memory.
	m = New(st, msg)
	if err := m.ApplySCMObservation(ctx, "mer-1", providerMergeabilityObservation(domain.MergeBlocked, "MERGEABLE", "BLOCKED")); err != nil {
		t.Fatal(err)
	}
	m = New(st, msg)
	if err := m.ApplySCMObservation(ctx, "mer-1", providerMergeabilityObservation(domain.MergeConflicting, "CONFLICTING", "DIRTY")); err != nil {
		t.Fatal(err)
	}
	if len(msg.msgs) != 2 {
		t.Fatalf("persisted provider-confirmed re-arm should deliver the later conflict, got %d nudges: %v", len(msg.msgs), msg.msgs)
	}
}
