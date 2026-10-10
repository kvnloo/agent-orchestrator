package telemetry

import "testing"

// A failed session send records coarse session facts on the request. They are
// only useful if they survive the remote allow-list and reach the exported
// event unchanged.
func TestHTTP5xxExportKeepsFailedSendSessionFacts(t *testing.T) {
	for _, tc := range []struct{ mode, state string }{
		{"chat", "chat_unavailable"},
		{"chat", "chat_live"},
		{"tui", "runtime_missing"},
		{"tui", "active"},
		{"tui", "terminated"},
	} {
		out := sanitizeRemotePayload("ao.http.5xx", map[string]any{
			"status":                500,
			"session_mode":          tc.mode,
			"session_runtime_state": tc.state,
		})
		if got := out["session_mode"]; got != tc.mode {
			t.Errorf("session_mode exported as %#v, want %q", got, tc.mode)
		}
		if got := out["session_runtime_state"]; got != tc.state {
			t.Errorf("session_runtime_state exported as %#v, want %q", got, tc.state)
		}
	}
}

// The facts are enums. Nothing that identifies a session may ride along.
func TestHTTP5xxExportStillDropsSessionIdentifiers(t *testing.T) {
	out := sanitizeRemotePayload("ao.http.5xx", map[string]any{
		"status":       500,
		"session_mode": "chat",
		"session_id":   "ao-1",
		"sessionId":    "ao-1",
	})
	for _, key := range []string{"session_id", "sessionId"} {
		if _, leaked := out[key]; leaked {
			t.Errorf("%s exported: %#v", key, out)
		}
	}
}
