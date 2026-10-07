package auth

import (
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

func TestCodexUsageCeilingBlocksAtConfiguredThreshold(t *testing.T) {
	auth := &Auth{
		Provider: "codex",
		Quota: QuotaState{
			ObservedAt: time.Now(),
			Signals: map[string]string{
				"X-Codex-Primary-Window-Minutes": "300",
				"X-Codex-Primary-Used-Percent":   "95",
				"X-Codex-Secondary-Window-Minutes": "10080",
				"X-Codex-Secondary-Used-Percent":   "97.9",
			},
		},
	}
	opts := cliproxyexecutor.Options{Metadata: map[string]any{
		cliproxyexecutor.CodexFiveHourUsageCeilingPercentMetadataKey: 95.0,
		cliproxyexecutor.CodexWeeklyUsageCeilingPercentMetadataKey:   98.0,
	}}

	if codexUsageCeilingAllowsAuth(auth, opts) {
		t.Fatal("expected auth to be blocked at the 5-hour ceiling")
	}
}

func TestCodexUsageCeilingAllowsBelowThreshold(t *testing.T) {
	auth := &Auth{
		Provider: "codex",
		Quota: QuotaState{
			ObservedAt: time.Now(),
			Signals: map[string]string{
				"X-Codex-Primary-Window-Minutes":   "300",
				"X-Codex-Primary-Used-Percent":     "94.9",
				"X-Codex-Secondary-Window-Minutes": "10080",
				"X-Codex-Secondary-Used-Percent":   "97.9",
			},
		},
	}
	opts := cliproxyexecutor.Options{Metadata: map[string]any{
		cliproxyexecutor.CodexFiveHourUsageCeilingPercentMetadataKey: 95.0,
		cliproxyexecutor.CodexWeeklyUsageCeilingPercentMetadataKey:   98.0,
	}}

	if !codexUsageCeilingAllowsAuth(auth, opts) {
		t.Fatal("expected auth below both ceilings to remain eligible")
	}
}

func TestCodexQuotaExpiredResetAtDoesNotKeepBlocking(t *testing.T) {
	now := time.Unix(2000, 0)
	quota := QuotaState{
		ObservedAt: time.Unix(1000, 0),
		Signals: map[string]string{
			"X-Codex-Primary-Window-Minutes": "300",
			"X-Codex-Primary-Used-Percent":   "99",
			"X-Codex-Primary-Reset-At":       "1500",
		},
	}

	used, ok := codexQuotaUsedPercentForWindow(quota, codexFiveHourWindowMinutes, now)
	if !ok || used != 0 {
		t.Fatalf("expired quota snapshot = (%v, %v), want (0, true)", used, ok)
	}
}

func TestCodexQuotaResetAfterSecondsUsesObservationTime(t *testing.T) {
	observedAt := time.Unix(1000, 0)
	quota := QuotaState{
		ObservedAt: observedAt,
		Signals: map[string]string{
			"X-Codex-Primary-Window-Minutes":      "300",
			"X-Codex-Primary-Used-Percent":        "99",
			"X-Codex-Primary-Reset-After-Seconds": "300",
		},
	}

	if used, ok := codexQuotaUsedPercentForWindow(quota, codexFiveHourWindowMinutes, observedAt.Add(299*time.Second)); !ok || used != 99 {
		t.Fatalf("pre-reset quota snapshot = (%v, %v), want (99, true)", used, ok)
	}
	if used, ok := codexQuotaUsedPercentForWindow(quota, codexFiveHourWindowMinutes, observedAt.Add(300*time.Second)); !ok || used != 0 {
		t.Fatalf("post-reset quota snapshot = (%v, %v), want (0, true)", used, ok)
	}
}

func TestCodexUsageCeilingDoesNotAffectOtherProviders(t *testing.T) {
	auth := &Auth{Provider: "claude"}
	opts := cliproxyexecutor.Options{Metadata: map[string]any{
		cliproxyexecutor.CodexFiveHourUsageCeilingPercentMetadataKey: 95.0,
	}}

	if !codexUsageCeilingAllowsAuth(auth, opts) {
		t.Fatal("Codex ceiling unexpectedly blocked non-Codex auth")
	}
}
