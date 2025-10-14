package mtglib

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"

	"github.com/dewadaru/mtg/v2/essentials"
)

const bufferSize = 32 * 1024 // 32KB, tune as needed

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, bufferSize)
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
		c.stream.Send(c.ctx, NewEventTraffic(c.streamID, uint(n), true))
	}

	return n, err //nolint: wrapcheck
}

func (c connTraffic) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)

	if n > 0 {
		c.stream.Send(c.ctx, NewEventTraffic(c.streamID, uint(n), false))
	}

	return n, err //nolint: wrapcheck
}

type connRewind struct {
	essentials.Conn

	active io.Reader
	buf    bytes.Buffer
	mutex  sync.RWMutex
}

func (c *connRewind) Read(p []byte) (int, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.active.Read(p) //nolint: wrapcheck
}

func (c *connRewind) Rewind() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.active = io.MultiReader(&c.buf, c.Conn)
}

func newConnRewind(conn essentials.Conn) *connRewind {
	rv := &connRewind{
		Conn: conn,
	}
	rv.active = io.TeeReader(conn, &rv.buf)

	return rv
}

// Copy copies data from src to dst using a pooled buffer.
func Copy(dst net.Conn, src net.Conn) (written int64, err error) {
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}

// Optionally, add a context-aware copy for cancellation support.
