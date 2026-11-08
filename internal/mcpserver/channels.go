package mcpserver

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultChannelLimit = 50
	maxChannelLimit     = 100
)

type listChannelsArgs struct {
	Limit          int  `json:"limit,omitempty" jsonschema:"maximum number of rooms to return (default 50, max 100)"`
	IncludePrivate bool `json:"include_private,omitempty" jsonschema:"include private groups (default false)"`
}

type channelInfo struct {
	ID            string `json:"id" jsonschema:"room ID"`
	Name          string `json:"name" jsonschema:"room name"`
	Type          string `json:"type" jsonschema:"room type: channel or group"`
	Messages      int    `json:"messages" jsonschema:"message count"`
	Users         int    `json:"users" jsonschema:"member count"`
	ReadOnly      bool   `json:"read_only,omitempty" jsonschema:"true if the room is read-only"`
	CreatedAt     string `json:"created_at,omitempty" jsonschema:"room creation time in RFC 3339 format"`
	UpdatedAt     string `json:"updated_at,omitempty" jsonschema:"last room update time in RFC 3339 format"`
	LastMessageAt string `json:"last_message_at,omitempty" jsonschema:"time of the last message in RFC 3339 format"`
}

type listChannelsOutput struct {
	Channels []channelInfo `json:"channels" jsonschema:"rooms that the user can see"`
}

type roomListResponse struct {
	Channels []roomSummary `json:"channels"`
	Groups   []roomSummary `json:"groups"`
	Success  bool          `json:"success"`
}

type roomSummary struct {
	ID          string    `json:"_id"`
	Name        string    `json:"name"`
	Type        string    `json:"t"`
	Messages    int       `json:"msgs"`
	Users       int       `json:"usersCount"`
	ReadOnly    bool      `json:"ro"`
	CreatedAt   time.Time `json:"ts"`
	UpdatedAt   time.Time `json:"_updatedAt"`
	LastMessage struct {
		Timestamp time.Time `json:"ts"`
	} `json:"lastMessage"`
	LastMessageAt time.Time `json:"lm"`
}

func addChannelTools(server *mcp.Server, api *rocketAPI, hasCredentials bool) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "rocketchat_list_channels",
		Title:       "Rocket.Chat: list channels",
		Description: "List public channels and, when include_private is true, private groups. Returns room IDs for rocketchat_list_messages.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listChannelsArgs) (*mcp.CallToolResult, listChannelsOutput, error) {
		if !hasCredentials {
			return nil, listChannelsOutput{}, ErrNoCredentials
		}
		return listChannels(ctx, api, args)
	})
}

func listChannels(ctx context.Context, api *rocketAPI, args listChannelsArgs) (*mcp.CallToolResult, listChannelsOutput, error) {
	limit := clampLimit(args.Limit, defaultChannelLimit, maxChannelLimit)
	query := url.Values{"count": {strconv.Itoa(limit)}}

	var channelResponse roomListResponse
	if err := api.get(ctx, "channels.list", query, &channelResponse); err != nil {
		return nil, listChannelsOutput{}, fmt.Errorf("list channels: %w", err)
	}
	if !channelResponse.Success {
		return nil, listChannelsOutput{}, fmt.Errorf("%w: channels.list returned success=false", ErrRequestFailed)
	}

	rooms := make([]channelInfo, 0, len(channelResponse.Channels))
	for _, room := range channelResponse.Channels {
		rooms = append(rooms, newChannelInfo(room))
	}

	if args.IncludePrivate {
		var groupResponse roomListResponse
		if err := api.get(ctx, "groups.list", query, &groupResponse); err != nil {
			return nil, listChannelsOutput{}, fmt.Errorf("list private groups: %w", err)
		}
		if !groupResponse.Success {
			return nil, listChannelsOutput{}, fmt.Errorf("%w: groups.list returned success=false", ErrRequestFailed)
		}
		for _, room := range groupResponse.Groups {
			rooms = append(rooms, newChannelInfo(room))
		}
	}

	sort.Slice(rooms, func(i, j int) bool {
		return strings.ToLower(rooms[i].Name) < strings.ToLower(rooms[j].Name)
	})
	if len(rooms) > limit {
		rooms = rooms[:limit]
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: channelText(rooms)}},
	}, listChannelsOutput{Channels: rooms}, nil
}

func newChannelInfo(room roomSummary) channelInfo {
	info := channelInfo{
		ID:        room.ID,
		Name:      room.Name,
		Type:      roomType(room.Type),
		Messages:  room.Messages,
		Users:     room.Users,
		ReadOnly:  room.ReadOnly,
		CreatedAt: formatChannelTime(room.CreatedAt),
		UpdatedAt: formatChannelTime(room.UpdatedAt),
	}

	lastMessageAt := room.LastMessage.Timestamp
	if lastMessageAt.IsZero() {
		lastMessageAt = room.LastMessageAt
	}
	if !lastMessageAt.IsZero() {
		info.LastMessageAt = lastMessageAt.UTC().Format(time.RFC3339)
	}
	return info
}

func formatChannelTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func roomType(value string) string {
	switch value {
	case "c":
		return "channel"
	case "p":
		return "group"
	default:
		return value
	}
}

func channelText(rooms []channelInfo) string {
	if len(rooms) == 0 {
		return "No channels found."
	}

	var text strings.Builder
	for _, room := range rooms {
		fmt.Fprintf(&text, "%s (%s, %d messages", room.Name, room.Type, room.Messages)
		if room.Users > 0 {
			fmt.Fprintf(&text, ", %d users", room.Users)
		}
		if room.ReadOnly {
			text.WriteString(", read-only")
		}
		if room.LastMessageAt != "" {
			fmt.Fprintf(&text, ", last message %s", room.LastMessageAt)
		}
		text.WriteString(")\n")
	}
	return text.String()
}
