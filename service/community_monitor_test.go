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

func TestSharkeyChatRoomScan(t *testing.T) {
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
	config.SourceURL = "https://93.184.216.34"
	config.RoomID = "testroom"
	config.AccessToken = "test-token"
	config.Query = "sk-"
	config.ExtractRegex = `sk-[A-Za-z0-9_-]{8,}`
	state := CommunityMonitorState{}

	oldImpl := fetchSharkeyChatMessagesImpl
	fetchSharkeyChatMessagesImpl = func(apiURL, accessToken, roomID string, limit int, sinceID string) ([]sharkeyChatMessage, error) {
		return []sharkeyChatMessage{
			{ID: "msg1", CreatedAt: "2026-06-10T00:00:00Z", Text: "hello sk-abcdefghijklmnopqrstuvwxyz and sk-abcdefghijklmnopqrstuvwxyz", FromUserID: "user1", ToRoomID: "testroom"},
			{ID: "msg2", CreatedAt: "2026-06-10T00:01:00Z", Text: "another key sk-BIAzRn6jCszMRuF4RM6chp608jJkxfqL here", FromUserID: "user2", ToRoomID: "testroom"},
			{ID: "msg3", CreatedAt: "2026-06-10T00:02:00Z", Text: "no key here", FromUserID: "user3", ToRoomID: "testroom"},
		}, nil
	}
	defer func() { fetchSharkeyChatMessagesImpl = oldImpl }()

	if err := scanCommunityMonitorLocked(config, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(state.Results))
	}
	if state.LastMessageID != "msg1" {
		t.Fatalf("expected lastMessageID to be 'msg1', got '%s'", state.LastMessageID)
	}
	if state.Progress.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", state.Progress.Hits)
	}
	if state.Progress.Read != 3 {
		t.Fatalf("expected 3 messages read, got %d", state.Progress.Read)
	}
}
