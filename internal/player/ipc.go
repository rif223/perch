package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Microsoft/go-winio"
)

var errIPCClosed = errors.New("IPC connection to mpv is closed")

// ipcMessage covers both command replies and asynchronous events of the
// mpv JSON IPC protocol (https://mpv.io/manual/stable/#json-ipc).
type ipcMessage struct {
	Event     string          `json:"event"`
	RequestID int64           `json:"request_id"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	Name      string          `json:"name"`
	Reason    string          `json:"reason"`
	FileError string          `json:"file_error"`
	Level     string          `json:"level"`
	Prefix    string          `json:"prefix"`
	Text      string          `json:"text"`
}

type ipcReply struct {
	err  string
	data json.RawMessage
}

// ipcClient is a single persistent connection to mpv's named pipe. Replies
// are matched to requests by request_id, events are handed to onEvent on the
// reader goroutine. onEvent must not call back into the client synchronously.
type ipcClient struct {
	conn    net.Conn
	onEvent func(ipcMessage)

	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[int64]chan ipcReply
	nextID  atomic.Int64

	closed    chan struct{}
	closeOnce sync.Once
}

// dialIPC retries until mpv has created the pipe or ctx expires.
func dialIPC(ctx context.Context, path string, onEvent func(ipcMessage)) (*ipcClient, error) {
	for {
		attempt, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		conn, err := winio.DialPipeContext(attempt, path)
		cancel()
		if err == nil {
			c := &ipcClient{
				conn:    conn,
				onEvent: onEvent,
				pending: make(map[int64]chan ipcReply),
				closed:  make(chan struct{}),
			}
			go c.readLoop()
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("Could not connect to mpv: %w", err)
		case <-time.After(75 * time.Millisecond):
		}
	}
}

func (c *ipcClient) readLoop() {
	defer c.Close()
	r := bufio.NewReaderSize(c.conn, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			var msg ipcMessage
			if json.Unmarshal(line, &msg) == nil {
				c.dispatch(msg)
			}
		}
		if err != nil {
			return
		}
	}
}

func (c *ipcClient) dispatch(msg ipcMessage) {
	if msg.Event != "" {
		if c.onEvent != nil {
			c.onEvent(msg)
		}
		return
	}
	c.mu.Lock()
	ch, ok := c.pending[msg.RequestID]
	delete(c.pending, msg.RequestID)
	c.mu.Unlock()
	if ok {
		ch <- ipcReply{err: msg.Error, data: msg.Data}
	}
}

// command sends one mpv command and waits for its reply.
func (c *ipcClient) command(ctx context.Context, args ...any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	payload, err := json.Marshal(map[string]any{"command": args, "request_id": id})
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')

	ch := make(chan ipcReply, 1)
	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return nil, errIPCClosed
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.pending != nil {
			delete(c.pending, id)
		}
		c.mu.Unlock()
	}()

	c.writeMu.Lock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = c.conn.Write(payload)
	c.writeMu.Unlock()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("Could not send command to mpv: %w", err)
	}

	select {
	case reply := <-ch:
		if reply.err != "success" {
			return nil, fmt.Errorf("mpv error: %s", reply.err)
		}
		return reply.data, nil
	case <-c.closed:
		return nil, errIPCClosed
	case <-ctx.Done():
		return nil, errors.New("mpv did not respond in time")
	}
}

func (c *ipcClient) Close() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
		c.mu.Lock()
		c.pending = nil
		c.mu.Unlock()
		close(c.closed)
	})
}
