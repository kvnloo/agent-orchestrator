//go:build windows

// External validation only: not part of the candidate commit or a live provider.
package sessionmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	"golang.org/x/sys/windows"
)

type waveBChildLauncher struct {
	*recordingLauncher
	cmd     *exec.Cmd
	done    chan struct{}
	stopErr error
}

func (l *waveBChildLauncher) StopChat(ctx context.Context, id domain.SessionID) error {
	l.stopped = append(l.stopped, id)
	if l.stopErr != nil {
		return l.stopErr
	}
	if err := l.cmd.Process.Kill(); err != nil {
		return err
	}
	select {
	case <-l.done:
		l.live = false
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type waveBRealWorktree struct {
	*fakeWorkspace
	repo string
}

func (w *waveBRealWorktree) ForceDestroy(ctx context.Context, info ports.WorkspaceInfo) error {
	out, err := exec.CommandContext(ctx, "git", "-C", w.repo, "worktree", "remove", "--force", info.Path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("real git worktree remove: %w: %s", err, out)
	}
	return nil
}

func TestWaveBWindowsRetirementReleasesChildWorktree(t *testing.T) {
	if path := os.Getenv("WAVE_B_LOCK_WORKTREE"); path != "" {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		// Deliberately omit FILE_SHARE_DELETE, like a child retaining a cwd or
		// file handle. No simulation of Windows delete semantics in the parent.
		h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer windows.CloseHandle(h)
		if err := os.Chdir(path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("WAVE_B_READY"), []byte("locked"), 0600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Second)
		}
	}
	for _, failFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop-failure-first=%v", failFirst), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			root := t.TempDir()
			repo := filepath.Join(root, "repo")
			worktree := filepath.Join(root, "worktree")
			ready := filepath.Join(root, "ready")
			git := func(args ...string) {
				t.Helper()
				out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
			}
			git("init", repo)
			git("-C", repo, "-c", "user.name=Validation Fixture", "-c", "user.email=validation@example.invalid", "commit", "--allow-empty", "-m", "fixture")
			git("-C", repo, "worktree", "add", "-b", "ao/wave-b-orchestrator", worktree)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestWaveBWindowsRetirementReleasesChildWorktree$")
			cmd.Env = append(os.Environ(), "WAVE_B_LOCK_WORKTREE="+worktree, "WAVE_B_READY="+ready)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("child did not settle during cleanup")
				}
			})
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("child never acquired the worktree lock")
				case <-done:
					t.Fatal("child exited before acquiring the worktree lock")
				case <-time.After(20 * time.Millisecond):
				}
			}
			if err := os.Rename(worktree, worktree+"-moved"); err == nil {
				_ = os.Rename(worktree+"-moved", worktree)
				t.Fatal("negative control: live child did not deny worktree rename")
			} else {
				t.Logf("NATIVE_LOCK_CONTROL child_pid=%d rename_denied=%v", cmd.Process.Pid, err)
			}
			launcher := &waveBChildLauncher{recordingLauncher: &recordingLauncher{live: true}, cmd: cmd, done: done}
			m, st, _ := newChatManager(launcher)
			m.dataDir = filepath.Join(root, "ao-data")
			m.workspace = &waveBRealWorktree{fakeWorkspace: &fakeWorkspace{}, repo: repo}
			id := domain.SessionID("mer-orchestrator")
			rec := chatOrchestratorForReplacement(id)
			rec.Metadata.WorkspacePath = worktree
			rec.Metadata.Branch = "ao/wave-b-orchestrator"
			st.sessions[id] = rec
			st.worktrees[id] = []domain.SessionWorktreeRecord{{SessionID: id, RepoName: domain.RootWorkspaceRepoName, WorktreePath: worktree, Branch: rec.Metadata.Branch, State: "active"}}
			if failFirst {
				launcher.stopErr = errors.New("injected child stop refusal")
				if err := m.RetireForReplacement(ctx, id); !errors.Is(err, launcher.stopErr) {
					t.Fatalf("stop failure = %v", err)
				}
				if st.sessions[id].IsTerminated || len(st.worktrees[id]) != 1 {
					t.Fatal("failed stop mutated durable live-session facts")
				}
				select {
				case <-done:
					t.Fatal("failed stop did not leave the real child alive")
				default:
				}
				launcher.stopErr = nil
			}
			if err := m.RetireForReplacement(ctx, id); err != nil {
				t.Fatalf("retire with real child-held worktree: %v", err)
			}
			select {
			case <-done:
			default:
				t.Fatal("retirement returned before child settlement")
			}
			if _, err := os.Stat(worktree); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("retired worktree still exists: %v", err)
			}
			if !st.sessions[id].IsTerminated || len(st.worktrees[id]) != 0 {
				t.Fatal("retirement did not finish durable state transition")
			}
			git("-C", repo, "worktree", "add", worktree, rec.Metadata.Branch)
			git("-C", repo, "worktree", "remove", "--force", worktree)
			t.Logf("NATIVE_CHILD_SETTLED pid=%d canonical_branch_reclaimed=true", cmd.Process.Pid)
		})
	}
}
