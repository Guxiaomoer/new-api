package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommunityMonitorExtractsAndMasksCandidates(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	config := defaultCommunityMonitorConfig()
	config.RoomURL = "https://93.184.216.34/room"
	state := CommunityMonitorState{}
	body := []byte("hello sk-abcdefghijklmnopqrstuvwxyz and sk-abcdefghijklmnopqrstuvwxyz")

	oldFetch := fetchCommunityMonitorBody
	fetchCommunityMonitorBody = func(rawURL string, config CommunityMonitorConfig) ([]byte, error) {
		return body, nil
	}
	defer func() { fetchCommunityMonitorBody = oldFetch }()

	if err := scanCommunityMonitorLocked(config, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(state.Results))
	}
	if state.Results[0].Value == state.Results[0].MaskedValue {
		t.Fatal("expected masked value to differ from raw value")
	}
	if state.Results[0].Fingerprint == "" {
		t.Fatal("expected fingerprint")
	}
	publicResults := publicCommunityMonitorResults(state.Results)
	if publicResults[0].Value != "" {
		t.Fatal("public results must not include raw secret")
	}
}

func TestCommunityMonitorConfigPersistencePreservesSecretWhenOmitted(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	first := defaultCommunityMonitorConfig()
	first.RoomURL = "https://93.184.216.34/room"
	first.AccessToken = "secret-token"
	if _, err := SaveCommunityMonitorConfig(first); err != nil {
		t.Fatal(err)
	}
	second := defaultCommunityMonitorConfig()
	second.RoomURL = "https://93.184.216.34/room2"
	if _, err := SaveCommunityMonitorConfig(second); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadCommunityMonitorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != "secret-token" {
		t.Fatal("expected omitted access token to be preserved")
	}
	if _, err := os.Stat(filepath.Join("data", "community-monitor", "config.json")); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutboundURLBlocksPrivateAddress(t *testing.T) {
	if err := validateOutboundURL("http://127.0.0.1:8080"); err == nil {
		t.Fatal("expected loopback URL to be blocked")
	}
}
