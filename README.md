# rocketchat-mcp

An MCP server for Rocket.Chat, written in Go. This repository is a skeleton.
It has the transport setup, configuration, a Rocket.Chat API client, two
example tools, and tests. Add the tools that your workflow needs.

The server uses the official
[Go SDK](https://github.com/modelcontextprotocol/go-sdk).

## Layout

```
cmd/rocketchat-mcp/    entry point, flags, transport setup
internal/config/       environment configuration
internal/mcpserver/    MCP server construction and tool registration
internal/rocketchat/   Rocket.Chat REST API client
```

## Requirements

- Go 1.25 or later

## Build and run

```sh
make build
export ROCKETCHAT_URL=https://chat.example.com
export ROCKETCHAT_USER_ID=your-user-id
export ROCKETCHAT_AUTH_TOKEN=your-personal-access-token
./bin/rocketchat-mcp
```

The server uses stdio by default. It writes logs to stderr, so stdout stays
clean for the MCP protocol.

To serve streamable HTTP:

```sh
make run-http
# MCP endpoint: http://localhost:8080/mcp
# Health check: http://localhost:8080/healthz
```

The HTTP handler is stateless. Each request gets a temporary MCP session.

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-transport` | `stdio` | Transport to use: `stdio` or `http` |
| `-addr` | `:8080` | Listen address for `-transport=http` |
| `-log-level` | `info` | `debug`, `info`, `warn`, or `error` |
| `-version` | | Print the version and exit |

### Environment

| Variable | Default | Description |
| --- | --- | --- |
| `ROCKETCHAT_URL` | `http://localhost:3000` | Base URL of the workspace |
| `ROCKETCHAT_USER_ID` | | ID of the user that owns the token |
| `ROCKETCHAT_AUTH_TOKEN` | | Rocket.Chat personal access token |

The server starts without credentials. `rocketchat_me` then returns an error
that tells the user which variables are missing.

Create a personal access token in Rocket.Chat under **Profile > Personal
Access Tokens**.

## Client configuration

Example for an MCP client that starts the server as a subprocess:

```json
{
  "mcpServers": {
    "rocketchat": {
      "command": "/absolute/path/to/bin/rocketchat-mcp",
      "env": {
        "ROCKETCHAT_URL": "https://chat.example.com",
        "ROCKETCHAT_USER_ID": "your-user-id",
        "ROCKETCHAT_AUTH_TOKEN": "your-personal-access-token"
      }
    }
  }
}
```

## Tools

| Tool | Description |
| --- | --- |
| `ping` | Check that the server is alive. Returns `pong`. |
| `rocketchat_me` | Return the user that owns the configured credentials. |

## Add a tool

1. Add a method to `internal/rocketchat/client.go`.
2. Register the tool with `mcp.AddTool` in `internal/mcpserver/tools.go`.
3. Add a test in `internal/mcpserver/server_test.go`.

Example registration:

```go
type channelArgs struct {
	RoomID string `json:"room_id" jsonschema:"ID of the channel"`
}

type channelOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

mcp.AddTool(server, &mcp.Tool{
	Name:        "rocketchat_channel",
	Description: "Get one channel by ID.",
	Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
}, func(ctx context.Context, _ *mcp.CallToolRequest, args channelArgs) (*mcp.CallToolResult, channelOutput, error) {
	// Call the Rocket.Chat client and return a typed output value.
})
```

## Development

```sh
make test   # go test ./...
make vet    # go vet ./...
make fmt    # go fmt ./...
```
