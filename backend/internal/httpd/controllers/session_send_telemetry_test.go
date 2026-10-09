package controllers_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/controllers"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

type failingSendSessionService struct {
	*fakeSessionService
	err error
}

func (s *failingSendSessionService) Send(
	context.Context,
	domain.SessionID,
	string,
	*ports.SpawnAttachment,
) error {
	return s.err
}

func (s *failingSendSessionService) SendWithOptions(
	context.Context,
	domain.SessionID,
	string,
	*ports.SpawnAttachment,
	ports.MessageDeliveryOptions,
) error {
	return s.err
}

func exerciseFailedSessionSend(
	t *testing.T,
	session domain.Session,
) envelope.CapturedError {
	t.Helper()
	base := newFakeSessionService()
	base.sessions[session.ID] = session
	svc := &failingSendSessionService{
		fakeSessionService: base,
		err:                errors.New("runtime transport failed"),
	}

	router := chi.NewRouter()
	controller := &controllers.SessionsController{Svc: svc}
	controller.Register(router)

	req := httptest.NewRequest(
		http.MethodPost,
		"/sessions/"+string(session.ID)+"/send",
		bytes.NewBufferString(`{"message":"follow up"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req, captured := envelope.WithErrorCapture(req)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	return captured()
}

func TestSessionSendFailureCapturesChatControllerState(t *testing.T) {
	session := newFakeSessionService().sessions["ao-1"]
	session.Mode = domain.SessionModeChat
	session.Status = domain.StatusWorking
	session.Activity.State = domain.ActivityActive
	session.ChatProviderPreserved = false

	captured := exerciseFailedSessionSend(t, session)
	if got := captured.Fields["session_mode"]; got != "chat" {
		t.Fatalf("session_mode = %#v, want chat", got)
	}
	if got := captured.Fields["session_runtime_state"]; got != "chat_unavailable" {
		t.Fatalf("session_runtime_state = %#v, want chat_unavailable", got)
	}
	if _, leaked := captured.Fields["session_id"]; leaked {
		t.Fatalf("telemetry leaked session id: %#v", captured.Fields)
	}
}

func TestSessionSendFailureCapturesTUIRuntimeState(t *testing.T) {
	session := newFakeSessionService().sessions["ao-1"]
	session.Mode = domain.SessionModeTUI
	session.Status = domain.StatusWorking
	session.Activity.State = domain.ActivityActive
	session.Metadata.RuntimeHandleID = "runtime-handle"

	captured := exerciseFailedSessionSend(t, session)
	if got := captured.Fields["session_mode"]; got != "tui" {
		t.Fatalf("session_mode = %#v, want tui", got)
	}
	if got := captured.Fields["session_runtime_state"]; got != "active" {
		t.Fatalf("session_runtime_state = %#v, want active", got)
	}
}

func TestSessionSendFailureCapturesDurableTerminalStates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*domain.Session)
		want   string
	}{
		{
			name: "provisioning",
			mutate: func(session *domain.Session) {
				session.ProvisionState = domain.SessionProvisionProvisioning
			},
			want: "provisioning",
		},
		{
			name: "hibernated",
			mutate: func(session *domain.Session) {
				now := session.UpdatedAt
				session.HibernatedAt = &now
			},
			want: "hibernated",
		},
		{
			name: "exited",
			mutate: func(session *domain.Session) {
				session.Activity.State = domain.ActivityExited
			},
			want: "exited",
		},
		{
			name: "terminated",
			mutate: func(session *domain.Session) {
				session.IsTerminated = true
			},
			want: "terminated",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := newFakeSessionService().sessions["ao-1"]
			tc.mutate(&session)
			captured := exerciseFailedSessionSend(t, session)
			if got := captured.Fields["session_runtime_state"]; got != tc.want {
				t.Fatalf("session_runtime_state = %#v, want %s", got, tc.want)
			}
		})
	}
}
