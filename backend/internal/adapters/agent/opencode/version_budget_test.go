package opencode

import (
	"testing"
	"time"
)

func TestVersionProbeTimeoutForPlatform(t *testing.T) {
	for _, tc := range []struct {
		goos string
		want time.Duration
	}{
		{goos: "windows", want: 15 * time.Second},
		{goos: "darwin", want: 3 * time.Second},
		{goos: "linux", want: 3 * time.Second},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			if got := VersionProbeTimeout(tc.goos); got != tc.want {
				t.Fatalf("VersionProbeTimeout(%q) = %v, want %v", tc.goos, got, tc.want)
			}
		})
	}
}

func TestWindowsVersionProbeBudgetHasMeasuredHeadroom(t *testing.T) {
	// Reporter measurements peaked at 9.12s on Windows. Require at least five
	// seconds of deterministic headroom so a small cold-start or antivirus
	// variance does not turn a nominal fix into another intermittent timeout.
	const measuredWorst = 9120 * time.Millisecond
	if margin := VersionProbeTimeout("windows") - measuredWorst; margin < 5*time.Second {
		t.Fatalf("Windows probe headroom = %v, want at least 5s above %v", margin, measuredWorst)
	}
}
