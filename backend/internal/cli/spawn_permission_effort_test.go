package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProjectAgentConfigRoundTripsEffort(t *testing.T) {
	input := []byte(`{
		"agentConfig":{"model":"base","effort":"low"},
		"worker":{"agent":"codex","agentConfig":{"model":"worker","effort":"medium"}},
		"orchestrator":{"agent":"claude-code","agentConfig":{"model":"orchestrator","effort":"high"}},
		"reviewers":[{"harness":"claude-code","agentConfig":{"model":"reviewer","effort":"medium"}}]
	}`)
	var cfg projectConfig
	if err := json.Unmarshal(input, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AgentConfig.Effort != "low" {
		t.Fatalf("top-level effort = %q, want low", cfg.AgentConfig.Effort)
	}
	if cfg.Worker.AgentConfig.Effort != "medium" {
		t.Fatalf("worker effort = %q, want medium", cfg.Worker.AgentConfig.Effort)
	}
	if cfg.Orchestrator.AgentConfig.Effort != "high" {
		t.Fatalf("orchestrator effort = %q, want high", cfg.Orchestrator.AgentConfig.Effort)
	}
	if len(cfg.Reviewers) != 1 || cfg.Reviewers[0].AgentConfig == nil || cfg.Reviewers[0].AgentConfig.Effort != "medium" {
		t.Fatalf("reviewer effort did not survive decode: %#v", cfg.Reviewers)
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip projectConfig
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.AgentConfig.Effort != "low" || roundTrip.Worker.AgentConfig.Effort != "medium" || roundTrip.Orchestrator.AgentConfig.Effort != "high" {
		t.Fatalf("effort values were dropped by round trip: %s", encoded)
	}
}

func TestSpawnForwardsPermissionAndEffortOverrides(t *testing.T) {
	cfg := setConfigEnv(t)
	var captured spawnRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/demo":
			_, _ = io.WriteString(w, `{"status":"ok","project":{"id":"demo","name":"Demo","path":"/repo/demo","repo":"https://github.com/aoagents/agent-orchestrator","defaultBranch":"main"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/agents/readiness/ensure":
			_, _ = io.WriteString(w, authorizedAgentsJSON("codex"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sessions":
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatal(err)
			}
			_, _ = io.WriteString(w, `{"session":{"id":"demo-1","status":"idle","displayName":"worker"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	writeRunFileFor(t, cfg, srv)

	_, stderr, err := executeCLI(
		t,
		Deps{ProcessAlive: func(int) bool { return true }},
		"spawn",
		"--project", "demo",
		"--agent", "codex",
		"--name", "worker",
		"--permission", "bypass-permissions",
		"--effort", "high",
	)
	if err != nil {
		t.Fatalf("spawn overrides failed: %v\nstderr: %s", err, stderr)
	}
	if captured.Permissions != "bypass-permissions" {
		t.Fatalf("permissions = %q, want bypass-permissions", captured.Permissions)
	}
	if captured.Effort != "high" {
		t.Fatalf("effort = %q, want high", captured.Effort)
	}
}
