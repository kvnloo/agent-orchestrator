package sessionmanager

import (
	"context"
	"fmt"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// stopReplacementController releases the one live agent controller before a
// replacement tears down the session workspace. Chat has no runtime handle: its
// controller owns the provider child process, so leaving it alive can keep the
// worktree open (especially on Windows) and can publish late events into a row
// AO already marked terminated.
func (m *Manager) stopReplacementController(ctx context.Context, rec domain.SessionRecord) error {
	if domain.NormalizeSessionMode(rec.Mode) == domain.SessionModeChat {
		if m.chat == nil {
			return ErrIncompleteHandle
		}
		if err := m.chat.StopChat(ctx, rec.ID); err != nil {
			return fmt.Errorf("chat controller: %w", err)
		}
		return nil
	}
	if err := m.terminateNativeSession(ctx, rec); err != nil {
		return fmt.Errorf("native session: %w", err)
	}
	return nil
}
