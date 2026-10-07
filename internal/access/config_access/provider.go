package configaccess

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v8/sdk/access"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v8/sdk/config"
)

// Register ensures the config-access provider is available to the access manager.
func Register(cfg *sdkconfig.SDKConfig) {
	if cfg == nil {
		sdkaccess.UnregisterProvider(sdkaccess.AccessProviderTypeConfigAPIKey)
		return
	}

	keys := normalizeKeys(cfg.APIKeys)
	if len(keys) == 0 {
		sdkaccess.UnregisterProvider(sdkaccess.AccessProviderTypeConfigAPIKey)
		return
	}

	sdkaccess.RegisterProvider(
		sdkaccess.AccessProviderTypeConfigAPIKey,
		newProvider(sdkaccess.DefaultAccessProviderName, keys, cfg.CodexUsageCeilings),
	)
}

const (
	codexFiveHourUsageLimitMetadataKey = "codex_5h_usage_limit_percent"
	codexWeeklyUsageLimitMetadataKey   = "codex_weekly_usage_limit_percent"
)

type provider struct {
	name   string
	keys   map[string]struct{}
	ceilings map[string]sdkconfig.CodexUsageCeiling
}

func newProvider(name string, keys []string, ceilings map[string]sdkconfig.CodexUsageCeiling) *provider {
	providerName := strings.TrimSpace(name)
	if providerName == "" {
		providerName = sdkaccess.DefaultAccessProviderName
	}
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	ceilingSet := make(map[string]sdkconfig.CodexUsageCeiling)
	for key, ceiling := range ceilings {
		key = strings.TrimSpace(key)
		if _, ok := keySet[key]; !ok {
			continue
		}
		if validUsageLimitPercent(ceiling.FiveHourPercent) || validUsageLimitPercent(ceiling.WeeklyPercent) {
			ceilingSet[key] = ceiling
		}
	}
	return &provider{name: providerName, keys: keySet, ceilings: ceilingSet}
}

func (p *provider) Identifier() string {
	if p == nil || p.name == "" {
		return sdkaccess.DefaultAccessProviderName
	}
	return p.name
}

func (p *provider) Authenticate(_ context.Context, r *http.Request) (*sdkaccess.Result, *sdkaccess.AuthError) {
	if p == nil {
		return nil, sdkaccess.NewNotHandledError()
	}
	if len(p.keys) == 0 {
		return nil, sdkaccess.NewNotHandledError()
	}
	authHeader := r.Header.Get("Authorization")
	authHeaderGoogle := r.Header.Get("X-Goog-Api-Key")
	authHeaderAnthropic := r.Header.Get("X-Api-Key")
	queryKey := ""
	queryAuthToken := ""
	if r.URL != nil {
		queryKey = r.URL.Query().Get("key")
		queryAuthToken = r.URL.Query().Get("auth_token")
	}
	if authHeader == "" && authHeaderGoogle == "" && authHeaderAnthropic == "" && queryKey == "" && queryAuthToken == "" {
		return nil, sdkaccess.NewNoCredentialsError()
	}

	apiKey := extractBearerToken(authHeader)

	candidates := []struct {
		value  string
		source string
	}{
		{apiKey, "authorization"},
		{authHeaderGoogle, "x-goog-api-key"},
		{authHeaderAnthropic, "x-api-key"},
		{queryKey, "query-key"},
		{queryAuthToken, "query-auth-token"},
	}

	for _, candidate := range candidates {
		if candidate.value == "" {
			continue
		}
		if _, ok := p.keys[candidate.value]; ok {
			metadata := map[string]string{"source": candidate.source}
			if ceiling, exists := p.ceilings[candidate.value]; exists {
				if validUsageLimitPercent(ceiling.FiveHourPercent) {
					metadata[codexFiveHourUsageLimitMetadataKey] = strconv.FormatFloat(ceiling.FiveHourPercent, 'f', -1, 64)
				}
				if validUsageLimitPercent(ceiling.WeeklyPercent) {
					metadata[codexWeeklyUsageLimitMetadataKey] = strconv.FormatFloat(ceiling.WeeklyPercent, 'f', -1, 64)
				}
			}
			return &sdkaccess.Result{
				Provider:  p.Identifier(),
				Principal: candidate.value,
				Metadata: metadata,
			}, nil
		}
	}

	return nil, sdkaccess.NewInvalidCredentialError()
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return header
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return header
	}
	return strings.TrimSpace(parts[1])
}

func normalizeKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, exists := seen[trimmedKey]; exists {
			continue
		}
		seen[trimmedKey] = struct{}{}
		normalized = append(normalized, trimmedKey)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func validUsageLimitPercent(value float64) bool {
	return value > 0 && value <= 100
}
