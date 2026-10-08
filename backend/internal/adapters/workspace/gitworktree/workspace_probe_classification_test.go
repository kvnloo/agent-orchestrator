package gitworktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

const (
	probeSleepHelperEnv      = "GO_WANT_GITWORKTREE_PROBE_SLEEP"
	probeSleepHelperBackstop = 30 * time.Second
)

func workspaceWithProbeError(t *testing.T, runErr error) *Workspace {
	t.Helper()
	workspace, err := New(Options{
		ManagedRoot: t.TempDir(),
		RepoResolver: StaticRepoResolver{
			"project": t.TempDir(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace.run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, runErr
	}
	return workspace
}

func TestValidateBranchDoesNotCallInterruptedProbeAnInvalidBranch(t *testing.T) {
	const validBranch = "ao/agent-orchestrator-291/root"
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{
			name: "request canceled",
			err:  fmt.Errorf("git wrapper: %w", context.Canceled),
			want: context.Canceled,
		},
		{
			name: "deadline exceeded",
			err:  fmt.Errorf("git wrapper: %w", context.DeadlineExceeded),
			want: context.DeadlineExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := workspaceWithProbeError(t, tc.err).validateBranch(
				context.Background(),
				"/repo",
				validBranch,
			)
			if errors.Is(err, ports.ErrWorkspaceBranchInvalid) {
				t.Fatalf("valid branch was misclassified as invalid: %v", err)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want original interruption %v preserved", err, tc.want)
			}
		})
	}
}

func TestValidateBranchDoesNotCallSignalKilledProbeAnInvalidBranch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal termination is represented differently on Windows")
	}
	const validBranch = "ao/agent-orchestrator-291/root"
	probeErr := signalKilledProbeError(t)
	err := workspaceWithProbeError(t, probeErr).validateBranch(
		context.Background(),
		"/repo",
		validBranch,
	)
	if errors.Is(err, ports.ErrWorkspaceBranchInvalid) {
		t.Fatalf("signal-killed probe was misclassified as invalid branch: %v", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != -1 {
		t.Fatalf("error = %v, want preserved signal-killed ExitError", err)
	}
}

func TestValidateBranchStillClassifiesGenuineGitRejection(t *testing.T) {
	err := workspaceWithProbeError(t, exitStatusOne(t)).validateBranch(
		context.Background(),
		"/repo",
		"bad branch!!",
	)
	if !errors.Is(err, ports.ErrWorkspaceBranchInvalid) {
		t.Fatalf("error = %v, want ErrWorkspaceBranchInvalid", err)
	}
}

func signalKilledProbeError(t *testing.T) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestGitWorktreeProbeSleepHelper")
	cmd.Env = append(os.Environ(), probeSleepHelperEnv+"=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start probe helper: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill probe helper: %v", err)
	}
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("wait error = %v, want *exec.ExitError", err)
	}
	if exitErr.ExitCode() != -1 {
		t.Fatalf("killed process exit code = %d, want -1", exitErr.ExitCode())
	}
	return err
}

func TestGitWorktreeProbeSleepHelper(t *testing.T) {
	if os.Getenv(probeSleepHelperEnv) != "1" {
		return
	}
	time.Sleep(probeSleepHelperBackstop)
}
