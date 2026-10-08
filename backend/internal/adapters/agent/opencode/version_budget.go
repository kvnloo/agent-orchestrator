package opencode

import "time"

const (
	defaultVersionProbeTimeout = 3 * time.Second
	windowsVersionProbeTimeout = 15 * time.Second
)

// VersionProbeTimeout returns the bounded launch/readiness probe budget for a
// platform. Windows npm shims and cold Node/Bun startup have been measured above
// nine seconds; keep deterministic headroom there without slowing healthy
// probes or broadening the unmeasured macOS/Linux policy.
func VersionProbeTimeout(goos string) time.Duration {
	if goos == "windows" {
		return windowsVersionProbeTimeout
	}
	return defaultVersionProbeTimeout
}
