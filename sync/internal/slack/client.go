// Package slack provides Slack pulling logic for the sync pipeline.
//
// This is similar to slack-subgraph/internal/slack but with different concerns:
// the subgraph plugin serves individual requests on demand; the sync pipeline
// pulls bulk data on a schedule. We don't share code because the two use cases
// have different error handling, pagination, and rate-limit strategies.
package slack

import (
	"context"
	"fmt"
	"time"

	slackgo "github.com/slack-go/slack"
)

// Client wraps slack-go for the sync pipeline's bulk pulling.
type Client struct {
	api *slackgo.Client
}

// NewClient returns a Slack client backed by the Web API.
// The token must be a bot user OAuth token (xoxb-...).
func NewClient(token string) *Client {
	return &Client{
		api: slackgo.New(token),
	}
}

// Channel is what the sync pipeline needs to know about a Slack channel.
type Channel struct {
	ID      string
	Name    string
	Purpose string
}

// Thread is a parent message plus its replies.
type Thread struct {
	ChannelID string
	TS        string
	Permalink string
	Messages  []Message
}

// Message captures everything the upsert needs.
type Message struct {
	TS        string
	UserID    string
	Text      string
	ThreadTS  string
	Permalink string

	// Resolved on the fly during pulling — the sync pipeline batches user
	// lookups so each message arrives with author details already populated.
	AuthorEmail   string
	AuthorName    string
	AuthorSlackID string
}

// GetChannel fetches metadata for a channel.
func (c *Client) GetChannel(ctx context.Context, id string) (*Channel, error) {
	info, err := c.api.GetConversationInfoContext(ctx, &slackgo.GetConversationInfoInput{
		ChannelID:         id,
		IncludeNumMembers: false,
	})
	if err != nil {
		return nil, fmt.Errorf("get channel %s: %w", id, err)
	}

	return &Channel{
		ID:      info.ID,
		Name:    info.Name,
		Purpose: info.Purpose.Value,
	}, nil
}

// ThreadsInChannel pulls every thread in a channel, including non-threaded
// top-level messages (which are returned as single-message threads).
//
// `since` constrains how far back to pull. Pass a zero time to pull
// everything the bot can see.
func (c *Client) ThreadsInChannel(ctx context.Context, channelID string, since time.Time) ([]Thread, error) {
	parents, err := c.pullParentMessages(ctx, channelID, since)
	if err != nil {
		return nil, fmt.Errorf("pull parents: %w", err)
	}

	// Build a unique set of all user IDs we encounter so we can resolve
	// them in one pass after we've collected every message.
	userIDs := make(map[string]struct{})

	threads := make([]Thread, 0, len(parents))
	for _, parent := range parents {
		thread, err := c.buildThread(ctx, channelID, parent, userIDs)
		if err != nil {
			return nil, fmt.Errorf("build thread %s: %w", parent.Timestamp, err)
		}
		threads = append(threads, thread)
	}

	// Resolve all the user IDs we collected to (email, name) pairs.
	users, err := c.resolveUsers(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve users: %w", err)
	}

	// Walk threads/messages and populate author details from the user map.
	for ti := range threads {
		for mi := range threads[ti].Messages {
			user, ok := users[threads[ti].Messages[mi].UserID]
			if !ok {
				continue
			}
			threads[ti].Messages[mi].AuthorEmail = user.Email
			threads[ti].Messages[mi].AuthorName = user.Name
			threads[ti].Messages[mi].AuthorSlackID = user.ID
		}
	}

	return threads, nil
}

