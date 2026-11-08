# rocketchat-mcp

An MCP server for Rocket.Chat, written in Go. It has the transport setup,
configuration, example tools, and tests. The tool handlers use gorocket and a
small REST helper in `internal/mcpserver`. Add the tools that your workflow
needs.

The server uses the official
[Go SDK](https://github.com/modelcontextprotocol/go-sdk) and
[gorocket](https://github.com/badkaktus/gorocket).

## Layout

```
cmd/rocketchat-mcp/    entry point, flags, transport setup
internal/config/       environment configuration
internal/mcpserver/    MCP server construction and Rocket.Chat tools
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

The server starts without credentials. The Rocket.Chat tools then return an
error that names the missing variables.

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
| `rocketchat_list_channels` | List public channels and optional private groups. Includes creation, update, and last-message times. |
| `rocketchat_list_dms` | List direct message rooms, most recent first. |
| `rocketchat_list_messages` | List messages with cursor pagination. Supports channels, private groups, and direct messages. |

## Channel time fields

`rocketchat_list_channels` returns these time fields:

| Field | Meaning |
| --- | --- |
| `created_at` | Channel creation time. |
| `updated_at` | Last room update time. |
| `last_message_at` | Time of the last message. Empty for rooms without messages. |

`updated_at` is the room metadata update time, not the last message time. Use
`last_message_at` for activity.

## Direct messages

`rocketchat_list_dms` returns the DM rooms for the authenticated user. Use the
`id` field with `rocketchat_list_messages` to read a conversation. DM messages
use the same `before`, `after`, `limit`, and pagination behavior as channels.

The `usernames` field lists every participant. The `name` field joins those
usernames for display.

## Message pagination

`rocketchat_list_messages` returns one page at a time. The page size is capped
at 100 messages. To read older messages, pass `next_before` from the response
as `before`:

1. Call with `{"room_id": "...", "limit": 100}`.
2. Read `next_before` and `has_more`.
3. Call again with `{"room_id": "...", "limit": 100, "before": "<next_before>"}`.

Use `after` and `before` to set an RFC 3339 time range. The bounds are
exclusive. A one-year backfill is many small pages, not one large request.
Messages at the exact page boundary can have the same timestamp. The server
moves the cursor back by one millisecond to prevent a duplicate. If several
messages share that exact millisecond, the later ones can be skipped.

## Rate limiting

All Rocket.Chat requests use one HTTP client. The client retries HTTP 429
responses up to two times. It honors the `Retry-After` header in seconds or as
an HTTP date. If the server asks for a wait longer than five seconds, the tool
returns an error with the wait time. This stops an MCP call from blocking for a
long time. Without a header, the client uses exponential backoff.

Rocket.Chat rate limits are workspace settings. The server does not set a fixed
client-side request rate. If your workspace needs one, add a limiter with the
limit of that workspace.

## Add a tool

1. Add the tool and its handler with `mcp.AddTool` in `internal/mcpserver`.
2. Call the gorocket method in the handler and map the response to a typed output.
3. For an endpoint that gorocket does not implement, call `api.get`.
4. Add a test in `internal/mcpserver`.

gorocket does not accept a context. Wrap the call with `call(ctx, rocket.Method)`
so the handler stops waiting when the request is canceled.

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
	// Call gorocket and return a typed output value.
})
```

## Development

```sh
make test   # go test ./...
make vet    # go vet ./...
make fmt    # go fmt ./...
```
