package session

import (
	"context"
	"errors"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	sessionmanager "github.com/aoagents/agent-orchestrator/backend/internal/session_manager"
)

type singletonRestoreCommander struct {
	*fakeCommander
	calls []domain.SessionID
}

func (f *singletonRestoreCommander) RestoreWithMode(
	_ context.Context,
	id domain.SessionID,
) (sessionmanager.RestoreResult, error) {
	f.calls = append(f.calls, id)
	return sessionmanager.RestoreResult{}, errors.New("restore crossed singleton guard")
}

func TestRestoreRejectsHistoricalOrchestratorWhileSiblingIsLive(t *testing.T) {
	store := newFakeStore()
	store.sessions["project-old"] = domain.SessionRecord{
		ID:           "project-old",
		ProjectID:    "project",
		Kind:         domain.KindOrchestrator,
		IsTerminated: true,
		Activity:     domain.Activity{State: domain.ActivityExited},
	}
	store.sessions["project-live"] = domain.SessionRecord{
		ID:        "project-live",
		ProjectID: "project",
		Kind:      domain.KindOrchestrator,
		Activity:  domain.Activity{State: domain.ActivityIdle},
	}
	manager := &singletonRestoreCommander{fakeCommander: &fakeCommander{}}
	service := &Service{manager: manager, store: store}

	_, err := service.Restore(context.Background(), "project-old")
	var apiError *apierr.Error
	if !errors.As(err, &apiError) || apiError.Kind != apierr.KindConflict || apiError.Code != "ORCHESTRATOR_ALREADY_ACTIVE" {
		t.Fatalf("Restore error = %v, want conflict ORCHESTRATOR_ALREADY_ACTIVE", err)
	}
	if len(manager.calls) != 0 {
		t.Fatalf("manager restore calls = %v, want none before ownership is resolved", manager.calls)
	}
	if !store.sessions["project-old"].IsTerminated {
		t.Fatal("historical orchestrator changed despite singleton conflict")
	}
	if store.sessions["project-live"].IsTerminated {
		t.Fatal("live orchestrator changed despite singleton conflict")
	}
}
