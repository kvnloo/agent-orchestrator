package gitworktree

import (
	"context"
	"errors"
	"os/exec"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// ErrBranchProbeFailed is the adapter-local alias used when git never got to
// judge a branch name. Callers outside this package match the port sentinel.
var ErrBranchProbeFailed = ports.ErrWorkspaceProbeFailed

// probeInterrupted distinguishes transport/process interruption from a normal
// nonzero git exit. On Unix, ProcessState.ExitCode reports -1 when the child was
// terminated by a signal. Context errors remain in the wrapped error chain.
func probeInterrupted(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == -1
}
