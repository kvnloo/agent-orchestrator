package z0intelligence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func testRequest() ports.SpawnDecisionRequest {
	return ports.SpawnDecisionRequest{
		Schema:    ports.SpawnDecisionSchema,
		TraceID:   "ao-spawn-proj-1",
		SessionID: "proj-1",
		ProjectID: "proj",
		Kind:      "worker",
		Task:      "fix retry race",
		Current: ports.SpawnDecisionCurrent{
			Harness: "codex", Model: "gpt-5", Mode: "chat", Permission: "default",
		},
	}
}

func TestClientAdviseSpawn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != spawnDecisionPath {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var in ports.SpawnDecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		if in.TraceID != "ao-spawn-proj-1" {
			t.Fatalf("trace = %q", in.TraceID)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ports.SpawnDecision{
			Schema:         ports.SpawnDecisionSchema,
			DecisionID:     in.TraceID,
			Action:         "abstain",
			Recommendation: in.Current,
			Reason:         "shadow",
			PolicyRevision: "test-v1",
			ReceiptID:      in.TraceID,
		})
	}))
	defer server.Close()

	client, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.AdviseSpawn(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.DecisionID != "ao-spawn-proj-1" || got.Action != "abstain" {
		t.Fatalf("decision = %+v", got)
	}
}

func TestClientRejectsNonLoopbackURL(t *testing.T) {
	if _, err := New("https://example.com", time.Second); err == nil {
		t.Fatal("expected non-loopback URL rejection")
	}
}

func TestClientTimeoutIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client, err := New(server.URL, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.AdviseSpawn(context.Background(), testRequest()); err == nil {
		t.Fatal("expected timeout")
	}
}
