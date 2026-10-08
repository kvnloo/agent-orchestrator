package sessionmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestDefaultChatSpawnFallsBackToTUIForUnsupportedPermissionMode(t *testing.T) {
	launcher := &recordingLauncher{preflightErr: ports.ErrChatPermissionModeUnsupported}
	manager, store, runtime := newChatManager(launcher)
	manager.defaults = fixedSessionModeDefaults(domain.SessionModeChat)

	record, _, _, err := manager.Spawn(context.Background(), ports.SpawnConfig{
		ProjectID: chatTestProject,
		Kind:      domain.KindWorker,
		Harness:   domain.HarnessKimi,
	})
	if err != nil {
		t.Fatalf("default Chat spawn: %v", err)
	}
	if record.Mode != domain.SessionModeTUI {
		t.Fatalf("mode = %q, want TUI fallback", record.Mode)
	}
	if len(launcher.preflighted) != 1 || launcher.preflighted[0] != domain.HarnessKimi {
		t.Fatalf("preflighted harnesses = %v, want Kimi once", launcher.preflighted)
	}
	if len(launcher.started) != 0 {
		t.Fatalf("Chat controllers started = %d, want none", len(launcher.started))
	}
	if runtime.created != 1 {
		t.Fatalf("terminal runtimes created = %d, want one", runtime.created)
	}
	if stored := store.sessions[record.ID]; stored.Mode != domain.SessionModeTUI || stored.IsTerminated {
		t.Fatalf("stored fallback session = %+v", stored)
	}
}

func TestExplicitChatSpawnDoesNotFallbackForUnsupportedPermissionMode(t *testing.T) {
	launcher := &recordingLauncher{preflightErr: ports.ErrChatPermissionModeUnsupported}
	manager, store, runtime := newChatManager(launcher)

	_, _, _, err := manager.Spawn(context.Background(), ports.SpawnConfig{
		ProjectID:     chatTestProject,
		Kind:          domain.KindWorker,
		Harness:       domain.HarnessKimi,
		RequestedMode: domain.SessionModeChat,
	})
	if !errors.Is(err, ports.ErrChatPermissionModeUnsupported) {
		t.Fatalf("error = %v, want ErrChatPermissionModeUnsupported", err)
	}
	if runtime.created != 0 {
		t.Fatalf("explicit Chat request created %d terminal runtimes, want none", runtime.created)
	}
	sessions, listErr := store.ListAllSessions(context.Background())
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(sessions) != 0 {
		t.Fatalf("explicit rejected Chat request left %d session rows", len(sessions))
	}
}
