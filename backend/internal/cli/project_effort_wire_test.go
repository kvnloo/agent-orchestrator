package cli

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const projectEffortJSON = `{"agentConfig":{"model":"base","effort":"low"},"worker":{"agent":"codex","agentConfig":{"model":"worker","effort":"medium"}},"orchestrator":{"agent":"claude-code","agentConfig":{"model":"orchestrator","effort":"high"}},"reviewers":[{"harness":"claude-code","agentConfig":{"model":"reviewer","effort":"medium"}}]}`

func assertProjectEffortWire(t *testing.T, data []byte) {
	t.Helper()
	var cfg domain.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AgentConfig.Effort != "low" {
		t.Errorf("top-level effort = %q, want low", cfg.AgentConfig.Effort)
	}
	if cfg.Worker.AgentConfig.Effort != "medium" {
		t.Errorf("worker effort = %q, want medium", cfg.Worker.AgentConfig.Effort)
	}
	if cfg.Orchestrator.AgentConfig.Effort != "high" {
		t.Errorf("orchestrator effort = %q, want high", cfg.Orchestrator.AgentConfig.Effort)
	}
	if len(cfg.Reviewers) != 1 || cfg.Reviewers[0].AgentConfig.Effort != "medium" {
		t.Errorf("reviewer effort was lost: %s", data)
	}
}

func TestProjectSetConfigEffortWireRoundTrip(t *testing.T) {
	cfg := setConfigEnv(t)
	srv, capture := projectServer(t, http.StatusOK, `{"project":{"id":"demo","config":`+projectEffortJSON+`}}`)
	writeRunFileFor(t, cfg, srv)
	out, stderr, err := executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
		"project", "set-config", "demo", "--config-json", projectEffortJSON, "--json")
	if err != nil {
		t.Fatalf("set-config: %v; stderr=%s", err, stderr)
	}
	if capture.method != http.MethodPut || capture.path != "/api/v1/projects/demo/config" {
		t.Fatalf("unexpected request %s %s", capture.method, capture.path)
	}
	var request struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(capture.body, &request); err != nil {
		t.Fatal(err)
	}
	assertProjectEffortWire(t, request.Config)
	var response struct {
		Project struct {
			Config json.RawMessage `json:"config"`
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatal(err)
	}
	assertProjectEffortWire(t, response.Project.Config)
}

func TestProjectGetEffortWireRoundTrip(t *testing.T) {
	cfg := setConfigEnv(t)
	srv, _ := projectServer(t, http.StatusOK, `{"status":"ok","project":{"id":"demo","config":`+projectEffortJSON+`}}`)
	writeRunFileFor(t, cfg, srv)
	out, stderr, err := executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }}, "project", "get", "demo", "--json")
	if err != nil {
		t.Fatalf("project get: %v; stderr=%s", err, stderr)
	}
	var response struct {
		Project struct {
			Config json.RawMessage `json:"config"`
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatal(err)
	}
	assertProjectEffortWire(t, response.Project.Config)
}
