package session

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestToAPIErrorMapsWorkspaceProbeFailureToUnavailable(t *testing.T) {
	err := toAPIError(fmt.Errorf("spawn workspace: %w", ports.ErrWorkspaceProbeFailed))
	var apiError *apierr.Error
	if !errors.As(err, &apiError) {
		t.Fatalf("error = %v, want *apierr.Error", err)
	}
	if apiError.Kind != apierr.KindUnavailable || apiError.Code != "WORKSPACE_PROBE_FAILED" {
		t.Fatalf("API error = %+v, want unavailable WORKSPACE_PROBE_FAILED", apiError)
	}
	if !errors.Is(err, ports.ErrWorkspaceProbeFailed) {
		t.Fatalf("mapped error lost probe sentinel: %v", err)
	}
}
