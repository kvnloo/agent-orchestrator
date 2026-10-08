package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestModernACPModelSelectionOverWire(t *testing.T) {
	catalog := []acpsdk.SessionConfigOption{
		selectConfigOption("model", "Model", "model", "sonnet", "sonnet", "haiku"),
	}
	for _, tc := range []struct {
		name    string
		catalog []acpsdk.SessionConfigOption
		model   string
		invalid bool
	}{
		{name: "unadvertised", catalog: catalog, model: "unadvertised", invalid: true},
		{name: "advertised", catalog: catalog, model: "haiku"},
		{name: "no catalog", model: "provider-model"},
		{name: "other option only", catalog: []acpsdk.SessionConfigOption{selectConfigOption("effort", "Effort", "thought_level", "low", "low")}, model: "provider-model"},
		{name: "empty model catalog", catalog: []acpsdk.SessionConfigOption{selectConfigOption("model", "Model", "model", "")}, model: "provider-model", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &fakeAgent{newConfig: tc.catalog}
			driver := New(Config{
				Harness: domain.HarnessClaudeCode,
				Probe:   func(context.Context) error { return nil },
				Launch:  func(context.Context, LaunchConfig) (Launch, error) { return Launch{Command: "fake"}, nil },
				SessionOptions: func(settings ports.ChatTurnSettings) []SessionOption {
					return []SessionOption{{ID: "model", Value: settings.Model}}
				},
			}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			driver.useTestProcess(fakeSpawn(agent))
			opened, err := driver.Start(context.Background(), ports.ChatStartConfig{WorkspacePath: t.TempDir(), Model: tc.model})
			if opened != nil {
				defer opened.Close()
			}
			agent.mu.Lock()
			calls, selected := agent.setCalls, agent.options["model"]
			agent.mu.Unlock()
			if tc.invalid {
				if calls != 0 {
					t.Errorf("invalid model sent %d setter RPCs", calls)
				}
				if !errors.Is(err, ports.ErrChatConfigOptionInvalid) || !strings.Contains(err.Error(), tc.model) {
					t.Fatalf("want typed invalid-model error naming %q, got %v", tc.model, err)
				}
			} else if err != nil || calls != 1 || selected != tc.model {
				t.Fatalf("model=%q calls=%d err=%v, want one successful setter for %q", selected, calls, err, tc.model)
			}
		})
	}
}

func TestModernACPModelSendTurnRejectsBeforeSetter(t *testing.T) {
	agent := &fakeAgent{newConfig: []acpsdk.SessionConfigOption{
		selectConfigOption("model", "Model", "model", "sonnet", "sonnet"),
	}}
	driver := New(Config{
		Harness: domain.HarnessClaudeCode,
		Probe:   func(context.Context) error { return nil },
		Launch:  func(context.Context, LaunchConfig) (Launch, error) { return Launch{Command: "fake"}, nil },
		SessionOptions: func(settings ports.ChatTurnSettings) []SessionOption {
			return []SessionOption{{ID: "model", Value: settings.Model}}
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	driver.useTestProcess(fakeSpawn(agent))
	opened, err := driver.Start(context.Background(), ports.ChatStartConfig{WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	ref, err := opened.SendTurn(context.Background(), ports.ChatUserMessage{Text: "hello", Settings: ports.ChatTurnSettings{Model: "unadvertised"}})
	if !errors.Is(err, ports.ErrChatConfigOptionInvalid) || ref.ProviderTurnID != "" {
		t.Errorf("SendTurn = %#v, %v; want typed rejection without preparing a turn", ref, err)
	}
	agent.mu.Lock()
	defer agent.mu.Unlock()
	if agent.setCalls != 0 {
		t.Errorf("invalid model sent %d setter RPCs", agent.setCalls)
	}
}
