package sessionmanager

import (
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestRestoreAllRestoresAtMostOneSavedOrchestratorPerProject(t *testing.T) {
	manager, store, runtime, _ := newLifecycleManager()
	now := time.Date(2026, time.August, 28, 9, 0, 0, 0, time.UTC)

	for index, id := range []domain.SessionID{"project-old", "project-new"} {
		store.sessions[id] = domain.SessionRecord{
			ID:           id,
			ProjectID:    "project",
			Kind:         domain.KindOrchestrator,
			Harness:      domain.HarnessClaudeCode,
			Mode:         domain.SessionModeTUI,
			IsTerminated: true,
			Metadata: domain.SessionMetadata{
				WorkspacePath:  "/ws/" + string(id),
				Branch:         "ao/project-orchestrator",
				AgentSessionID: "native-" + string(id),
			},
			Activity:  domain.Activity{State: domain.ActivityExited},
			CreatedAt: now.Add(time.Duration(index) * time.Minute),
			UpdatedAt: now.Add(time.Duration(index) * time.Minute),
		}
		store.worktrees[id] = []domain.SessionWorktreeRecord{
			{
				SessionID:    id,
				RepoName:     domain.RootWorkspaceRepoName,
				Branch:       "ao/project-orchestrator",
				WorktreePath: "/ws/" + string(id),
				State:        "removed",
			},
		}
	}

	if err := manager.RestoreAll(ctx); err != nil {
		t.Fatalf("RestoreAll error = %v", err)
	}

	if runtime.created != 1 {
		t.Fatalf("runtime.Create calls = %d, want exactly one project orchestrator", runtime.created)
	}
	live := 0
	marked := 0
	for _, id := range []domain.SessionID{"project-old", "project-new"} {
		if !store.sessions[id].IsTerminated {
			live++
		}
		if len(store.worktrees[id]) > 0 {
			marked++
		}
	}
	if live != 1 {
		t.Fatalf("live orchestrators = %d, want exactly one", live)
	}
	if marked != 1 {
		t.Fatalf("recoverable restore markers = %d, want one non-selected marker preserved", marked)
	}
}
