// Package config reads server configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const defaultURL = "http://localhost:3000"

// Config holds the runtime settings of the server.
type Config struct {
	// RocketChatURL is the base URL of the Rocket.Chat workspace.
	RocketChatURL string
	// UserID is the ID of the user that owns the access token.
	UserID string
	// AuthToken is a Rocket.Chat personal access token.
	AuthToken string
}

// Load reads configuration from the environment.
//
// The server starts without credentials. Tools that call Rocket.Chat then
// return an error until ROCKETCHAT_USER_ID and ROCKETCHAT_AUTH_TOKEN are set.
func Load() (Config, error) {
	cfg := Config{
		RocketChatURL: strings.TrimRight(env("ROCKETCHAT_URL", defaultURL), "/"),
		UserID:        os.Getenv("ROCKETCHAT_USER_ID"),
		AuthToken:     os.Getenv("ROCKETCHAT_AUTH_TOKEN"),
	}

	u, err := url.Parse(cfg.RocketChatURL)
	if err != nil {
		return Config{}, fmt.Errorf("ROCKETCHAT_URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Config{}, fmt.Errorf("ROCKETCHAT_URL: unsupported scheme %q: use http or https", u.Scheme)
	}
	if u.Host == "" {
		return Config{}, fmt.Errorf("ROCKETCHAT_URL: missing host in %q", cfg.RocketChatURL)
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
