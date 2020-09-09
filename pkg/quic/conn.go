// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package quic

import (
	"context"
	"crypto/tls"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"

	"storj.io/common/memory"
	"storj.io/common/rpc"
)

// Conn is a wrapper around a quic connection and fulfills net.Conn interface.
type Conn struct {
	once sync.Once
	// The Conn.stream varible should never be directly accessed.
	// Always use Conn.getStream() instead.
	stream quic.Stream

	acceptErr error
	session   quic.Session
}

// Read implements the Conn Read method.
func (c *Conn) Read(b []byte) (n int, err error) {
	stream, err := c.getStream()
	if err != nil {
		return 0, err
	}
	return stream.Read(b)
}

// Write implements the Conn Write method.
func (c *Conn) Write(b []byte) (int, error) {
	stream, err := c.getStream()
	if err != nil {
		return 0, err
	}
	return stream.Write(b)
}

func (c *Conn) getStream() (quic.Stream, error) {
	// Outgoing connections `stream` gets set when the Conn is initialized.
	// It's only with incoming connections that `stream == nil` and this
	// AcceptStream() code happens.
	if c.stream == nil {
		// When this function completes, it guarantees either c.acceptErr is not nil or c.stream is not nil
		c.once.Do(func() {
			stream, err := c.session.AcceptStream(context.Background())
			if err != nil {
				c.acceptErr = err
				return
			}

			c.stream = stream
		})
		if c.acceptErr != nil {
			return nil, c.acceptErr
		}
	}

	return c.stream, nil
}

// ConnectionState converts quic session state to tls connection state and returns tls state.
func (c *Conn) ConnectionState() tls.ConnectionState {
	return c.session.ConnectionState().ConnectionState
}

// Close closes the quic connection.
func (c *Conn) Close() error {
	return c.session.CloseWithError(quic.ErrorCode(0), "")
}

// LocalAddr returns the local address.
func (c *Conn) LocalAddr() net.Addr {
	return c.session.LocalAddr()
}

// RemoteAddr returns the address of the peer.
func (c *Conn) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
func (c *Conn) SetReadDeadline(t time.Time) error {
	stream, err := c.getStream()
	if err != nil {
		return err
	}
	return stream.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	stream, err := c.getStream()
	if err != nil {
		return err
	}
	return stream.SetWriteDeadline(t)
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
func (c *Conn) SetDeadline(t time.Time) error {
	stream, err := c.getStream()
	if err != nil {
		return err
	}

	return stream.SetDeadline(t)
}

//
// timed conns
//

// timedConn wraps a rpc.ConnectorConn so that all reads and writes get the specified timeout and
// return bytes no faster than the rate. If the timeout or rate are zero, they are
// ignored.
type timedConn struct {
	rpc.ConnectorConn
	rate memory.Size
}

// now returns time.Now if there's a nonzero rate.
func (t *timedConn) now() (now time.Time) {
	if t.rate > 0 {
		now = time.Now()
	}
	return now
}

// delay ensures that we sleep to keep the rate if it is nonzero. n is the number of
// bytes in the read or write operation we need to delay.
func (t *timedConn) delay(start time.Time, n int) {
	if t.rate > 0 {
		expected := time.Duration(n * int(time.Second) / t.rate.Int())
		if actual := time.Since(start); expected > actual {
			time.Sleep(expected - actual)
		}
	}
}

// Read wraps the connection read and adds sleeping to ensure the rate.
func (t *timedConn) Read(p []byte) (int, error) {
	start := t.now()
	n, err := t.ConnectorConn.Read(p)
	t.delay(start, n)
	return n, err
}

// Write wraps the connection write and adds sleeping to ensure the rate.
func (t *timedConn) Write(p []byte) (int, error) {
	start := t.now()
	n, err := t.ConnectorConn.Write(p)
	t.delay(start, n)
	return n, err
}

// closeTrackingConn wraps a rpc.ConnectorConn and keeps track of if it was closed
// or if it was leaked (and closes it if it was leaked).
type closeTrackingConn struct {
	rpc.ConnectorConn
}

// trackClose wraps the conn and sets a  finalizer on the returned value to
// close the conn and monitor that it was leaked.
func trackClose(conn rpc.ConnectorConn) rpc.ConnectorConn {
	tracked := &closeTrackingConn{ConnectorConn: conn}
	runtime.SetFinalizer(tracked, (*closeTrackingConn).finalize)
	return tracked
}

// Close clears the finalizer and closes the connection.
func (c *closeTrackingConn) Close() error {
	runtime.SetFinalizer(c, nil)
	mon.Event("quic_connection_closed")
	return c.ConnectorConn.Close()
}

// finalize monitors that a connection was leaked and closes the connection.
func (c *closeTrackingConn) finalize() {
	mon.Event("quic_connection_leaked")
	_ = c.ConnectorConn.Close()
}
