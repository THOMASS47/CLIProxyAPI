package auth

import (
	"strconv"
	"strings"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

const (
	codexFiveHourWindowMinutes = 5 * 60
	codexWeeklyWindowMinutes   = 7 * 24 * 60
)

type codexUsageLimits struct {
	fiveHour float64
	weekly   float64
}

func codexUsageLimitsFromOptions(opts cliproxyexecutor.Options) codexUsageLimits {
	return codexUsageLimits{
		fiveHour: metadataUsageLimitPercent(opts.Metadata, cliproxyexecutor.CodexFiveHourUsageLimitPercentMetadataKey),
		weekly:   metadataUsageLimitPercent(opts.Metadata, cliproxyexecutor.CodexWeeklyUsageLimitPercentMetadataKey),
	}
}

func (l codexUsageLimits) configured() bool {
	return l.fiveHour > 0 || l.weekly > 0
}

func metadataUsageLimitPercent(metadata map[string]any, key string) float64 {
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

func codexUsageLimitAllowsAuth(auth *Auth, opts cliproxyexecutor.Options) bool {
	limits := codexUsageLimitsFromOptions(opts)
	if !limits.configured() || auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		return true
	}
	if limits.fiveHour > 0 {
		if used, ok := codexQuotaUsedPercentForWindow(auth.Quota.Signals, codexFiveHourWindowMinutes); ok && used >= limits.fiveHour {
			return false
		}
	}
	if limits.weekly > 0 {
		if used, ok := codexQuotaUsedPercentForWindow(auth.Quota.Signals, codexWeeklyWindowMinutes); ok && used >= limits.weekly {
			return false
		}
	}
	return true
}

func codexQuotaUsedPercentForWindow(signals map[string]string, windowMinutes int) (float64, bool) {
	for _, window := range []string{"Primary", "Secondary"} {
		prefix := "X-Codex-" + window + "-"
		minutes, errMinutes := strconv.Atoi(strings.TrimSpace(signals[prefix+"Window-Minutes"]))
		if errMinutes != nil || minutes != windowMinutes {
			continue
		}
		used, errUsed := strconv.ParseFloat(strings.TrimSpace(signals[prefix+"Used-Percent"]), 64)
		if errUsed != nil || used < 0 || used > 100 {
			continue
		}
		return used, true
	}
	return 0, false
}
