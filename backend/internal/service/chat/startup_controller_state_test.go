package chat

import (
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestPreConversationControllerStateDescribesAControllerThatHasNotStarted(t *testing.T) {
	hibernatedAt := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		record domain.SessionRecord
		want   ports.ChatControllerState
	}{
		{
			name:   "ordinary fresh session",
			record: domain.SessionRecord{Activity: domain.Activity{State: domain.ActivityIdle}},
			want:   ports.ChatControllerConnecting,
		},
		{
			name: "active startup gap",
			record: domain.SessionRecord{
				Activity: domain.Activity{State: domain.ActivityActive},
			},
			want: ports.ChatControllerConnecting,
		},
		{
			name: "terminated before first conversation",
			record: domain.SessionRecord{
				IsTerminated: true,
				Activity:     domain.Activity{State: domain.ActivityIdle},
			},
			want: ports.ChatControllerStopped,
		},
		{
			name: "agent process exited before first conversation",
			record: domain.SessionRecord{
				Activity: domain.Activity{State: domain.ActivityExited},
			},
			want: ports.ChatControllerStopped,
		},
		{
			name: "hibernated session",
			record: domain.SessionRecord{
				HibernatedAt: &hibernatedAt,
				Activity:     domain.Activity{State: domain.ActivityIdle},
			},
			want: ports.ChatControllerHibernated,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := preConversationControllerState(tc.record); got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExistingConversationWithoutControllerStillReportsStopped(t *testing.T) {
	record := domain.SessionRecord{Activity: domain.Activity{State: domain.ActivityIdle}}
	if got := idleControllerState(record); got != ports.ChatControllerStopped {
		t.Fatalf("existing conversation state = %q, want stopped", got)
	}
}
