package agent

import (
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/opencode"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const openCodeReadinessHeadroom = 2 * time.Second

func installationCheckTimeoutForHarness(
	configured time.Duration,
	harness domain.AgentHarness,
	goos string,
) time.Duration {
	// Tests and callers that explicitly configure a timeout remain authoritative.
	if configured != defaultInstallCheckTimeout {
		return configured
	}
	switch harness {
	case domain.HarnessOpenCode, domain.HarnessOpenCodeV2:
		return opencode.VersionProbeTimeout(goos) + openCodeReadinessHeadroom
	default:
		return configured
	}
}
