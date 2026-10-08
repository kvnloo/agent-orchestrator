package agent

import (
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/opencode"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestInstallationCheckTimeoutForHarnessOutlivesOpenCodeProbe(t *testing.T) {
	for _, harness := range []domain.AgentHarness{domain.HarnessOpenCode, domain.HarnessOpenCodeV2} {
		got := installationCheckTimeoutForHarness(defaultInstallCheckTimeout, harness, "windows")
		probe := opencode.VersionProbeTimeout("windows")
		if got < probe+2*time.Second {
			t.Fatalf("%s readiness timeout = %v, want at least %v", harness, got, probe+2*time.Second)
		}
	}
}

func TestInstallationCheckTimeoutForHarnessPreservesOtherAgentsAndOverrides(t *testing.T) {
	if got := installationCheckTimeoutForHarness(defaultInstallCheckTimeout, domain.HarnessClaudeCode, "windows"); got != defaultInstallCheckTimeout {
		t.Fatalf("Claude timeout = %v, want default %v", got, defaultInstallCheckTimeout)
	}

	custom := 45 * time.Second
	if got := installationCheckTimeoutForHarness(custom, domain.HarnessOpenCode, "windows"); got != custom {
		t.Fatalf("explicit OpenCode timeout = %v, want configured %v", got, custom)
	}
}

func TestOpenCodeReadinessBudgetTracksPlatformProbePolicy(t *testing.T) {
	windows := installationCheckTimeoutForHarness(defaultInstallCheckTimeout, domain.HarnessOpenCode, "windows")
	linux := installationCheckTimeoutForHarness(defaultInstallCheckTimeout, domain.HarnessOpenCode, "linux")
	if windows <= linux {
		t.Fatalf("Windows readiness timeout = %v, want greater than Linux %v", windows, linux)
	}
}
