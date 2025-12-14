package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC client for the BlazeLog server.
type Client struct {
	conn     *grpc.ClientConn
	client   blazelogv1.LogServiceClient
	stream   blazelogv1.LogService_StreamLogsClient
	agentID  string
	sequence uint64

	mu       sync.Mutex
	closed   bool
}

// NewClient creates a new gRPC client for the given server address.
// Uses insecure connection (mTLS will be added in Milestone 15).
func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to server: %w", err)
	}

	return &Client{
		conn:   conn,
		client: blazelogv1.NewLogServiceClient(conn),
	}, nil
}

// Register registers the agent with the server.
func (c *Client) Register(ctx context.Context, info *blazelogv1.AgentInfo) (*blazelogv1.RegisterResponse, error) {
	req := &blazelogv1.RegisterRequest{
		Agent: info,
	}

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("register agent: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("registration failed: %s", resp.ErrorMessage)
	}

	c.agentID = resp.AgentId
	return resp, nil
}

// StartStream starts the bidirectional log streaming.
func (c *Client) StartStream(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, err := c.client.StreamLogs(ctx)
	if err != nil {
		return fmt.Errorf("start stream: %w", err)
	}

	c.stream = stream
	return nil
}

// SendBatch sends a batch of log entries to the server.
func (c *Client) SendBatch(ctx context.Context, entries []*blazelogv1.LogEntry) error {
	c.mu.Lock()
	stream := c.stream
	c.mu.Unlock()

	if stream == nil {
		return fmt.Errorf("stream not started")
	}

	seq := atomic.AddUint64(&c.sequence, 1)
	batch := &blazelogv1.LogBatch{
		Entries: entries,
		AgentId: c.agentID,
		Sequence: seq,
	}

	if err := stream.Send(batch); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

// ReceiveResponses starts receiving responses from the server.
// Returns a channel for StreamResponse messages.
func (c *Client) ReceiveResponses(ctx context.Context) (<-chan *blazelogv1.StreamResponse, <-chan error) {
	responses := make(chan *blazelogv1.StreamResponse, 10)
	errs := make(chan error, 1)

	go func() {
		defer close(responses)
		defer close(errs)

		c.mu.Lock()
		stream := c.stream
		c.mu.Unlock()

		if stream == nil {
			errs <- fmt.Errorf("stream not started")
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := stream.Recv()
				if err != nil {
					errs <- err
					return
				}
				select {
				case responses <- resp:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return responses, errs
}

// Heartbeat sends a heartbeat to the server.
func (c *Client) Heartbeat(ctx context.Context, status *blazelogv1.AgentStatus) (*blazelogv1.HeartbeatResponse, error) {
	req := &blazelogv1.HeartbeatRequest{
		AgentId: c.agentID,
		Status:  status,
	}

	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: %w", err)
	}

	return resp, nil
}

// AgentID returns the server-assigned agent ID.
func (c *Client) AgentID() string {
	return c.agentID
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.stream != nil {
		if err := c.stream.CloseSend(); err != nil {
			// Log but don't fail
		}
	}

	return c.conn.Close()
}
