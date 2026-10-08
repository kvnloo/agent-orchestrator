package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProjectSetConfigUnknownJSONNeverContactsDaemon(t *testing.T) {
	for _, input := range []string{
		`{"typoOnly":true}`,
		`{"agentConfig":{"typoOnly":"high"}}`,
		`{"worker":{"typoOnly":true}}`,
		`{"worker":{"agentConfig":{"typoOnly":"high"}}}`,
		`{"orchestrator":{"agentConfig":{"typoOnly":"high"}}}`,
		`{"reviewers":[{"harness":"codex","agentConfig":{"typoOnly":"high"}}]}`,
		`{"trackerIntake":{"typoOnly":true}}`,
		`{"containerReap":{"typoOnly":true}}`,
	} {
		t.Run(input, func(t *testing.T) {
			cfg := setConfigEnv(t)
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"project":{"id":"demo"}}`)
			}))
			t.Cleanup(srv.Close)
			writeRunFileFor(t, cfg, srv)
			_, stderr, err := executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
				"project", "set-config", "demo", "--config-json", input)
			if requests.Load() != 0 {
				t.Errorf("invalid config contacted daemon %d times", requests.Load())
			}
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("want usage error; got %v; stderr=%s", err, stderr)
			}
			if !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), "typoOnly") {
				t.Fatalf("error must identify unknown key: %v", err)
			}
		})
	}
}

func TestBuildProjectConfigRejectsTrailingJSON(t *testing.T) {
	for _, input := range []string{`{} {}`, `{} true`, `{} trailing`, `{"defaultBranch":"main"} []`} {
		t.Run(input, func(t *testing.T) {
			_, err := buildProjectConfig(projectSetConfigOptions{configJSON: input})
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("want usage error for trailing JSON, got %v", err)
			}
		})
	}
}

func TestProjectSetConfigInvalidJSONEntrypointNeverContactsDaemon(t *testing.T) {
	for _, input := range []string{`{"typoOnly":true}`, `{"worker":{"agentConfig":{"typoOnly":"high"}}}`, `{} {}`} {
		t.Run(input, func(t *testing.T) {
			cfg := setConfigEnv(t)
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"project":{"id":"demo"}}`)
			}))
			t.Cleanup(srv.Close)
			writeRunFileFor(t, cfg, srv)
			err := executeWithDeps(Deps{ProcessAlive: func(int) bool { return true }, Out: io.Discard, Err: io.Discard},
				[]string{"project", "set-config", "demo", "--config-json", input})
			if requests.Load() != 0 {
				t.Errorf("invalid config contacted daemon %d times", requests.Load())
			}
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("want usage error, got %v", err)
			}
		})
	}
}

func TestBuildProjectConfigStrictJSONPreservesValidConfig(t *testing.T) {
	cfg, err := buildProjectConfig(projectSetConfigOptions{
		configJSON:    " \n" + `{"defaultBranch":"main","env":{"custom-key":"value"},"worker":{"agent":"codex","agentConfig":{"model":"gpt-5"}},"reviewers":[{"harness":"codex","agentConfig":{"permissions":"default"}}]}` + "\n ",
		defaultBranch: "ignored-flag",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultBranch != "main" || cfg.Env["custom-key"] != "value" || cfg.Worker.Agent != "codex" || cfg.Worker.AgentConfig.Model != "gpt-5" || len(cfg.Reviewers) != 1 || cfg.Reviewers[0].AgentConfig.Permissions != "default" {
		t.Fatalf("valid replacement config changed: %#v", cfg)
	}
}
