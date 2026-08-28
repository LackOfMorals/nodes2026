package main

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	service "github.com/wundergraph/cosmo/plugin/generated"
	slackclient "github.com/wundergraph/cosmo/plugin/src/slackclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const bufSize = 1024 * 1024

// mockClient is an in-memory slackclient.Client. Each configured error takes
// precedence over the configured data; the captured *got* fields record the
// arguments the service layer passed through, so default-limit and
// nil-wrapper behaviour can be asserted without a network.
type mockClient struct {
	channelErr  error
	messagesErr error
	threadErr   error
	userErr     error

	channel  *slackclient.Channel
	messages []slackclient.Message
	thread   []slackclient.Message
	user     *slackclient.User

	gotMessageLimit int
}

var _ slackclient.Client = (*mockClient)(nil)

func (m *mockClient) GetChannel(ctx context.Context, id string) (*slackclient.Channel, error) {
	if m.channelErr != nil {
		return nil, m.channelErr
	}
	return m.channel, nil
}

func (m *mockClient) GetMessages(ctx context.Context, channelID string, limit int) ([]slackclient.Message, error) {
	m.gotMessageLimit = limit
	if m.messagesErr != nil {
		return nil, m.messagesErr
	}
	return m.messages, nil
}

func (m *mockClient) GetThread(ctx context.Context, channelID, threadTS string) ([]slackclient.Message, error) {
	if m.threadErr != nil {
		return nil, m.threadErr
	}
	return m.thread, nil
}

func (m *mockClient) GetUser(ctx context.Context, id string) (*slackclient.User, error) {
	if m.userErr != nil {
		return nil, m.userErr
	}
	return m.user, nil
}

// setupTestService starts a bufconn gRPC server with the SlackService backed
// by the given mock and returns a client connected to it.
func setupTestService(t *testing.T, mock *mockClient) (service.SlackServiceClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	service.RegisterSlackServiceServer(grpcServer, NewSlackService(mock))

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			// Server stopped during cleanup — not a test failure.
			return
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := service.NewSlackServiceClient(conn)
	cleanup := func() {
		require.NoError(t, conn.Close())
		grpcServer.Stop()
	}
	return client, cleanup
}

func TestQuerySlackChannelFound(t *testing.T) {
	mock := &mockClient{channel: &slackclient.Channel{
		ID: "C0BSX7Q9M0E", Name: "nodes-demo-eng",
		Topic: "demo topic", Purpose: "demo purpose", MemberCount: 6, IsPrivate: false,
	}}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackChannel(context.Background(),
		&service.QuerySlackChannelRequest{Id: "C0BSX7Q9M0E"})
	require.NoError(t, err)

	ch := resp.GetSlackChannel()
	require.NotNil(t, ch)
	assert.Equal(t, "C0BSX7Q9M0E", ch.GetId())
	assert.Equal(t, "nodes-demo-eng", ch.GetName())
	assert.Equal(t, "demo topic", ch.GetTopic().GetValue())
	assert.Equal(t, "demo purpose", ch.GetPurpose().GetValue())
	assert.Equal(t, int32(6), ch.GetMemberCount().GetValue())
	assert.False(t, ch.GetIsPrivate())
}

func TestQuerySlackChannelNotFoundReturnsNil(t *testing.T) {
	mock := &mockClient{channelErr: slackclient.ErrNotFound}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackChannel(context.Background(),
		&service.QuerySlackChannelRequest{Id: "C999"})
	require.NoError(t, err)
	assert.Nil(t, resp.GetSlackChannel())
}

