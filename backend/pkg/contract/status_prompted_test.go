package contract

import (
	"testing"
	"time"
)

func TestDeriveStatusNoSignalRequiresPromptSinceCurrentSpawn(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	facts := SessionFacts{
		Activity:       ActivityIdle,
		LastActivityAt: now.Add(-10 * time.Minute),
		SignalExpected: true,
		HasSignal:      false,
	}

	if got := DeriveStatus(facts, nil, now, 90*time.Second); got != StatusIdle {
		t.Fatalf("never-prompted silent session = %q, want idle", got)
	}

	facts.PromptedSinceSpawn = true
	if got := DeriveStatus(facts, nil, now, 90*time.Second); got != StatusNoSignal {
		t.Fatalf("prompted hook-silent session = %q, want no_signal", got)
	}
}

func TestDeriveStatusNoSignalStillYieldsToRealActivity(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	facts := SessionFacts{
		Activity:          ActivityActive,
		LastActivityAt:    now.Add(-10 * time.Minute),
		SignalExpected:    true,
		HasSignal:         false,
		PromptedSinceSpawn: true,
	}
	if got := DeriveStatus(facts, nil, now, 90*time.Second); got != StatusWorking {
		t.Fatalf("active session = %q, want working", got)
	}
}
