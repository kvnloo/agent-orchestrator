package cli

import (
	"strings"
	"testing"
)

func TestProjectSetConfigRejectsUnknownJSONFieldBeforeRequest(t *testing.T) {
	cfg := setConfigEnv(t)
	srv, capture := projectServer(t, 200, `{"status":"ok","project":{"id":"demo"}}`)
	writeRunFileFor(t, cfg, srv)

	_, stderr, err := executeCLI(t, Deps{
		ProcessAlive: func(int) bool { return true },
	}, "project", "set-config", "demo", "--config-json", `{"typoOnly":true}`)
	if err == nil {
		t.Fatal("unknown config key unexpectedly succeeded")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d, want usage error 2; err=%v stderr=%s", got, err, stderr)
	}
	message := err.Error() + "\n" + stderr
	if !strings.Contains(message, "typoOnly") || !strings.Contains(strings.ToLower(message), "unknown") {
		t.Fatalf("error must identify the unknown key: %s", message)
	}
	if capture.method != "" || capture.path != "" {
		t.Fatalf("unknown JSON reached the daemon as %s %s", capture.method, capture.path)
	}
}

func TestBuildProjectConfigRejectsNestedUnknownJSONField(t *testing.T) {
	_, err := buildProjectConfig(projectSetConfigOptions{
		configJSON: `{"worker":{"agent":"codex","agentConfig":{"model":"gpt-5.6-sol","typoEffort":"high"}}}`,
	})
	if err == nil {
		t.Fatal("nested unknown config key unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "typoEffort") {
		t.Fatalf("error = %q, want nested key name", err)
	}
}
