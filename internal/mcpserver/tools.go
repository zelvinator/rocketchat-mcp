package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.srv.hnatekmar.dev/hnatekmar/rocketchat-mcp/internal/rocketchat"
)

// emptyArgs marks a tool that takes no arguments.
type emptyArgs struct{}

// pingOutput is the structured result of the ping tool.
type pingOutput struct {
	Status string `json:"status" jsonschema:"always pong"`
}

// meOutput is the structured result of the rocketchat_me tool.
type meOutput struct {
	ID       string `json:"id" jsonschema:"Rocket.Chat user ID"`
	Username string `json:"username" jsonschema:"Rocket.Chat username"`
	Name     string `json:"name,omitempty" jsonschema:"display name"`
	Status   string `json:"status,omitempty" jsonschema:"presence status"`
}

func addTools(server *mcp.Server, rc *rocketchat.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ping",
		Title:       "Ping",
		Description: "Check that the MCP server is alive.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, ping)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "rocketchat_me",
		Title:       "Rocket.Chat: current user",
		Description: "Get the Rocket.Chat user that owns the configured credentials. Use this tool to test the credentials.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, meOutput, error) {
		user, err := rc.Me(ctx)
		if err != nil {
			return nil, meOutput{}, fmt.Errorf("get current Rocket.Chat user: %w", err)
		}

		text := fmt.Sprintf("Rocket.Chat user %s (%s)", user.Username, user.ID)
		output := meOutput{
			ID:       user.ID,
			Username: user.Username,
			Name:     user.Name,
			Status:   user.Status,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, output, nil
	})
}

func ping(context.Context, *mcp.CallToolRequest, emptyArgs) (*mcp.CallToolResult, pingOutput, error) {
	return nil, pingOutput{Status: "pong"}, nil
}
