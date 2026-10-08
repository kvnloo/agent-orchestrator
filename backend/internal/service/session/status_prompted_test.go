package session

import (
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestContractSessionFactsPromptedSinceCurrentSpawn(t *testing.T) {
	spawned := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	rec := domain.SessionRecord{
		Mode: domain.SessionModeTUI,
		Activity: domain.Activity{
			State:          domain.ActivityIdle,
			LastActivityAt: spawned,
		},
	}

	rec.Metadata.LatestUserPromptAt = spawned.Add(-time.Hour)
	if facts := toContractSessionFacts(rec, true); facts.PromptedSinceSpawn {
		t.Fatalf("prompt from an earlier controller lifetime counted as current: %+v", facts)
	}

	rec.Metadata.LatestUserPromptAt = spawned.Add(time.Second)
	if facts := toContractSessionFacts(rec, true); !facts.PromptedSinceSpawn {
		t.Fatalf("prompt after current spawn was not projected: %+v", facts)
	}
}
