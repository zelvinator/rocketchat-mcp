package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrNoCredentials reports that the Rocket.Chat credentials are not configured.
var ErrNoCredentials = errors.New("ROCKETCHAT_USER_ID and ROCKETCHAT_AUTH_TOKEN are required")

// ErrRequestFailed reports that the Rocket.Chat API returned success=false.
var ErrRequestFailed = errors.New("rocketchat: API request failed")

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

func addTools(server *mcp.Server, rocket *gorocketClient, api *rocketAPI, hasCredentials bool) {
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
		if !hasCredentials {
			return nil, meOutput{}, ErrNoCredentials
		}

		response, err := call(ctx, rocket.me)
		if err != nil {
			return nil, meOutput{}, fmt.Errorf("get current Rocket.Chat user: %w", err)
		}
		if response == nil || !response.Success {
			return nil, meOutput{}, fmt.Errorf("%w: GET /api/v1/me returned success=false", ErrRequestFailed)
		}

		text := fmt.Sprintf("Rocket.Chat user %s (%s)", response.Username, response.ID)
		output := meOutput{
			ID:       response.ID,
			Username: response.Username,
			Name:     response.Name,
			Status:   response.Status,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, output, nil
	})

	addChannelTools(server, api, hasCredentials)
	addDMTools(server, api, hasCredentials)
	addMessageTools(server, api, hasCredentials)
}

func ping(context.Context, *mcp.CallToolRequest, emptyArgs) (*mcp.CallToolResult, pingOutput, error) {
	return nil, pingOutput{Status: "pong"}, nil
}

func clampLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
