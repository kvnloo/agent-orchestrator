package report

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// orderedBatchStore keeps the coordinator test fake aligned with the real
// store: ClaimPendingReportBatch selects the earliest scheduled batch for the
// project, not the first row's slice position.
type orderedBatchStore struct {
	*coordinatorStore
}

func (s *orderedBatchStore) ClaimPendingReportBatch(
	_ context.Context,
	project domain.ProjectID,
	token string,
	at time.Time,
) ([]domain.ReportRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pending []domain.ReportRecord
	for _, rec := range s.reports {
		if rec.ProjectID == project && rec.DeliveryState == domain.ReportPending {
			pending = append(pending, rec)
		}
	}
	sortReports(pending)
	if len(pending) == 0 {
		return nil, nil
	}

	batchID := pending[0].DeliveryBatchID
	if batchID == "" {
		batchID = "report-batch:" + pending[0].ID
		for i := range s.reports {
			if s.reports[i].ProjectID == project &&
				s.reports[i].DeliveryState == domain.ReportPending &&
				s.reports[i].DeliveryBatchID == "" {
				s.reports[i].DeliveryBatchID = batchID
			}
		}
	}

	var claimed []domain.ReportRecord
	for i := range s.reports {
		if s.reports[i].ProjectID != project ||
			s.reports[i].DeliveryState != domain.ReportPending ||
			s.reports[i].DeliveryBatchID != batchID {
			continue
		}
		s.reports[i].DeliveryState = domain.ReportClaimed
		s.reports[i].ClaimToken = token
		s.reports[i].ClaimedAt = at
		s.reports[i].DeliveryAttempts++
		claimed = append(claimed, s.reports[i])
	}
	sortReports(claimed)
	return claimed, nil
}

type selectiveReportDelivery struct {
	failText string
	err      error
	attempts []string
}

func (d *selectiveReportDelivery) Submit(
	_ context.Context,
	_ domain.SessionID,
	message string,
	_ string,
) error {
	d.attempts = append(d.attempts, message)
	if strings.Contains(message, d.failText) {
		return d.err
	}
	return nil
}

func (*selectiveReportDelivery) Interrupt(context.Context, domain.SessionID) error { return nil }

func TestCoordinatorFailureDoesNotStarveNewerReportBatch(t *testing.T) {
	now := time.Date(2026, 10, 1, 8, 45, 0, 0, time.UTC)
	retryDelay := 5 * time.Second
	base := newCoordinatorStore(now)
	store := &orderedBatchStore{coordinatorStore: base}
	store.sessions = []domain.SessionRecord{{
		ID: "orch", ProjectID: "p", Kind: domain.KindOrchestrator, Mode: domain.SessionModeChat,
	}}

	blocked := reportAt("blocked", "worker-old", now.Add(-time.Minute), now, "permanently unsupported")
	blocked.DeliveryBatchID = "batch-blocked"
	newer := reportAt("newer", "worker-new", now, now, "new report must be attempted")
	newer.DeliveryBatchID = "batch-newer"
	store.add(blocked, newer)

	wantErr := errors.New("semantic message acceptance unsupported")
	delivery := &selectiveReportDelivery{failText: "permanently unsupported", err: wantErr}
	coordinator := NewCoordinator(CoordinatorDeps{
		Store: store, Delivery: delivery, Now: func() time.Time { return now },
		RetryDelay: retryDelay,
		NewToken: func() string { return "claim-token" },
	})

	delay, err := coordinator.RunDue(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunDue error = %v, want original delivery error", err)
	}
	if delay != retryDelay {
		t.Fatalf("delay = %v, want %v", delay, retryDelay)
	}
	if len(delivery.attempts) != 2 {
		t.Fatalf("delivery attempts = %d, want blocked batch and newer batch in the same coordinator cycle: %#v", len(delivery.attempts), delivery.attempts)
	}
	if !strings.Contains(delivery.attempts[0], "permanently unsupported") ||
		!strings.Contains(delivery.attempts[1], "new report must be attempted") {
		t.Fatalf("delivery order = %#v", delivery.attempts)
	}
	if store.acked != 1 {
		t.Fatalf("acknowledged reports = %d, want newer batch acknowledged", store.acked)
	}

	var blockedAfter, newerAfter domain.ReportRecord
	for _, report := range store.reports {
		switch report.ID {
		case "blocked":
			blockedAfter = report
		case "newer":
			newerAfter = report
		}
	}
	if blockedAfter.DeliveryState != domain.ReportPending || !blockedAfter.AvailableAt.Equal(now.Add(retryDelay)) {
		t.Fatalf("blocked batch = %+v, want pending with bounded retry backoff", blockedAfter)
	}
	if newerAfter.DeliveryState != domain.ReportAcknowledged {
		t.Fatalf("newer batch = %+v, want acknowledged", newerAfter)
	}
}
