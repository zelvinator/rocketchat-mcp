// Package mcpserver builds the MCP server and registers its tools.
package mcpserver

import (
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.srv.hnatekmar.dev/hnatekmar/rocketchat-mcp/internal/rocketchat"
)

// Options configures the MCP server.
type Options struct {
	// Version is reported to clients in the server metadata.
	Version string
	// Logger receives server logs. A nil logger discards them.
	Logger *slog.Logger
}

// New returns an MCP server with all Rocket.Chat tools registered.
func New(rc *rocketchat.Client, opts Options) *mcp.Server {
	if opts.Version == "" {
		opts.Version = "dev"
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "rocketchat-mcp",
		Title:   "Rocket.Chat MCP",
		Version: opts.Version,
	}, &mcp.ServerOptions{
		Instructions: "Tools for a Rocket.Chat workspace. Call rocketchat_me to check the configured credentials.",
		Logger:       opts.Logger,
	})

	addTools(server, rc)
	return server
}
