package z0intelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

const spawnDecisionPath = "/v1/integrations/agent-orchestrator/spawn-decision"

// Client is the loopback-only HTTP adapter for the z0intelligence policy plane.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a loopback-only z0intelligence client. The AO daemon has access
// to private task text, so this experimental bridge must never become an
// arbitrary outbound URL.
func New(rawURL string, timeout time.Duration) (*Client, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse z0intelligence URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("z0intelligence URL must be an http(s) loopback URL")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("z0intelligence URL must use a loopback host")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("z0intelligence URL must not contain query or fragment")
	}
	if timeout <= 0 {
		timeout = 250 * time.Millisecond
	}
	return &Client{
		baseURL: strings.TrimRight(rawURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// AdviseSpawn records and retrieves non-authoritative shadow advice for one spawn.
func (c *Client) AdviseSpawn(ctx context.Context, in ports.SpawnDecisionRequest) (ports.SpawnDecision, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return ports.SpawnDecision{}, fmt.Errorf("encode spawn decision: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+spawnDecisionPath, bytes.NewReader(body))
	if err != nil {
		return ports.SpawnDecision{}, fmt.Errorf("build spawn decision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return ports.SpawnDecision{}, fmt.Errorf("z0intelligence spawn decision: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return ports.SpawnDecision{}, fmt.Errorf("z0intelligence spawn decision: HTTP %d", resp.StatusCode)
	}

	var out ports.SpawnDecision
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return ports.SpawnDecision{}, fmt.Errorf("decode z0intelligence spawn decision: %w", err)
	}
	if out.Schema != ports.SpawnDecisionSchema || strings.TrimSpace(out.DecisionID) == "" || strings.TrimSpace(out.ReceiptID) == "" {
		return ports.SpawnDecision{}, fmt.Errorf("invalid z0intelligence spawn decision identity")
	}
	switch out.Action {
	case "keep", "recommend", "abstain":
	default:
		return ports.SpawnDecision{}, fmt.Errorf("invalid z0intelligence spawn decision action %q", out.Action)
	}
	return out, nil
}

var _ ports.IntelligenceAdvisor = (*Client)(nil)
