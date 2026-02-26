package google

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailMessage struct {
	ID             string
	ThreadID       string
	Subject        string
	From           string
	Snippet        string
	Body           string
	Labels         []string
	HasUnsubscribe bool
	GmailCategory  string
	Date           time.Time
	Attachments    []string
}

type MinimalMessage struct {
	ID     string
	Labels []string
}

type GmailClient struct {
	service *gmail.Service
	userID  string
}

func NewGmailClient(ctx context.Context, tokenSource oauth2.TokenSource) (*GmailClient, error) {
	srv, err := gmail.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &GmailClient{service: srv, userID: "me"}, nil
}

func (c *GmailClient) ListUnread(ctx context.Context, maxResults int) ([]GmailMessage, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	resp, err := c.service.Users.Messages.List(c.userID).
		Q("in:inbox is:unread").
		MaxResults(int64(maxResults)).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("list unread: %w", err)
	}

	messages := make([]GmailMessage, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		msg, err := c.GetMessage(ctx, m.Id)
		if err != nil {
			continue
		}
		messages = append(messages, *msg)
	}
	return messages, nil
}

func (c *GmailClient) GetMessage(ctx context.Context, messageID string) (*GmailMessage, error) {
	m, err := c.service.Users.Messages.Get(c.userID, messageID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("get message %s: %w", messageID, err)
	}
	return parseGmailMessage(m), nil
}

func (c *GmailClient) GetThread(ctx context.Context, threadID string) ([]GmailMessage, error) {
	t, err := c.service.Users.Threads.Get(c.userID, threadID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("get thread %s: %w", threadID, err)
	}
	messages := make([]GmailMessage, 0, len(t.Messages))
	for _, m := range t.Messages {
		messages = append(messages, *parseGmailMessage(m))
	}
	return messages, nil
}

func (c *GmailClient) ListLabels(ctx context.Context) (map[string]string, error) {
	resp, err := c.service.Users.Labels.List(c.userID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	labelMap := make(map[string]string, len(resp.Labels))
	for _, l := range resp.Labels {
		labelMap[l.Name] = l.Id
	}
	return labelMap, nil
}

func (c *GmailClient) ApplyLabels(ctx context.Context, messageID string, addLabelIDs, removeLabelIDs []string) error {
	_, err := c.service.Users.Messages.Modify(c.userID, messageID, &gmail.ModifyMessageRequest{
		AddLabelIds:    addLabelIDs,
		RemoveLabelIds: removeLabelIDs,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("apply labels to %s: %w", messageID, err)
	}
	return nil
}

func (c *GmailClient) ArchiveMessage(ctx context.Context, messageID string) error {
	return c.ApplyLabels(ctx, messageID, nil, []string{"INBOX"})
}

func (c *GmailClient) TrashMessage(ctx context.Context, messageID string) error {
	_, err := c.service.Users.Messages.Trash(c.userID, messageID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("trash message %s: %w", messageID, err)
	}
	return nil
}

func (c *GmailClient) MarkAsRead(ctx context.Context, messageID string) error {
	return c.ApplyLabels(ctx, messageID, nil, []string{"UNREAD"})
}

func (c *GmailClient) GetThreadMinimal(ctx context.Context, threadID string) ([]MinimalMessage, error) {
	t, err := c.service.Users.Threads.Get(c.userID, threadID).
		Format("minimal").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("get thread minimal %s: %w", threadID, err)
	}
	msgs := make([]MinimalMessage, 0, len(t.Messages))
	for _, m := range t.Messages {
		msgs = append(msgs, MinimalMessage{
			ID:     m.Id,
			Labels: m.LabelIds,
		})
	}
	return msgs, nil
}

func (c *GmailClient) SearchMessages(ctx context.Context, query string, maxResults int) ([]GmailMessage, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	resp, err := c.service.Users.Messages.List(c.userID).
		Q(query).
		MaxResults(int64(maxResults)).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	messages := make([]GmailMessage, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		msg, err := c.GetMessage(ctx, m.Id)
		if err != nil {
			continue
		}
		messages = append(messages, *msg)
	}
	return messages, nil
}

func (c *GmailClient) ListInbox(ctx context.Context, maxResults int) ([]GmailMessage, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	resp, err := c.service.Users.Messages.List(c.userID).
		Q("in:inbox").
		MaxResults(int64(maxResults)).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	messages := make([]GmailMessage, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		msg, err := c.GetMessage(ctx, m.Id)
		if err != nil {
			continue
		}
		messages = append(messages, *msg)
	}
	return messages, nil
}

func GmailPermalink(messageID string) string {
	return "https://mail.google.com/mail/u/0/#all/" + messageID
}

func parseGmailMessage(m *gmail.Message) *GmailMessage {
	msg := &GmailMessage{
		ID:       m.Id,
		ThreadID: m.ThreadId,
		Snippet:  m.Snippet,
		Labels:   m.LabelIds,
	}

	if m.Payload != nil {
		for _, h := range m.Payload.Headers {
			switch strings.ToLower(h.Name) {
			case "subject":
				msg.Subject = h.Value
			case "from":
				msg.From = h.Value
			case "date":
				if t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", h.Value); err == nil {
					msg.Date = t
				} else if t, err := time.Parse(time.RFC1123Z, h.Value); err == nil {
					msg.Date = t
				}
			case "list-unsubscribe":
				msg.HasUnsubscribe = true
			}
		}
		msg.Body = extractPlainText(m.Payload, 500)
		msg.Attachments = extractAttachmentNames(m.Payload)
	}

	for _, label := range m.LabelIds {
		if strings.HasPrefix(label, "CATEGORY_") {
			msg.GmailCategory = label
		}
	}

	return msg
}

func extractPlainText(payload *gmail.MessagePart, limit int) string {
	if payload == nil {
		return ""
	}

	if payload.MimeType == "text/plain" && payload.Body != nil && payload.Body.Data != "" {
		decoded, err := decodeBase64URL(payload.Body.Data)
		if err == nil {
			text := strings.TrimSpace(string(decoded))
			if len(text) > limit {
				return text[:limit]
			}
			return text
		}
	}

	for _, part := range payload.Parts {
		if text := extractPlainText(part, limit); text != "" {
			return text
		}
	}

	return ""
}

func extractAttachmentNames(payload *gmail.MessagePart) []string {
	if payload == nil {
		return nil
	}
	var names []string
	for _, part := range payload.Parts {
		if part.Filename != "" {
			names = append(names, part.Filename)
		}
		names = append(names, extractAttachmentNames(part)...)
	}
	return names
}

func decodeBase64URL(data string) ([]byte, error) {
	if decoded, err := base64.RawURLEncoding.DecodeString(data); err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(data)
}
