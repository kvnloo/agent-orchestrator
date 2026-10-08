package chat_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/kimiacp"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	chatsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/chat"
)

type kimiLaunchValidationPlugin struct{}

func (kimiLaunchValidationPlugin) ResolveBinary(context.Context) (string, error) {
	return "/usr/bin/kimi", nil
}

func (kimiLaunchValidationPlugin) AuthStatus(context.Context) (ports.AgentAuthStatus, error) {
	return ports.AgentAuthStatusAuthorized, nil
}

func TestPreflightChatRunsProviderLaunchValidation(t *testing.T) {
	driver := kimiacp.New(kimiLaunchValidationPlugin{}, nil)
	service := chatsvc.New(chatsvc.Options{Drivers: fakeRegistry{driver: driver}})

	if err := service.PreflightChat(
		context.Background(),
		domain.HarnessKimi,
		ports.PermissionModeDefault,
	); err != nil {
		t.Fatalf("default Kimi preflight: %v", err)
	}

	err := service.PreflightChat(
		context.Background(),
		domain.HarnessKimi,
		ports.PermissionModeAuto,
	)
	if !errors.Is(err, ports.ErrChatPermissionModeUnsupported) {
		t.Fatalf("auto Kimi preflight error = %v, want ErrChatPermissionModeUnsupported", err)
	}
}
