package sessionmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

type retirementChatLauncher struct {
	*recordingLauncher
	stopErr error
}

func (l *retirementChatLauncher) StopChat(_ context.Context, id domain.SessionID) error {
	l.stopped = append(l.stopped, id)
	if l.stopErr == nil {
		l.live = false
	}
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

type retirementChatWorkspace struct {
	*fakeWorkspace
	t        *testing.T
	launcher *retirementChatLauncher
	removed  int
}

func (w *retirementChatWorkspace) ForceDestroy(ctx context.Context, info ports.WorkspaceInfo) error {
	if w.launcher.live {
		w.t.Fatal("workspace teardown reached while Chat controller was still live")
	}
	w.removed++
	return w.fakeWorkspace.ForceDestroy(ctx, info)
}

func TestRetireForReplacementChatStopPrecedesWorkspaceTeardown(t *testing.T) {
	for _, workspace := range []string{"none", "single", "multi"} {
		for _, stopFails := range []bool{false, true} {
			name := workspace + "/stopped"
			if stopFails {
				name = workspace + "/stop-failed"
			}
			t.Run(name, func(t *testing.T) {
				launcher := &retirementChatLauncher{recordingLauncher: &recordingLauncher{live: true}}
				if stopFails {
					launcher.stopErr = errors.New("provider controller still holds workspace")
				}
				m, st, rt := newChatManager(launcher)
				ws := &retirementChatWorkspace{fakeWorkspace: &fakeWorkspace{}, t: t, launcher: launcher}
				m.workspace = ws
				ws.stashHook = func() {
					if launcher.live {
						t.Fatal("workspace preservation reached while Chat controller was still live")
					}
				}
				id := domain.SessionID("mer-orchestrator")
				rec := chatOrchestratorForReplacement(id)
				if workspace != "none" {
					rec.Metadata.WorkspacePath = t.TempDir()
					rec.Metadata.Branch = "ao/mer-orchestrator"
				}
				st.sessions[id] = rec
				st.worktrees[id] = []domain.SessionWorktreeRecord{{SessionID: id, RepoName: domain.RootWorkspaceRepoName, WorktreePath: rec.Metadata.WorkspacePath, Branch: rec.Metadata.Branch, State: "active"}}
				wantRemoved := 0
				if workspace != "none" {
					wantRemoved = 1
				}
				if workspace == "multi" {
					project := st.projects[string(chatTestProject)]
					project.Kind = domain.ProjectKindWorkspace
					project.Path = t.TempDir()
					st.projects[string(chatTestProject)] = project
					st.workspaceRepo[string(chatTestProject)] = []domain.WorkspaceRepoRecord{{ProjectID: chatTestProject, Name: "api", RelativePath: "api"}}
					st.worktrees[id] = append(st.worktrees[id], domain.SessionWorktreeRecord{SessionID: id, RepoName: "api", WorktreePath: rec.Metadata.WorkspacePath + "/api", Branch: rec.Metadata.Branch, State: "active"})
					wantRemoved = 2
				}
				wantRows := len(st.worktrees[id])

				err := m.RetireForReplacement(context.Background(), id)
				if !errors.Is(err, launcher.stopErr) {
					t.Fatalf("retirement error = %v, want %v", err, launcher.stopErr)
				}
				if len(launcher.stopped) != 1 || launcher.stopped[0] != id {
					t.Fatalf("Chat stops = %v, want exactly [%s]", launcher.stopped, id)
				}
				if rt.destroyed != 0 {
					t.Fatalf("Chat retirement destroyed %d TUI runtimes", rt.destroyed)
				}
				if stopFails {
					if !launcher.live || st.sessions[id].IsTerminated || st.sessions[id].Activity.State != domain.ActivityActive {
						t.Fatal("failed stop must leave the old Chat session live")
					}
					if ws.removed != 0 || ws.stashCalls != 0 || len(st.worktrees[id]) != wantRows {
						t.Fatalf("failed stop changed workspace: removed=%d stash=%d rows=%d", ws.removed, ws.stashCalls, len(st.worktrees[id]))
					}
					return
				}
				if launcher.live || !st.sessions[id].IsTerminated || ws.removed != wantRemoved || len(st.worktrees[id]) != 0 {
					t.Fatalf("retirement incomplete: live=%v terminated=%v removed=%d rows=%d", launcher.live, st.sessions[id].IsTerminated, ws.removed, len(st.worktrees[id]))
				}
			})
		}
	}
}