func TestQuerySlackChannelErrorPropagates(t *testing.T) {
	mock := &mockClient{channelErr: errors.New("slack_api_error: ratelimited")}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	_, err := client.QuerySlackChannel(context.Background(),
		&service.QuerySlackChannelRequest{Id: "C999"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ratelimited")
}

func TestQuerySlackMessagesDefaultLimitAndMapping(t *testing.T) {
	mock := &mockClient{messages: []slackclient.Message{
		{ChannelID: "C1", TS: "1787845145.093149", UserID: "U1", Text: "hello", ThreadTS: "1787845100.000000", Permalink: "https://x.slack.com/1"},
		{ChannelID: "C1", TS: "1787845146.000001", UserID: "U1", Text: "top-level (no thread, no permalink on fetch failure)"},
	}}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	// No limit in the request — schema default 50 must reach the client.
	resp, err := client.QuerySlackMessages(context.Background(),
		&service.QuerySlackMessagesRequest{ChannelId: "C1"})
	require.NoError(t, err)
	assert.Equal(t, 50, mock.gotMessageLimit)

	msgs := resp.GetSlackMessages()
	require.Len(t, msgs, 2)
	assert.Equal(t, "1787845145.093149", msgs[0].GetTs())
	assert.Equal(t, "1787845100.000000", msgs[0].GetThreadTs().GetValue())
	assert.Equal(t, "https://x.slack.com/1", msgs[0].GetPermalink().GetValue())
	// Empty domain values must map to nil wrappers (null in GraphQL).
	assert.Nil(t, msgs[1].GetThreadTs())
	assert.Nil(t, msgs[1].GetPermalink())
}

func TestQuerySlackMessagesExplicitLimit(t *testing.T) {
	mock := &mockClient{messages: nil}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	_, err := client.QuerySlackMessages(context.Background(),
		&service.QuerySlackMessagesRequest{ChannelId: "C1", Limit: wrapperspb.Int32(5)})
	require.NoError(t, err)
	assert.Equal(t, 5, mock.gotMessageLimit)
}

func TestQuerySlackMessagesNotFoundReturnsEmpty(t *testing.T) {
	mock := &mockClient{messagesErr: slackclient.ErrNotFound}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackMessages(context.Background(),
		&service.QuerySlackMessagesRequest{ChannelId: "C999"})
	require.NoError(t, err)
	assert.Empty(t, resp.GetSlackMessages())
}

func TestQuerySlackThread(t *testing.T) {
	mock := &mockClient{thread: []slackclient.Message{
		{ChannelID: "C1", TS: "t0", UserID: "U1", Text: "parent"},
		{ChannelID: "C1", TS: "t1", UserID: "U2", Text: "reply", ThreadTS: "t0"},
	}}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackThread(context.Background(),
		&service.QuerySlackThreadRequest{ChannelId: "C1", ThreadTs: "t0"})
	require.NoError(t, err)
	msgs := resp.GetSlackThread()
	require.Len(t, msgs, 2)
	assert.Equal(t, "t0", msgs[0].GetTs())
	assert.Equal(t, "t0", msgs[1].GetThreadTs().GetValue())
}

func TestQuerySlackThreadNotFoundReturnsEmpty(t *testing.T) {
	mock := &mockClient{threadErr: slackclient.ErrNotFound}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackThread(context.Background(),
		&service.QuerySlackThreadRequest{ChannelId: "C1", ThreadTs: "nope"})
	require.NoError(t, err)
	assert.Empty(t, resp.GetSlackThread())
}

func TestQuerySlackUserFound(t *testing.T) {
	mock := &mockClient{user: &slackclient.User{
		ID: "U0BSZ59TY66", Name: "jg", RealName: "John Giffard",
		Email: "j.giffard@icloud.com", IsBot: false,
	}}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackUser(context.Background(),
		&service.QuerySlackUserRequest{Id: "U0BSZ59TY66"})
	require.NoError(t, err)

	u := resp.GetSlackUser()
	require.NotNil(t, u)
	assert.Equal(t, "U0BSZ59TY66", u.GetId())
	assert.Equal(t, "jg", u.GetName())
	assert.Equal(t, "John Giffard", u.GetRealName().GetValue())
	assert.Equal(t, "j.giffard@icloud.com", u.GetEmail().GetValue())
	assert.False(t, u.GetIsBot())
}

func TestQuerySlackUserNotFoundReturnsNil(t *testing.T) {
	mock := &mockClient{userErr: slackclient.ErrNotFound}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	resp, err := client.QuerySlackUser(context.Background(),
		&service.QuerySlackUserRequest{Id: "U999"})
	require.NoError(t, err)
	assert.Nil(t, resp.GetSlackUser())
}

func TestQuerySlackUserErrorPropagates(t *testing.T) {
	mock := &mockClient{userErr: errors.New("slack_api_error: invalid_auth")}
	client, cleanup := setupTestService(t, mock)
	defer cleanup()

	_, err := client.QuerySlackUser(context.Background(),
		&service.QuerySlackUserRequest{Id: "U999"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_auth")
}
