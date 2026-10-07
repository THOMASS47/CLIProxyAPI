package configaccess

import (
	"context"
	"net/http"
	"testing"

	sdkconfig "github.com/router-for-me/CLIProxyAPI/v8/sdk/config"
)

func TestProviderAttachesCodexUsageCeilingsForMatchingKey(t *testing.T) {
	p := newProvider("test", []string{"limited", "unlimited"}, map[string]sdkconfig.CodexUsageCeiling{
		"limited": {
			FiveHourPercent: 95,
			WeeklyPercent:   98,
		},
	})

	req, err := http.NewRequest(http.MethodPost, "http://localhost/v1/responses", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer limited")

	result, authErr := p.Authenticate(context.Background(), req)
	if authErr != nil {
		t.Fatalf("Authenticate() error = %v", authErr)
	}
	if got := result.Metadata[codexFiveHourUsageCeilingMetadataKey]; got != "95" {
		t.Fatalf("5-hour ceiling metadata = %q, want %q", got, "95")
	}
	if got := result.Metadata[codexWeeklyUsageCeilingMetadataKey]; got != "98" {
		t.Fatalf("weekly ceiling metadata = %q, want %q", got, "98")
	}
}

func TestProviderDoesNotAttachCeilingsToOtherKeys(t *testing.T) {
	p := newProvider("test", []string{"limited", "unlimited"}, map[string]sdkconfig.CodexUsageCeiling{
		"limited": {
			FiveHourPercent: 95,
			WeeklyPercent:   98,
		},
	})

	req, err := http.NewRequest(http.MethodPost, "http://localhost/v1/responses", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer unlimited")

	result, authErr := p.Authenticate(context.Background(), req)
	if authErr != nil {
		t.Fatalf("Authenticate() error = %v", authErr)
	}
	if _, ok := result.Metadata[codexFiveHourUsageCeilingMetadataKey]; ok {
		t.Fatal("unexpected 5-hour ceiling metadata on unlimited key")
	}
	if _, ok := result.Metadata[codexWeeklyUsageCeilingMetadataKey]; ok {
		t.Fatal("unexpected weekly ceiling metadata on unlimited key")
	}
}
