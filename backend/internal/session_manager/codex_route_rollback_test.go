package sessionmanager

import (
	"context"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

type rollbackRouteProvider struct {
	revoked []string
}

func (p *rollbackRouteProvider) RouteForSession(context.Context, string) (ports.AgentProviderRoute, error) {
	return ports.AgentProviderRoute{}, nil
}

func (p *rollbackRouteProvider) SwitchSessionAccount(context.Context, string, string) (string, error) {
	return "", nil
}

func (p *rollbackRouteProvider) SwitchAllSessionsAccount(context.Context, string) (string, error) {
	return "", nil
}

func (p *rollbackRouteProvider) RevokeSession(_ context.Context, sessionID string) error {
	p.revoked = append(p.revoked, sessionID)
	return nil
}

func TestRollbackSpawnSeedRowRevokesProviderRoute(t *testing.T) {
	m, st, _, _ := newManager()
	routes := &rollbackRouteProvider{}
	m.codexRouteProvider = routes
	st.sessions["mer-1"] = domain.SessionRecord{
		ID:        "mer-1",
		ProjectID: "mer",
		Activity:  domain.Activity{State: domain.ActivityIdle},
	}

	m.rollbackSpawnSeedRow(context.Background(), "mer-1")

	if _, ok := st.sessions["mer-1"]; ok {
		t.Fatal("seed row remained after rollback")
	}
	if len(routes.revoked) != 1 || routes.revoked[0] != "mer-1" {
		t.Fatalf("revoked routes = %#v, want [mer-1]", routes.revoked)
	}
}

func TestRollbackSpawnPublicSeedDeleteRevokesProviderRoute(t *testing.T) {
	m, st, _, _ := newManager()
	routes := &rollbackRouteProvider{}
	m.codexRouteProvider = routes
	st.sessions["mer-1"] = domain.SessionRecord{
		ID:        "mer-1",
		ProjectID: "mer",
		Activity:  domain.Activity{State: domain.ActivityIdle},
	}

	deleted, killed, err := m.RollbackSpawn(context.Background(), "mer-1")
	if err != nil {
		t.Fatalf("RollbackSpawn: %v", err)
	}
	if !deleted || killed {
		t.Fatalf("deleted=%v killed=%v, want true,false", deleted, killed)
	}
	if len(routes.revoked) != 1 || routes.revoked[0] != "mer-1" {
		t.Fatalf("revoked routes = %#v, want [mer-1]", routes.revoked)
	}
}
