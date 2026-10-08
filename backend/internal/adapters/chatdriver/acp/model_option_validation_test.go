package acp

import (
	"context"
	"errors"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestModernACPModelRejectsUnadvertisedChoiceBeforeRPC(t *testing.T) {
	conv := &conversation{
		sessionID: "session-1",
		optionsFor: func(settings ports.ChatTurnSettings) []SessionOption {
			return []SessionOption{{ID: "model", Value: settings.Model}}
		},
		configOptions: []ports.ChatConfigOption{
			{
				ID:   "model",
				Type: ports.ChatConfigOptionSelect,
				Choices: []ports.ChatConfigOptionChoice{
					{Value: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5"},
				},
			},
		},
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unadvertised model reached the ACP RPC connection: %v", recovered)
		}
	}()

	err := conv.applyTurnSettings(context.Background(), ports.ChatTurnSettings{Model: "claude-haiku-5-5"})
	if !errors.Is(err, ports.ErrChatConfigOptionInvalid) {
		t.Fatalf("error = %v, want ErrChatConfigOptionInvalid", err)
	}
}

func TestModernACPModelKeepsCompatibilityPathWhenCatalogUnknown(t *testing.T) {
	conv := &conversation{
		sessionID: "session-1",
		optionsFor: func(settings ports.ChatTurnSettings) []SessionOption {
			return []SessionOption{{ID: "model", Value: settings.Model}}
		},
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("unknown catalog unexpectedly rejected before the existing RPC compatibility path")
		}
	}()

	// No model option is advertised. The new validation must preserve the
	// existing compatibility path rather than guessing that the value is invalid.
	_ = conv.applyTurnSettings(context.Background(), ports.ChatTurnSettings{Model: "provider-model"})
}
