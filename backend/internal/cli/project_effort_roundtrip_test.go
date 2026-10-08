package cli

import (
	"encoding/json"
	"testing"
)

func TestProjectConfigJSONRoundTripsAgentEffort(t *testing.T) {
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
	if roundTrip.AgentConfig.Effort != "low" ||
		roundTrip.Worker.AgentConfig.Effort != "medium" ||
		roundTrip.Orchestrator.AgentConfig.Effort != "high" ||
		roundTrip.Reviewers[0].AgentConfig == nil ||
		roundTrip.Reviewers[0].AgentConfig.Effort != "medium" {
		t.Fatalf("effort values were dropped by round trip: %s", encoded)
	}
}
