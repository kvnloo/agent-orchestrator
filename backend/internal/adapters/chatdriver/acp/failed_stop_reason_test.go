package acp

import (
	"log/slog"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestFailedACPStopReasonCarriesTurnCompletionError(t *testing.T) {
	conv := &conversation{
		activeTurn: "turn-1",
		events:     make(chan ports.ChatEvent, 8),
		log:        slog.New(slog.DiscardHandler),
	}

	conv.finishPrompt("turn-1", acpsdk.PromptResponse{
		StopReason: acpsdk.StopReason("provider_error"),
	}, nil)
	close(conv.events)

	var completion ports.ChatEvent
	for event := range conv.events {
		if event.Kind == ports.ChatEventTurnCompleted {
			completion = event
			break
		}
	}
	if completion.Kind != ports.ChatEventTurnCompleted || completion.TurnState != domain.TurnStateFailed {
		t.Fatalf("completion = %#v, want failed turn completion", completion)
	}
	if completion.Err == nil {
		t.Fatal("failed provider stop reason produced an empty turn error")
	}
	if !strings.Contains(completion.Err.Error(), "provider_error") {
		t.Fatalf("completion error = %q, want stop reason", completion.Err)
	}
}
