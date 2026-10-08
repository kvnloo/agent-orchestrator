package controllers

import (
	"net/http"
	"strings"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
)

// captureSessionSendFailureContext adds bounded session facts to request-local
// telemetry after a send fails. The diagnostic read is best effort: it must
// never replace or wrap the original delivery error, and it deliberately
// excludes identifiers, paths, prompts, attachments, models, and provider IDs.
func captureSessionSendFailureContext(
	r *http.Request,
	svc SessionService,
	id domain.SessionID,
) {
	if r == nil || svc == nil {
		return
	}
	session, err := svc.Get(r.Context(), id)
	if err != nil {
		return
	}
	envelope.SetTelemetryField(
		r,
		"session_mode",
		string(domain.NormalizeSessionMode(session.Mode)),
	)
	envelope.SetTelemetryField(
		r,
		"session_runtime_state",
		sessionSendRuntimeState(session),
	)
}

func sessionSendRuntimeState(session domain.Session) string {
	switch {
	case session.IsTerminated:
		return "terminated"
	case session.ProvisionState.IsProvisioning():
		return "provisioning"
	case session.HibernatedAt != nil:
		return "hibernated"
	case session.Activity.State == domain.ActivityExited:
		return "exited"
	}

	if domain.NormalizeSessionMode(session.Mode) == domain.SessionModeChat {
		if session.ChatProviderPreserved {
			return "chat_live"
		}
		return "chat_unavailable"
	}
	if strings.TrimSpace(session.Metadata.RuntimeHandleID) == "" {
		return "runtime_missing"
	}
	state := strings.TrimSpace(string(session.Activity.State))
	if state == "" {
		return "unknown"
	}
	return state
}
