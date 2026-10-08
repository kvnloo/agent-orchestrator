package store

import (
	"context"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestChangeLogRetentionMaxRowsIsASequenceWindowNotACount(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedProject(t, store, "mer")

	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-8 * 24 * time.Hour)
	record, err := store.CreateSession(ctx, domain.SessionRecord{
		ProjectID: "mer",
		Kind:      domain.KindWorker,
		Activity:  domain.Activity{State: domain.ActivityActive, LastActivityAt: now},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	// seq 1 is create; seq 2..5 are updates. Stamp the second update old so
	// age pruning punches a hole at seq 3 before the size cap runs.
	for index, at := range []time.Time{
		now.Add(time.Second), old, now.Add(3 * time.Second), now.Add(4 * time.Second),
	} {
		if index%2 == 0 {
			record.Activity.State = domain.ActivityIdle
		} else {
			record.Activity.State = domain.ActivityActive
		}
		record.UpdatedAt = at
		if err := store.UpdateSession(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	if removed, err := store.PruneChangeLogBefore(ctx, now.Add(-7*24*time.Hour), 100); err != nil || removed != 1 {
		t.Fatalf("age prune removed = %d, err = %v, want seq 3 only", removed, err)
	}

	// Rows left are seq 1,2,4,5. A sequence replay window of three keeps seq
	// 3..5, so seq 1 and 2 are removed despite only four physical rows existing.
	removed, err := store.PruneChangeLogToMaxRows(ctx, 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want seq 1 and 2", removed)
	}
	events, err := store.EventsAfter(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Seq != 4 || events[1].Seq != 5 {
		t.Fatalf("remaining events = %#v, want seq 4 and 5", events)
	}
}
