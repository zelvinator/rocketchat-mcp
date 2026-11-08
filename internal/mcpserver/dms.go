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
	defaultDMLimit = 50
	maxDMLimit     = 100
)

type listDMsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of direct messages to return (default 50, max 100)"`
}

type dmInfo struct {
	ID            string   `json:"id" jsonschema:"room ID"`
	Name          string   `json:"name" jsonschema:"participant usernames"`
	Usernames     []string `json:"usernames" jsonschema:"DM participant usernames"`
	Messages      int      `json:"messages" jsonschema:"message count"`
	Users         int      `json:"users" jsonschema:"member count"`
	CreatedAt     string   `json:"created_at,omitempty" jsonschema:"room creation time in RFC 3339 format"`
	UpdatedAt     string   `json:"updated_at,omitempty" jsonschema:"last room update time in RFC 3339 format"`
	LastMessageAt string   `json:"last_message_at,omitempty" jsonschema:"time of the last message in RFC 3339 format"`
}

type listDMsOutput struct {
	DMs []dmInfo `json:"dms" jsonschema:"direct message rooms, most recent first"`
}

type dmListResponse struct {
	DMs     []dmSummary `json:"ims"`
	Success bool        `json:"success"`
}

type dmSummary struct {
	ID          string    `json:"_id"`
	Type        string    `json:"t"`
	Usernames   []string  `json:"usernames"`
	Users       int       `json:"usersCount"`
	Messages    int       `json:"msgs"`
	CreatedAt   time.Time `json:"ts"`
	UpdatedAt   time.Time `json:"_updatedAt"`
	LastMessage struct {
		Timestamp time.Time `json:"ts"`
	} `json:"lastMessage"`
}

func addDMTools(server *mcp.Server, api *rocketAPI, hasCredentials bool) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "rocketchat_list_dms",
		Title:       "Rocket.Chat: list direct messages",
		Description: "List direct message rooms for the authenticated user, most recent first. Use the room ID with rocketchat_list_messages to read a conversation.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listDMsArgs) (*mcp.CallToolResult, listDMsOutput, error) {
		if !hasCredentials {
			return nil, listDMsOutput{}, ErrNoCredentials
		}
		return listDMs(ctx, api, args)
	})
}

func listDMs(ctx context.Context, api *rocketAPI, args listDMsArgs) (*mcp.CallToolResult, listDMsOutput, error) {
	limit := clampLimit(args.Limit, defaultDMLimit, maxDMLimit)

	var response dmListResponse
	query := url.Values{
		"count": {strconv.Itoa(maxDMLimit)},
		"sort":  {`{"_updatedAt":-1}`},
	}
	if err := api.get(ctx, "im.list", query, &response); err != nil {
		return nil, listDMsOutput{}, fmt.Errorf("list direct messages: %w", err)
	}
	if !response.Success {
		return nil, listDMsOutput{}, fmt.Errorf("%w: im.list returned success=false", ErrRequestFailed)
	}

	sort.SliceStable(response.DMs, func(i, j int) bool {
		return response.DMs[i].LastMessage.Timestamp.After(response.DMs[j].LastMessage.Timestamp)
	})

	dms := make([]dmInfo, 0, len(response.DMs))
	for _, dm := range response.DMs {
		info := dmInfo{
			ID:        dm.ID,
			Name:      strings.Join(dm.Usernames, ", "),
			Usernames: dm.Usernames,
			Messages:  dm.Messages,
			Users:     dm.Users,
			CreatedAt: formatChannelTime(dm.CreatedAt),
			UpdatedAt: formatChannelTime(dm.UpdatedAt),
		}
		if !dm.LastMessage.Timestamp.IsZero() {
			info.LastMessageAt = dm.LastMessage.Timestamp.UTC().Format(time.RFC3339)
		}
		dms = append(dms, info)
	}

	if len(dms) > limit {
		dms = dms[:limit]
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: dmText(dms)}},
	}, listDMsOutput{DMs: dms}, nil
}

func dmText(dms []dmInfo) string {
	if len(dms) == 0 {
		return "No direct messages found."
	}

	var text strings.Builder
	for _, dm := range dms {
		fmt.Fprintf(&text, "%s (id %s, DM, %d messages", dm.Name, dm.ID, dm.Messages)
		if dm.LastMessageAt != "" {
			fmt.Fprintf(&text, ", last message %s", dm.LastMessageAt)
		}
		text.WriteString(")\n")
	}
	return text.String()
}
