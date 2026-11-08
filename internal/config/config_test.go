package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ROCKETCHAT_URL", "")
	t.Setenv("ROCKETCHAT_USER_ID", "")
	t.Setenv("ROCKETCHAT_AUTH_TOKEN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RocketChatURL != defaultURL {
		t.Errorf("RocketChatURL = %q, want %q", cfg.RocketChatURL, defaultURL)
	}
	if cfg.UserID != "" || cfg.AuthToken != "" {
		t.Errorf("credentials = %q/%q, want empty", cfg.UserID, cfg.AuthToken)
	}
}

func TestLoadRejectsInvalidURL(t *testing.T) {
	for _, rawURL := range []string{"localhost:3000", "ftp://chat.example.com", "http://"} {
		t.Setenv("ROCKETCHAT_URL", rawURL)
		if _, err := Load(); err == nil {
			t.Errorf("Load with ROCKETCHAT_URL=%q returned no error", rawURL)
		}
	}
}
