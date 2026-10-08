package ports

import "errors"

// ErrWorkspaceProbeFailed reports that a workspace validation probe could not
// complete, so the probe never established whether the user-supplied value was
// valid. It is retryable and must not be grouped with deterministic validation
// failures such as ErrWorkspaceBranchInvalid.
var ErrWorkspaceProbeFailed = errors.New("workspace: validation probe failed")
