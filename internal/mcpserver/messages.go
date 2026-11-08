package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMessageLimit = 50
	maxMessageLimit     = 100
)

type listMessagesArgs struct {
	RoomID string `json:"room_id" jsonschema:"ID of the channel or private group"`
	Limit  int    `json:"limit,omitempty" jsonschema:"messages per page (default 50, max 100)"`
	Before string `json:"before,omitempty" jsonschema:"return messages older than this RFC 3339 time (exclusive). Pass next_before from the previous page."`
	After  string `json:"after,omitempty" jsonschema:"return messages newer than this RFC 3339 time (exclusive)"`
}

type messageInfo struct {
	ID          string              `json:"id" jsonschema:"message ID"`
	Author      string              `json:"author" jsonschema:"author username"`
	AuthorName  string              `json:"author_name,omitempty" jsonschema:"author display name"`
	Text        string              `json:"text" jsonschema:"message text"`
	Timestamp   string              `json:"timestamp" jsonschema:"message time in RFC 3339 format"`
	Attachments []messageAttachment `json:"attachments,omitempty" jsonschema:"message attachments"`
}

type messageAttachment struct {
	Title  string            `json:"title,omitempty" jsonschema:"attachment title"`
	Text   string            `json:"text,omitempty" jsonschema:"attachment text"`
	Fields []attachmentField `json:"fields,omitempty" jsonschema:"attachment fields"`
}

type attachmentField struct {
	Title string `json:"title" jsonschema:"field title"`
	Value string `json:"value" jsonschema:"field value"`
}

type listMessagesOutput struct {
	Messages   []messageInfo `json:"messages" jsonschema:"newest messages first"`
	NextBefore string        `json:"next_before,omitempty" jsonschema:"cursor for the next older page"`
	HasMore    bool          `json:"has_more" jsonschema:"true when this page is full and older messages may exist"`
}

type roomInfoResponse struct {
	Room    roomInfo `json:"room"`
	Success bool     `json:"success"`
}

type roomInfo struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Type string `json:"t"`
}

type historyResponse struct {
	Messages []historyMessage `json:"messages"`
	Success  bool             `json:"success"`
}

type historyMessage struct {
	ID          string              `json:"_id"`
	RoomID      string              `json:"rid"`
	Text        string              `json:"msg"`
	Timestamp   time.Time           `json:"ts"`
	User        messageUser         `json:"u"`
	Attachments []messageAttachment `json:"attachments"`
}

type messageUser struct {
	ID       string `json:"_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func addMessageTools(server *mcp.Server, api *rocketAPI, hasCredentials bool) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "rocketchat_list_messages",
		Title:       "Rocket.Chat: list messages",
		Description: "List messages in a public channel, private group, or direct message, newest first. The page size is capped at 100. Pass next_before from the response as before to page back in time. Use after and before for an exclusive RFC 3339 time range.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listMessagesArgs) (*mcp.CallToolResult, listMessagesOutput, error) {
		if !hasCredentials {
			return nil, listMessagesOutput{}, ErrNoCredentials
		}
		return listMessages(ctx, api, args)
	})
}

func listMessages(ctx context.Context, api *rocketAPI, args listMessagesArgs) (*mcp.CallToolResult, listMessagesOutput, error) {
	if args.RoomID == "" {
		return nil, listMessagesOutput{}, errors.New("room_id is required")
	}
	limit := clampLimit(args.Limit, defaultMessageLimit, maxMessageLimit)

	before, err := parseOptionalTime(args.Before)
	if err != nil {
		return nil, listMessagesOutput{}, fmt.Errorf("before: %w", err)
	}
	after, err := parseOptionalTime(args.After)
	if err != nil {
		return nil, listMessagesOutput{}, fmt.Errorf("after: %w", err)
	}
	if !before.IsZero() && !after.IsZero() && !after.Before(before) {
		return nil, listMessagesOutput{}, errors.New("after must be earlier than before")
	}

	var info roomInfoResponse
	if err := api.get(ctx, "rooms.info", url.Values{"roomId": {args.RoomID}}, &info); err != nil {
		return nil, listMessagesOutput{}, fmt.Errorf("get room %s: %w", args.RoomID, err)
	}
	if !info.Success {
		return nil, listMessagesOutput{}, fmt.Errorf("%w: rooms.info returned success=false", ErrRequestFailed)
	}

	var path string
	switch info.Room.Type {
	case "c":
		path = "channels.history"
	case "p":
		path = "groups.history"
	case "d":
		path = "im.history"
	default:
		return nil, listMessagesOutput{}, fmt.Errorf("room %q has type %q; only channels, private groups, and direct messages are supported", args.RoomID, info.Room.Type)
	}

	query := url.Values{
		"roomId": {args.RoomID},
		"count":  {strconv.Itoa(limit)},
	}
	if !before.IsZero() {
		// Rocket.Chat includes a message that matches latest, even when
		// inclusive=false. Move the bound back by one millisecond to make it
		// exclusive and to prevent a duplicate at the page boundary.
		query.Set("latest", before.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano))
	}
	if !after.IsZero() {
		// Move the bound forward by one millisecond for the same reason.
		query.Set("oldest", after.Add(time.Millisecond).UTC().Format(time.RFC3339Nano))
	}

	var history historyResponse
	if err := api.get(ctx, path, query, &history); err != nil {
		return nil, listMessagesOutput{}, fmt.Errorf("list messages: %w", err)
	}
	if !history.Success {
		return nil, listMessagesOutput{}, fmt.Errorf("%w: %s returned success=false", ErrRequestFailed, path)
	}

	messages := make([]messageInfo, 0, len(history.Messages))
	for _, message := range history.Messages {
		messages = append(messages, messageInfo{
			ID:          message.ID,
			Author:      message.User.Username,
			AuthorName:  message.User.Name,
			Text:        message.Text,
			Timestamp:   message.Timestamp.UTC().Format(time.RFC3339),
			Attachments: message.Attachments,
		})
	}

	output := listMessagesOutput{
		Messages: messages,
		HasMore:  len(messages) == limit,
	}
	if output.HasMore && len(history.Messages) > 0 {
		oldest := history.Messages[len(history.Messages)-1]
		output.NextBefore = oldest.Timestamp.UTC().Format(time.RFC3339Nano)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: messageText(output)}},
	}, output, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use RFC 3339 or YYYY-MM-DD", value)
}

func messageText(output listMessagesOutput) string {
	if len(output.Messages) == 0 {
		return "No messages found."
	}

	var text strings.Builder
	for _, message := range output.Messages {
		fmt.Fprintf(&text, "%s %s: %s", message.Timestamp, message.Author, message.Text)
		for _, attachment := range message.Attachments {
			if attachment.Title != "" {
				fmt.Fprintf(&text, " [%s]", attachment.Title)
			}
		}
		text.WriteString("\n")
	}

	if output.HasMore && output.NextBefore != "" {
		fmt.Fprintf(&text, "\nNext page: use before=%s\n", output.NextBefore)
	}
	return text.String()
}
