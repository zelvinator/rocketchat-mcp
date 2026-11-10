// Package mcpserver builds the MCP server and registers its tools.
package mcpserver

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/badkaktus/gorocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// requestTimeout limits one call to the Rocket.Chat API.
const requestTimeout = 30 * time.Second

// Options configures the MCP server.
type Options struct {
	// RocketChatURL is the base URL of the Rocket.Chat workspace.
	RocketChatURL string
	// UserID is the ID of the user that owns AuthToken.
	UserID string
	// AuthToken is a Rocket.Chat personal access token.
	AuthToken string
	// Version is reported to clients in the server metadata.
	Version string
	// Logger receives server logs. A nil logger discards them.
	Logger *slog.Logger
}

// New returns an MCP server with all Rocket.Chat tools registered.
func New(opts Options) *mcp.Server {
	if opts.Version == "" {
		opts.Version = "dev"
	}

	httpClient := newHTTPClient()

	// gorocket uses package-level pagination state, so serialize its calls.
	rocket := &gorocketClient{
		client: gorocket.NewWithOptions(
			strings.TrimRight(opts.RocketChatURL, "/"),
			gorocket.WithUserID(opts.UserID),
			gorocket.WithXToken(opts.AuthToken),
			gorocket.WithTimeout(requestTimeout),
		),
	}
	rocket.client.HTTPClient = httpClient

	api := &rocketAPI{
		baseURL:   strings.TrimRight(opts.RocketChatURL, "/"),
		userID:    opts.UserID,
		authToken: opts.AuthToken,
		client:    httpClient,
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "rocketchat-mcp",
		Title:   "Rocket.Chat MCP",
		Version: opts.Version,
	}, &mcp.ServerOptions{
		Instructions: "Tools for a Rocket.Chat workspace. Use rocketchat_list_channels and rocketchat_list_dms to find room IDs, then rocketchat_list_messages to read a room or rocketchat_search_messages to search inside one. Call rocketchat_me to check the configured credentials.",
		Logger:       opts.Logger,
	})

	addTools(server, rocket, api, opts.UserID != "" && opts.AuthToken != "")
	return server
}

// gorocketClient wraps the gorocket client with a mutex.
type gorocketClient struct {
	mu     sync.Mutex
	client *gorocket.Client
}

func (c *gorocketClient) me() (*gorocket.MeResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.Me()
}
