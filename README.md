# rocketchat-mcp

This is an MCP server for Rocket.Chat. It connects to Rocket.Chat workspaces and provides tools to read channels, messages, and user information.

## Quickstart

1. Install the server:

```bash
curl -LO https://github.com/zelvinator/rocketchat-mcp/releases/latest/download/rocketchat-mcp_0.1.0_linux_amd64.tar.gz
tar -xzf rocketchat-mcp_0.1.0_linux_amd64.tar.gz
```

1. Create a personal access token in Rocket.Chat: **Profile > Personal Access Tokens**

2. Configure your MCP client:

```json
{
  "mcpServers": {
    "rocketchat": {
      "command": "/path/to/rocketchat-mcp",
      "env": {
        "ROCKETCHAT_URL": "https://your-workplace.com",
        "ROCKETCHAT_USER_ID": "your-user-id",
        "ROCKETCHAT_AUTH_TOKEN": "your-token"
      }
    }
  }
}
```

You can now use Rocket.Chat tools in your MCP client.

## Tools

| Tool | Description |
| --- | --- |
| `ping` | Check that the server is alive. Returns `pong`. |
| `rocketchat_me` | Return the user that owns the configured credentials. |
| `rocketchat_list_channels` | List public channels and optional private groups. Includes creation, update, and last-message times. |
| `rocketchat_list_dms` | List direct message rooms, most recent first. |
| `rocketchat_list_messages` | List messages with cursor pagination. Supports channels, private groups, and direct messages. |
| `rocketchat_search_messages` | Search one room by text and/or author. Accepts Rocket.Chat filters (`from:`, `mention:`, `before:`, `after:`, `on:`, `has:star`, `is:pinned`, `order:`) and pages with `offset`. |
