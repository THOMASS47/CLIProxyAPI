package auth

import (
	"strconv"
	"strings"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

const (
	codexFiveHourWindowMinutes = 5 * 60
	codexWeeklyWindowMinutes   = 7 * 24 * 60
)

type codexUsageCeilings struct {
	fiveHour float64
	weekly   float64
}

func codexUsageCeilingsFromOptions(opts cliproxyexecutor.Options) codexUsageCeilings {
	return codexUsageCeilings{
		fiveHour: metadataUsageCeilingPercent(opts.Metadata, cliproxyexecutor.CodexFiveHourUsageCeilingPercentMetadataKey),
		weekly:   metadataUsageCeilingPercent(opts.Metadata, cliproxyexecutor.CodexWeeklyUsageCeilingPercentMetadataKey),
	}
}

func (c codexUsageCeilings) configured() bool {
	return c.fiveHour > 0 || c.weekly > 0
}

func metadataUsageCeilingPercent(metadata map[string]any, key string) float64 {
	if len(metadata) == 0 {
		return 0
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return 0
	}
	var value float64
	switch typed := raw.(type) {
	case float64:
		value = typed
	case float32:
		value = float64(typed)
	case int:
		value = float64(typed)
	case int64:
		value = float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0
		}
		value = parsed
	default:
		return 0
	}
	if value <= 0 || value > 100 {
		return 0
	}
	return value
}

func codexUsageCeilingAllowsAuth(auth *Auth, opts cliproxyexecutor.Options) bool {
	ceilings := codexUsageCeilingsFromOptions(opts)
	if !ceilings.configured() || auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		return true
	}
	now := time.Now()
	if ceilings.fiveHour > 0 {
		if used, ok := codexQuotaUsedPercentForWindow(auth.Quota, codexFiveHourWindowMinutes, now); ok && used >= ceilings.fiveHour {
			return false
		}
	}
	if ceilings.weekly > 0 {
		if used, ok := codexQuotaUsedPercentForWindow(auth.Quota, codexWeeklyWindowMinutes, now); ok && used >= ceilings.weekly {
			return false
		}
	}
	return true
}

func codexQuotaUsedPercentForWindow(quota QuotaState, windowMinutes int, now time.Time) (float64, bool) {
	for _, window := range []string{"Primary", "Secondary"} {
		prefix := "X-Codex-" + window + "-"
		minutes, errMinutes := strconv.Atoi(strings.TrimSpace(quota.Signals[prefix+"Window-Minutes"]))
		if errMinutes != nil || minutes != windowMinutes {
			continue
		}

		if codexQuotaWindowReset(quota, prefix, now) {
			return 0, true
		}

		used, errUsed := strconv.ParseFloat(strings.TrimSpace(quota.Signals[prefix+"Used-Percent"]), 64)
		if errUsed != nil || used < 0 || used > 100 {
			continue
		}
		return used, true
	}
	return 0, false
}

func codexQuotaWindowReset(quota QuotaState, prefix string, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}

	if resetAt, err := strconv.ParseInt(strings.TrimSpace(quota.Signals[prefix+"Reset-At"]), 10, 64); err == nil && resetAt > 0 {
		return !now.Before(time.Unix(resetAt, 0))
	}

	if quota.ObservedAt.IsZero() {
		return false
	}
	resetAfterSeconds, err := strconv.ParseInt(strings.TrimSpace(quota.Signals[prefix+"Reset-After-Seconds"]), 10, 64)
	if err != nil || resetAfterSeconds <= 0 {
		return false
	}
	return !now.Before(quota.ObservedAt.Add(time.Duration(resetAfterSeconds) * time.Second))
}
