package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipe creates a pair of connected in-memory pipes suitable for use as fake
// stdin/stdout in tests. Returns (writeToProcess, readFromProcess) and their
// counterparts for the "process" side.
func pipe(t *testing.T) (clientStdin io.WriteCloser, clientStdout io.Reader, processStdin io.Reader, processStdout io.WriteCloser) {
	t.Helper()
	pr1, pw1 := io.Pipe() // client writes → process reads
	pr2, pw2 := io.Pipe() // process writes → client reads
	return pw1, pr2, pr1, pw2
}

func TestClient_Call_RequestResponse(t *testing.T) {
	clientStdin, clientStdout, processStdin, processStdout := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	// Simulate the server reading a request and writing a response.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		n, _ := processStdin.Read(buf)
		var req jsonRPCMsg
		_ = json.Unmarshal(buf[:n], &req)

		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"answer":42}}`+"\n", *req.ID)
		_, _ = processStdout.Write([]byte(resp))
	}()

	var out struct{ Answer int }
	err := c.Call(context.Background(), "test.method", map[string]any{"q": 1}, &out)
	require.NoError(t, err)
	assert.Equal(t, 42, out.Answer)
	wg.Wait()
}

func TestClient_Call_ServerError(t *testing.T) {
	clientStdin, clientStdout, processStdin, processStdout := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	go func() {
		buf := make([]byte, 4096)
		n, _ := processStdin.Read(buf)
		var req jsonRPCMsg
		_ = json.Unmarshal(buf[:n], &req)
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"error":{"code":-32600,"message":"invalid request"}}`+"\n", *req.ID)
		_, _ = processStdout.Write([]byte(resp))
	}()

	err := c.Call(context.Background(), "bad.method", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestClient_Call_ContextCancelled(t *testing.T) {
	clientStdin, clientStdout, processStdin, _ := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	// Drain the request so the write doesn't block, but never respond so the
	// call stays pending until the context deadline fires.
	go io.Copy(io.Discard, processStdin)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.Call(ctx, "slow.method", nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestClient_Call_ConcurrentRequests(t *testing.T) {
	clientStdin, clientStdout, processStdin, processStdout := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	// Server echoes requests back, preserving IDs.
	go func() {
		dec := json.NewDecoder(processStdin)
		for {
			var req jsonRPCMsg
			if err := dec.Decode(&req); err != nil {
				return
			}
			resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"id":%d}}`+"\n", *req.ID, *req.ID)
			_, _ = processStdout.Write([]byte(resp))
		}
	}()

	const n = 10
	errs := make([]error, n)
	outs := make([]struct{ ID int64 }, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = c.Call(context.Background(), "echo", nil, &outs[i])
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
	}
}

func TestClient_Notifications(t *testing.T) {
	clientStdin, clientStdout, _, processStdout := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	// Send two notifications from the server side.
	go func() {
		_, _ = processStdout.Write([]byte(`{"jsonrpc":"2.0","method":"event/tick","params":{"n":1}}` + "\n"))
		_, _ = processStdout.Write([]byte(`{"jsonrpc":"2.0","method":"event/tick","params":{"n":2}}` + "\n"))
		processStdout.Close()
	}()

	var received []Notification
	for n := range c.Notifications() {
		received = append(received, n)
	}
	require.Len(t, received, 2)
	assert.Equal(t, "event/tick", received[0].Method)
	assert.Equal(t, "event/tick", received[1].Method)
}

func TestClient_ServerRequests(t *testing.T) {
	clientStdin, clientStdout, _, processStdout := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	go func() {
		_, _ = processStdout.Write([]byte(`{"jsonrpc":"2.0","id":7,"method":"item/commandExecution/requestApproval","params":{"turnId":"t1"}}` + "\n"))
		processStdout.Close()
	}()

	var received []ServerRequest
	for req := range c.Requests() {
		received = append(received, req)
	}
	require.Len(t, received, 1)
	assert.Equal(t, int64(7), received[0].ID)
	assert.Equal(t, "item/commandExecution/requestApproval", received[0].Method)
	assert.Contains(t, string(received[0].Params), `"turnId":"t1"`)
}

func TestClient_Notify(t *testing.T) {
	clientStdin, clientStdout, processStdin, _ := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	ch := make(chan jsonRPCMsg, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := processStdin.Read(buf)
		var msg jsonRPCMsg
		_ = json.Unmarshal(buf[:n], &msg)
		ch <- msg
	}()

	err := c.Notify(context.Background(), "notify/ping", map[string]any{"x": 1})
	require.NoError(t, err)

	select {
	case msg := <-ch:
		assert.Equal(t, "notify/ping", msg.Method)
		assert.Nil(t, msg.ID, "notification must have no id")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestClient_Reply(t *testing.T) {
	clientStdin, clientStdout, processStdin, _ := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	ch := make(chan jsonRPCMsg, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := processStdin.Read(buf)
		var msg jsonRPCMsg
		_ = json.Unmarshal(buf[:n], &msg)
		ch <- msg
	}()

	err := c.Reply(context.Background(), 9, map[string]any{"ok": true})
	require.NoError(t, err)

	select {
	case msg := <-ch:
		require.NotNil(t, msg.ID)
		assert.Equal(t, int64(9), *msg.ID)
		assert.Equal(t, "", msg.Method)
		assert.Contains(t, string(msg.Result), `"ok":true`)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reply")
	}
}

func TestClient_Close_UnblocksCall(t *testing.T) {
	clientStdin, clientStdout, processStdin, _ := pipe(t)
	c := NewClient(clientStdin, clientStdout)

	// Drain so the write succeeds; never respond so the call stays pending.
	go io.Copy(io.Discard, processStdin)

	done := make(chan error, 1)
	go func() {
		done <- c.Call(context.Background(), "hang", nil, nil)
	}()

	// Give the goroutine time to block on the select.
	time.Sleep(20 * time.Millisecond)
	c.Close()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock in-flight Call")
	}
}

func TestClient_SkipsUnparseableLines(t *testing.T) {
	clientStdin, clientStdout, _, processStdout := pipe(t)
	c := NewClient(clientStdin, clientStdout)
	defer c.Close()

	go func() {
		// Garbage line followed by a valid notification.
		_, _ = processStdout.Write([]byte("not valid json\n"))
		_, _ = processStdout.Write([]byte(`{"jsonrpc":"2.0","method":"ok","params":{}}` + "\n"))
		processStdout.Close()
	}()

	var got []Notification
	for n := range c.Notifications() {
		got = append(got, n)
	}
	require.Len(t, got, 1)
	assert.Equal(t, "ok", got[0].Method)
}

func TestClient_IdFieldRoundtrip(t *testing.T) {
	// Verify that id=0 is properly handled (pointer vs zero-value distinction).
	raw := `{"jsonrpc":"2.0","id":0,"result":{}}`
	var msg jsonRPCMsg
	require.NoError(t, json.Unmarshal([]byte(raw), &msg))
	require.NotNil(t, msg.ID)
	assert.Equal(t, int64(0), *msg.ID)
}

func TestNotification_Unmarshal(t *testing.T) {
	raw := `{"jsonrpc":"2.0","method":"progress","params":{"pct":50}}`
	var msg jsonRPCMsg
	require.NoError(t, json.Unmarshal([]byte(raw), &msg))
	assert.Equal(t, "progress", msg.Method)
	assert.Nil(t, msg.ID)
	assert.True(t, strings.Contains(string(msg.Params), "50"))
}
