package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Notification is a server-sent JSON-RPC 2.0 message that has no id field.
// Agent app-servers use notifications to push events (tool calls, text deltas,
// permission requests) to the client without waiting for a request.
type Notification struct {
	Method string
	Params json.RawMessage
}

// ServerRequest is a server-initiated JSON-RPC request that expects a client
// response. Codex App Server v2 uses this shape for approval callbacks.
type ServerRequest struct {
	ID     int64
	Method string
	Params json.RawMessage
}

// jsonRPCMsg is a raw wire-level JSON-RPC 2.0 message. It can represent a
// response (has id, no method) or a server notification (has method, no id).
type jsonRPCMsg struct {
	JSONRPC string `json:"jsonrpc"`
	// ID uses a pointer so we can distinguish "absent" (notification) from 0.
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonRPCError) Error() string { return e.Message }

// Client is a JSON-RPC 2.0 client over a long-lived stdio connection.
//
// It supports:
//   - Concurrent request/response matching: each Call gets a unique id and
//     blocks on a per-call channel until the response arrives.
//   - Server-notification fan-out: incoming messages without an id are
//     dispatched to the Notifications channel without blocking the reader loop.
//
// The internal reader goroutine is started by NewClient and runs until the
// remote end closes stdout or Close is called.
type Client struct {
	writer io.WriteCloser
	reader *bufio.Reader

	mu      sync.Mutex // protects nextID and pending map
	wmu     sync.Mutex // serialises concurrent writes to writer
	nextID  int64
	pending map[int64]chan jsonRPCMsg

	notifications chan Notification
	requests      chan ServerRequest
	closeOnce     sync.Once
	done          chan struct{} // closed by Close to unblock in-flight Calls
}

// NewClient creates a Client that writes JSON-RPC requests to stdin and reads
// responses/notifications from stdout. A background goroutine is started
// immediately to dispatch incoming messages.
func NewClient(stdin io.WriteCloser, stdout io.Reader) *Client {
	c := &Client{
		writer:        stdin,
		reader:        bufio.NewReader(stdout),
		pending:       make(map[int64]chan jsonRPCMsg),
		notifications: make(chan Notification, 64),
		requests:      make(chan ServerRequest, 32),
		done:          make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Call sends a JSON-RPC request and blocks until the corresponding response
// arrives, ctx is cancelled, or the client is closed. The result field of the
// response is JSON-unmarshalled into out (must be a pointer, or nil to discard).
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan jsonRPCMsg, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.writeMsg(id, method, params); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return fmt.Errorf("jsonrpc: client closed")
	case msg := <-ch:
		if msg.Error != nil {
			return msg.Error
		}
		if out == nil || len(msg.Result) == 0 {
			return nil
		}
		return json.Unmarshal(msg.Result, out)
	}
}

// Notify sends a JSON-RPC notification (no id, no response expected).
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return c.writeMsg(-1, method, params)
}

// Notifications returns the channel on which server-sent notifications are
// delivered. Callers should drain this channel continuously; the reader
// goroutine drops notifications rather than block when the channel is full.
func (c *Client) Notifications() <-chan Notification {
	return c.notifications
}

// Requests returns the channel on which server-initiated JSON-RPC requests are
// delivered. Callers should drain this continuously; requests are dropped
// rather than blocking the reader goroutine when the channel is full.
func (c *Client) Requests() <-chan ServerRequest {
	return c.requests
}

// Reply sends a successful response to a previously received server request.
func (c *Client) Reply(ctx context.Context, id int64, result any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return c.writeResponse(id, result, nil)
}

// ReplyError sends an error response to a previously received server request.
func (c *Client) ReplyError(ctx context.Context, id int64, code int, message string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return c.writeResponse(id, nil, &jsonRPCError{Code: code, Message: message})
}

// Close shuts down the client by closing the done channel (to unblock in-flight
// Calls) and then closing the underlying writer. Safe to call multiple times.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		err = c.writer.Close()
	})
	return err
}

// writeResponse serialises and sends a single JSON-RPC response message.
func (c *Client) writeResponse(id int64, result any, rpcErr *jsonRPCError) error {
	type response struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      int64         `json:"id"`
		Result  any           `json:"result,omitempty"`
		Error   *jsonRPCError `json:"error,omitempty"`
	}
	resp := response{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr}
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("jsonrpc: marshal response %d: %w", id, err)
	}
	data = append(data, '\n')

	c.wmu.Lock()
	_, err = c.writer.Write(data)
	c.wmu.Unlock()
	if err != nil {
		return fmt.Errorf("jsonrpc: write response %d: %w", id, err)
	}
	return nil
}

// writeMsg serialises and sends a single JSON-RPC 2.0 message.
// Pass id == -1 to omit the id field (notification).
func (c *Client) writeMsg(id int64, method string, params any) error {
	type request struct {
		JSONRPC string `json:"jsonrpc"`
		ID      *int64 `json:"id,omitempty"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}
	req := request{JSONRPC: "2.0", Method: method, Params: params}
	if id >= 0 {
		req.ID = &id
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("jsonrpc: marshal %s: %w", method, err)
	}
	data = append(data, '\n')

	c.wmu.Lock()
	_, err = c.writer.Write(data)
	c.wmu.Unlock()
	if err != nil {
		return fmt.Errorf("jsonrpc: write %s: %w", method, err)
	}
	return nil
}

// readLoop reads newline-delimited JSON messages from the remote stdout,
// dispatching responses to their pending Call channels and notifications to
// the buffered notifications channel. It exits on EOF or read error.
//
// On exit the done channel is closed (via Close) so in-flight Calls unblock,
// and the notifications channel is closed so consumers can detect shutdown.
func (c *Client) readLoop() {
	defer close(c.notifications)
	defer close(c.requests)
	defer c.Close() // unblock pending Calls and close writer on subprocess exit
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return
		}
		if len(line) == 0 {
			continue
		}

		var msg jsonRPCMsg
		if jsonErr := json.Unmarshal(line, &msg); jsonErr != nil {
			// Unparseable line (e.g. a startup banner) — skip.
			continue
		}

		if msg.Method != "" {
			if msg.ID != nil {
				req := ServerRequest{
					ID:     *msg.ID,
					Method: msg.Method,
					Params: msg.Params,
				}
				select {
				case c.requests <- req:
				default:
					// Channel full — drop rather than stall the reader goroutine.
				}
			} else {
				// Server-sent notification: fan out without blocking the reader.
				n := Notification{Method: msg.Method, Params: msg.Params}
				select {
				case c.notifications <- n:
				default:
					// Channel full — drop rather than stall the reader goroutine.
				}
			}
			continue
		}

		if msg.ID == nil {
			continue
		}

		// Response: find the matching pending Call and deliver the message.
		id := *msg.ID
		c.mu.Lock()
		ch, ok := c.pending[id]
		c.mu.Unlock()
		if ok {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}
