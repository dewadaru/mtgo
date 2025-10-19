package mtglib

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/dewadaru/mtg/v2/essentials"
)

// Pool for EventTraffic objects to reduce allocations
var eventTrafficPool = sync.Pool{
	New: func() any {
		return new(EventTraffic)
	},
}

type connTraffic struct {
	essentials.Conn

	streamID string
	stream   EventStream
	ctx      context.Context
}

func (c connTraffic) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)

	if n > 0 {
		evt := eventTrafficPool.Get().(*EventTraffic)
		*evt = NewEventTraffic(c.streamID, uint(n), true)
		// *evt = *NewEventTraffic(c.streamID, uint(n), true)
		c.stream.Send(c.ctx, evt)
		eventTrafficPool.Put(evt)
	}

	return n, err //nolint: wrapcheck
}

func (c connTraffic) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)

	if n > 0 {
		evt := eventTrafficPool.Get().(*EventTraffic)
		*evt = NewEventTraffic(c.streamID, uint(n), false)
		c.stream.Send(c.ctx, evt)
		eventTrafficPool.Put(evt)
	}

	return n, err //nolint: wrapcheck
}

// Pool for bytes.Buffer to reduce allocations in connRewind
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type connRewind struct {
	essentials.Conn

	active io.Reader
	buf    *bytes.Buffer
	mutex  sync.Mutex
}

func (c *connRewind) Read(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.active.Read(p) //nolint: wrapcheck
}

func (c *connRewind) Rewind() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.active = io.MultiReader(c.buf, c.Conn)
}

func newConnRewind(conn essentials.Conn) *connRewind {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	rv := &connRewind{
		Conn: conn,
		buf:  buf,
	}
	rv.active = io.TeeReader(conn, rv.buf)

	return rv
}

// Add a Close method to return buffer to pool
func (c *connRewind) Close() error {
	if c.buf != nil {
		bufferPool.Put(c.buf)
		c.buf = nil
	}
	return c.Conn.Close()
}
