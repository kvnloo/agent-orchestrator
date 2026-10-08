package sessionmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

type retirementChatLauncher struct {
	*recordingLauncher
	stopErr error
}

func (l *retirementChatLauncher) StopChat(_ context.Context, id domain.SessionID) error {
	l.stopped = append(l.stopped, id)
	return l.stopErr
}

func chatOrchestratorForReplacement(id domain.SessionID) domain.SessionRecord {
	return domain.SessionRecord{
		ID:        id,
		ProjectID: chatTestProject,
		Kind:      domain.KindOrchestrator,
		Harness:   domain.HarnessCodex,
		Mode:      domain.SessionModeChat,
		Activity:  domain.Activity{State: domain.ActivityActive},
		Metadata: domain.SessionMetadata{
			ControllerGeneration: "generation-1",
		},
	}
}

func TestRetireForReplacementStopsChatController(t *testing.T) {
	launcher := &retirementChatLauncher{recordingLauncher: &recordingLauncher{live: true}}
	manager, store, _ := newChatManager(launcher)
	id := domain.SessionID("mer-orchestrator")
	store.sessions[id] = chatOrchestratorForReplacement(id)

	if err := manager.RetireForReplacement(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(launcher.stopped) != 1 || launcher.stopped[0] != id {
		t.Fatalf("stopped chat controllers = %v, want [%s]", launcher.stopped, id)
	}
	if rec := store.sessions[id]; !rec.IsTerminated {
		t.Fatalf("retired session was not terminated: %+v", rec)
	}
}

func TestRetireForReplacementDoesNotTerminateWhenChatControllerCannotStop(t *testing.T) {
	stopErr := errors.New("provider controller did not stop")
	launcher := &retirementChatLauncher{
		recordingLauncher: &recordingLauncher{live: true},
		stopErr:           stopErr,
	}
	manager, store, _ := newChatManager(launcher)
	id := domain.SessionID("mer-orchestrator")
	store.sessions[id] = chatOrchestratorForReplacement(id)

	err := manager.RetireForReplacement(context.Background(), id)
	if !errors.Is(err, stopErr) {
		t.Fatalf("error = %v, want chat stop error", err)
	}
	if len(launcher.stopped) != 1 || launcher.stopped[0] != id {
		t.Fatalf("stopped chat controllers = %v, want one attempted stop", launcher.stopped)
	}
	if rec := store.sessions[id]; rec.IsTerminated {
		t.Fatalf("replacement marked a still-live chat session terminated: %+v", rec)
	}
}
