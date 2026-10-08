package cdc

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestRetentionJanitorRunOnceClearsSustainedBurstBudget(t *testing.T) {
	// A 15-minute burst at the measured production rate is larger than the old
	// eight-batch ceiling. Keep returning full batches so this test pins the
	// amount of bounded work one janitor run is allowed to perform.
	sizeRows := make([]int64, 70)
	for i := range sizeRows {
		sizeRows[i] = DefaultRetentionBatch
	}
	store := &fakeRetentionStore{sizeRows: sizeRows}
	janitor := NewRetentionJanitor(store, RetentionConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err := janitor.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.sizes != 64 || store.ages != 64 {
		t.Fatalf("prune calls = age %d/size %d, want 64/64 so one run can drain a sustained burst", store.ages, store.sizes)
	}
}
