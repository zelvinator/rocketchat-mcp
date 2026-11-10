package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMessageLimit = 50
	maxMessageLimit     = 100
)

// usernamePattern is the character set Rocket.Chat's from: filter parses.
// A username outside it would not survive the filter, so it is rejected here
// instead of turning into a full-text search for the literal string.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// fromPattern finds a from: filter anywhere in the query, the same way
// Rocket.Chat's parser does.
var fromPattern = regexp.MustCompile(`(?i)from:[a-z0-9._-]+`)

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

type searchMessagesArgs struct {
	RoomID string `json:"room_id" jsonschema:"ID of the channel, private group, or direct message to search"`
	Query  string `json:"query,omitempty" jsonschema:"Rocket.Chat search text; supports filters such as from:<username>, mention:<username>, before/after/on:DD/MM/YYYY, has:star, has:url, is:pinned, and order:asc|desc. Quote a phrase to match it whole."`
	Author string `json:"author,omitempty" jsonschema:"only messages by this username; shorthand for from:<username>"`
	Limit  int    `json:"limit,omitempty" jsonschema:"results per page (default 50, max 100)"`
	Offset int    `json:"offset,omitempty" jsonschema:"results to skip; pass next_offset from the previous page"`
}

type searchMessagesOutput struct {
	Messages   []messageInfo `json:"messages" jsonschema:"newest matching messages first"`
	NextOffset int           `json:"next_offset,omitempty" jsonschema:"offset for the next result page"`
	HasMore    bool          `json:"has_more" jsonschema:"true when this page is full and more matches may exist"`
}

type searchResponse struct {
	Messages []historyMessage `json:"messages"`
	Success  bool             `json:"success"`
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

	mcp.AddTool(server, &mcp.Tool{
		Name:        "rocketchat_search_messages",
		Title:       "Rocket.Chat: search messages",
		Description: "Search messages in one room, newest first. Search is per room: get the room ID from rocketchat_list_channels or rocketchat_list_dms. query accepts Rocket.Chat search syntax, including the filters from:<username>, mention:<username>, before/after/on:DD/MM/YYYY, has:star, has:url, is:pinned, and order:asc|desc; put a phrase in double quotes to match it whole. author is shorthand for from:<username> and works on its own to list one person's messages. Page with offset and next_offset.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args searchMessagesArgs) (*mcp.CallToolResult, searchMessagesOutput, error) {
		if !hasCredentials {
			return nil, searchMessagesOutput{}, ErrNoCredentials
		}
		return searchMessages(ctx, api, args)
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

	messages := newMessageInfos(history.Messages)

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

func searchMessages(ctx context.Context, api *rocketAPI, args searchMessagesArgs) (*mcp.CallToolResult, searchMessagesOutput, error) {
	if args.RoomID == "" {
		return nil, searchMessagesOutput{}, errors.New("room_id is required")
	}

	query := strings.TrimSpace(args.Query)
	author := strings.TrimSpace(args.Author)
	if author != "" {
		if !usernamePattern.MatchString(author) {
			return nil, searchMessagesOutput{}, fmt.Errorf("author %q is not a valid Rocket.Chat username", author)
		}
		if fromPattern.MatchString(query) {
			return nil, searchMessagesOutput{}, errors.New("query already has a from: filter; use either author or from:<username>, not both")
		}
		if query == "" {
			query = "from:" + author
		} else {
			query += " from:" + author
		}
	}
	if query == "" {
		return nil, searchMessagesOutput{}, errors.New("query or author is required")
	}

	limit := clampLimit(args.Limit, defaultMessageLimit, maxMessageLimit)
	if args.Offset < 0 {
		return nil, searchMessagesOutput{}, errors.New("offset must not be negative")
	}

	// chat.search takes only roomId, searchText, count and offset; anything
	// else is rejected by the route schema, so time bounds and author filtering
	// have to travel inside searchText.
	params := url.Values{
		"roomId":     {args.RoomID},
		"searchText": {query},
		"count":      {strconv.Itoa(limit)},
		"offset":     {strconv.Itoa(args.Offset)},
	}

	var response searchResponse
	if err := api.get(ctx, "chat.search", params, &response); err != nil {
		return nil, searchMessagesOutput{}, fmt.Errorf("search messages: %w", err)
	}
	if !response.Success {
		return nil, searchMessagesOutput{}, fmt.Errorf("%w: chat.search returned success=false", ErrRequestFailed)
	}

	output := searchMessagesOutput{
		Messages: newMessageInfos(response.Messages),
		HasMore:  len(response.Messages) == limit,
	}
	if output.HasMore {
		output.NextOffset = args.Offset + len(response.Messages)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: searchMessageText(output)}},
	}, output, nil
}

func newMessageInfos(messages []historyMessage) []messageInfo {
	converted := make([]messageInfo, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, messageInfo{
			ID:          message.ID,
			Author:      message.User.Username,
			AuthorName:  message.User.Name,
			Text:        message.Text,
			Timestamp:   message.Timestamp.UTC().Format(time.RFC3339),
			Attachments: message.Attachments,
		})
	}
	return converted
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
	writeMessageLines(&text, output.Messages)
	if output.HasMore && output.NextBefore != "" {
		fmt.Fprintf(&text, "\nNext page: use before=%s\n", output.NextBefore)
	}
	return text.String()
}

func searchMessageText(output searchMessagesOutput) string {
	if len(output.Messages) == 0 {
		return "No messages found."
	}

	var text strings.Builder
	writeMessageLines(&text, output.Messages)
	if output.HasMore {
		fmt.Fprintf(&text, "\nNext page: use offset=%d\n", output.NextOffset)
	}
	return text.String()
}

func writeMessageLines(text *strings.Builder, messages []messageInfo) {
	for _, message := range messages {
		fmt.Fprintf(text, "%s %s: %s", message.Timestamp, message.Author, message.Text)
		for _, attachment := range message.Attachments {
			if attachment.Title != "" {
				fmt.Fprintf(text, " [%s]", attachment.Title)
			}
		}
		text.WriteString("\n")
	}
}