// pullParentMessages walks channel history and returns every top-level
// message. Replies (messages with thread_ts != ts) are filtered out — they
// come back when we expand each thread.
func (c *Client) pullParentMessages(ctx context.Context, channelID string, since time.Time) ([]slackgo.Message, error) {
	var all []slackgo.Message
	cursor := ""

	for {
		params := &slackgo.GetConversationHistoryParameters{
			ChannelID: channelID,
			Cursor:    cursor,
			Limit:     200,
			// Without this Slack omits reply_count from every message and
			// threaded messages are ingested as single-message threads.
			IncludeAllMetadata: true,
		}
		if !since.IsZero() {
			params.Oldest = fmt.Sprintf("%d.000000", since.Unix())
		}

		history, err := c.api.GetConversationHistoryContext(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("conversations.history: %w", err)
		}

		for _, m := range history.Messages {
			// Skip system events (channel_join, channel_rename, …) —
			// they are not discussion content.
			if m.SubType != "" {
				continue
			}
			// Skip replies — they belong to a thread, not the top level
			if m.ThreadTimestamp != "" && m.ThreadTimestamp != m.Timestamp {
				continue
			}
			all = append(all, m)
		}

		if !history.HasMore {
			break
		}
		cursor = history.ResponseMetaData.NextCursor
	}

	return all, nil
}

// buildThread fetches the full reply chain for a parent message and returns
// a Thread containing the parent plus every reply. Records every author's
// user ID in seenUsers for later resolution.
func (c *Client) buildThread(ctx context.Context, channelID string, parent slackgo.Message, seenUsers map[string]struct{}) (Thread, error) {
	// Single-message thread: no replies, just the parent itself
	if parent.ReplyCount == 0 {
		permalink := c.permalink(ctx, channelID, parent.Timestamp)
		seenUsers[parent.User] = struct{}{}

		return Thread{
			ChannelID: channelID,
			TS:        parent.Timestamp,
			Permalink: permalink,
			Messages: []Message{
				{
					TS:        parent.Timestamp,
					UserID:    parent.User,
					Text:      parent.Text,
					ThreadTS:  parent.Timestamp,
					Permalink: permalink,
				},
			},
		}, nil
	}

	// Multi-message thread: pull all replies, including the parent
	replies, _, _, err := c.api.GetConversationRepliesContext(ctx, &slackgo.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: parent.Timestamp,
	})
	if err != nil {
		return Thread{}, fmt.Errorf("conversations.replies: %w", err)
	}

	parentPermalink := c.permalink(ctx, channelID, parent.Timestamp)

	messages := make([]Message, 0, len(replies))
	for _, r := range replies {
		seenUsers[r.User] = struct{}{}
		messages = append(messages, Message{
			TS:        r.Timestamp,
			UserID:    r.User,
			Text:      r.Text,
			ThreadTS:  r.ThreadTimestamp,
			Permalink: c.permalink(ctx, channelID, r.Timestamp),
		})
	}

	return Thread{
		ChannelID: channelID,
		TS:        parent.Timestamp,
		Permalink: parentPermalink,
		Messages:  messages,
	}, nil
}

// permalink fetches the permalink for a single message. Errors are
// non-fatal — an empty permalink is acceptable.
func (c *Client) permalink(ctx context.Context, channelID, ts string) string {
	link, err := c.api.GetPermalinkContext(ctx, &slackgo.PermalinkParameters{
		Channel: channelID,
		Ts:      ts,
	})
	if err != nil {
		return ""
	}
	return link
}

// userInfo holds the resolved details for a Slack user.
type userInfo struct {
	ID    string
	Email string
	Name  string
}

// resolveUsers looks up every user in the supplied set and returns a map
// from user ID to userInfo.
func (c *Client) resolveUsers(ctx context.Context, userIDs map[string]struct{}) (map[string]userInfo, error) {
	out := make(map[string]userInfo, len(userIDs))

	for userID := range userIDs {
		if userID == "" {
			continue
		}

		u, err := c.api.GetUserInfoContext(ctx, userID)
		if err != nil {
			// Soft-fail: log and continue. A missing user means the
			// message will be ingested without author resolution.
			continue
		}

		name := u.RealName
		if name == "" {
			name = u.Name
		}

		out[userID] = userInfo{
			ID:    u.ID,
			Email: u.Profile.Email,
			Name:  name,
		}
	}

	return out, nil
}
