package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// Handler answers a call or hears a notification from the other side. For a
// notification what it answers is dropped. An error it returns that is an
// *Error goes back as it is; any other, as a call refused, with its words.
type Handler func(ctx context.Context, c *Conn, method string, params json.RawMessage) (any, error)

// Conn is one side of a connection: it calls the other side, answers the
// other side's calls, and hears its notifications. It is safe for use by
// several goroutines.
type Conn struct {
	rw     io.ReadWriteCloser
	handle Handler
	ctx    context.Context
	cancel context.CancelFunc

	wmu sync.Mutex
	enc *json.Encoder

	next    atomic.Int64
	mu      sync.Mutex
	pending map[int64]chan Message
	err     error
}

// NewConn begins reading rw; h answers what the other side sends, and may be
// nil for a side that is only ever called back with notifications it does
// not need.
func NewConn(rw io.ReadWriteCloser, h Handler) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Conn{rw: rw, handle: h, ctx: ctx, cancel: cancel, enc: json.NewEncoder(rw), pending: map[int64]chan Message{}}
	go c.read()
	return c
}

// Done is closed when the connection ends.
func (c *Conn) Done() <-chan struct{} { return c.ctx.Done() }

// Err is why the connection ended, once it has.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close ends the connection.
func (c *Conn) Close() error {
	c.cancel()
	return c.rw.Close()
}

// Call calls method with params on the other side and decodes its answer
// into result, which may be nil.
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	id := c.next.Add(1)
	ch := make(chan Message, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return c.err
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.send(Message{JSONRPC: "2.0", ID: &id, Method: method, Params: raw}); err != nil {
		return err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return m.Error
		}
		if result == nil || len(m.Result) == 0 {
			return nil
		}
		return json.Unmarshal(m.Result, result)
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		if err := c.Err(); err != nil {
			return err
		}
		return io.EOF
	}
}

// Notify tells the other side something, asking no answer.
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(Message{JSONRPC: "2.0", Method: method, Params: raw})
}

func (c *Conn) send(m Message) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.enc.Encode(m)
}

// read takes every message the other side sends until the connection ends:
// answers go to who waits for them, calls and notifications to the handler.
func (c *Conn) read() {
	dec := json.NewDecoder(c.rw)
	for {
		var m Message
		err := dec.Decode(&m)
		if err != nil {
			c.mu.Lock()
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				c.err = io.EOF
			} else {
				c.err = fmt.Errorf("the connection could not be read: %w", err)
			}
			c.mu.Unlock()
			c.cancel()
			_ = c.rw.Close()
			return
		}
		switch {
		case m.Method == "" && m.ID != nil:
			c.mu.Lock()
			ch := c.pending[*m.ID]
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		case m.Method != "" && m.ID == nil:
			// Notifications are heard in the order they were sent: what a run
			// tells of itself is told in order.
			c.answer(m)
		case m.Method != "":
			go c.answer(m)
		}
	}
}

func (c *Conn) answer(m Message) {
	var result any
	var err error
	if c.handle == nil {
		err = &Error{Code: MethodNotFound, Message: m.Method + " is not answered here"}
	} else {
		result, err = c.handle(c.ctx, c, m.Method, m.Params)
	}
	if m.ID == nil {
		return // a notification is answered by nothing
	}
	reply := Message{JSONRPC: "2.0", ID: m.ID}
	var e *Error
	switch {
	case errors.As(err, &e):
		reply.Error = e
	case err != nil:
		reply.Error = &Error{Code: Refused, Message: err.Error()}
	default:
		raw, merr := json.Marshal(result)
		if merr != nil {
			reply.Error = &Error{Code: InternalError, Message: merr.Error()}
		} else {
			reply.Result = raw
		}
	}
	_ = c.send(reply)
}

// Decode reads params into v, or says the params are not what the method
// takes.
func Decode(params json.RawMessage, v any) error {
	if len(params) == 0 {
		params = []byte("null")
	}
	if err := json.Unmarshal(params, v); err != nil {
		return &Error{Code: InvalidParams, Message: err.Error()}
	}
	return nil
}

// Socket is where the service listens: IFLOW_SOCKET where it is set, else
// beside the person's iflow configuration.
func Socket() string {
	if s := os.Getenv("IFLOW_SOCKET"); s != "" {
		return s
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "iflow", "serve.sock")
}

// Dial connects to the service at socket, with h hearing what it sends.
func Dial(ctx context.Context, socket string, h Handler) (*Conn, error) {
	var d net.Dialer
	nc, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, err
	}
	return NewConn(nc, h), nil
}
