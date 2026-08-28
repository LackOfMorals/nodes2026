// Package slackclient provides a thin wrapper over slack-go/slack exposing
// only the methods this subgraph needs. The wrapper exists for three reasons:
//
//   - It defines a small Client interface so the service layer can be tested
//     with a mock.
//   - It centralizes error wrapping so the service layer sees domain errors,
//     not raw Slack API errors.
//   - It gives us a place to add caching, retries, or instrumentation later
//     without touching the service layer.
package slackclient

import (
	"context"
	"errors"
	"fmt"

	slackgo "github.com/slack-go/slack"
)

// ErrNotFound is returned when a channel, message, or user does not exist
// or the bot does not have permission to see it.
var ErrNotFound = errors.New("slack: resource not found")

// Channel is the subgraph-facing representation of a Slack channel.
// It contains only the fields the GraphQL schema exposes.
type Channel struct {
	ID          string
	Name        string
	Topic       string
	Purpose     string
	MemberCount int
	IsPrivate   bool
}

// Message is the subgraph-facing representation of a Slack message.
type Message struct {
	ChannelID string
	TS        string
	UserID    string
	Text      string
	ThreadTS  string
	Permalink string
}

// User is the subgraph-facing representation of a Slack user.
type User struct {
	ID       string
	Name     string
	RealName string
	Email    string
	IsBot    bool
}

// Client is the contract the service layer depends on. A real implementation
// wraps slack-go; tests can supply a mock.
type Client interface {
	GetChannel(ctx context.Context, id string) (*Channel, error)
	GetMessages(ctx context.Context, channelID string, limit int) ([]Message, error)
	GetThread(ctx context.Context, channelID, threadTS string) ([]Message, error)
	GetUser(ctx context.Context, id string) (*User, error)
}

// client is the production Client backed by slack-go.
type client struct {
	api *slackgo.Client
}

// NewClient returns a Client backed by the Slack Web API.
// The token must be a bot user OAuth token (xoxb-...).
func NewClient(token string) Client {
	return &client{
		api: slackgo.New(token),
	}
}

// GetChannel returns channel metadata. Returns ErrNotFound if the channel
// does not exist or the bot cannot see it.
func (c *client) GetChannel(ctx context.Context, id string) (*Channel, error) {
	info, err := c.api.GetConversationInfoContext(ctx, &slackgo.GetConversationInfoInput{
		ChannelID:         id,
		IncludeLocale:     false,
		IncludeNumMembers: true,
	})
	if err != nil {
		if isSlackNotFound(err) {
			return nil, fmt.Errorf("get channel %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get channel %s: %w", id, err)
	}

	return &Channel{
		ID:          info.ID,
		Name:        info.Name,
		Topic:       info.Topic.Value,
		Purpose:     info.Purpose.Value,
		MemberCount: info.NumMembers,
		IsPrivate:   info.IsPrivate,
	}, nil
}

// GetMessages returns the most recent messages in a channel.
// limit is clamped between 1 and 100.
func (c *client) GetMessages(ctx context.Context, channelID string, limit int) ([]Message, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	history, err := c.api.GetConversationHistoryContext(ctx, &slackgo.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     limit,
	})
	if err != nil {
		if isSlackNotFound(err) {
			return nil, fmt.Errorf("get messages %s: %w", channelID, ErrNotFound)
		}
		return nil, fmt.Errorf("get messages %s: %w", channelID, err)
	}

	messages := make([]Message, 0, len(history.Messages))
	for _, m := range history.Messages {
		messages = append(messages, c.toMessage(ctx, channelID, m))
	}
	return messages, nil
}

// GetThread returns all replies in a thread, including the parent message.
func (c *client) GetThread(ctx context.Context, channelID, threadTS string) ([]Message, error) {
	replies, _, _, err := c.api.GetConversationRepliesContext(ctx, &slackgo.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
	})
	if err != nil {
		if isSlackNotFound(err) {
			return nil, fmt.Errorf("get thread %s/%s: %w", channelID, threadTS, ErrNotFound)
		}
		return nil, fmt.Errorf("get thread %s/%s: %w", channelID, threadTS, err)
	}

	messages := make([]Message, 0, len(replies))
	for _, m := range replies {
		messages = append(messages, c.toMessage(ctx, channelID, m))
	}
	return messages, nil
}

// GetUser returns user details. Returns ErrNotFound if the user does not exist.
func (c *client) GetUser(ctx context.Context, id string) (*User, error) {
	info, err := c.api.GetUserInfoContext(ctx, id)
	if err != nil {
		if isSlackNotFound(err) {
			return nil, fmt.Errorf("get user %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}

	return &User{
		ID:       info.ID,
		Name:     info.Name,
		RealName: info.RealName,
		Email:    info.Profile.Email,
		IsBot:    info.IsBot,
	}, nil
}

// toMessage builds a Message from a slack-go message, fetching the permalink.
// Permalink fetch failures are non-fatal — the message is returned without one.
func (c *client) toMessage(ctx context.Context, channelID string, m slackgo.Message) Message {
	msg := Message{
		ChannelID: channelID,
		TS:        m.Timestamp,
		UserID:    m.User,
		Text:      m.Text,
		ThreadTS:  m.ThreadTimestamp,
	}

	permalink, err := c.api.GetPermalinkContext(ctx, &slackgo.PermalinkParameters{
		Channel: channelID,
		Ts:      m.Timestamp,
	})
	if err == nil {
		msg.Permalink = permalink
	}
	return msg
}

// isSlackNotFound reports whether an error from slack-go indicates a missing
// or inaccessible resource. Slack returns these as string error codes.
func isSlackNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "channel_not_found" ||
		msg == "user_not_found" ||
		msg == "thread_not_found" ||
		msg == "not_in_channel"
}
