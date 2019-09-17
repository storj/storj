// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"net"
	"sync"
	"time"
)

type timeoutConn struct {
	conn    net.Conn
	timeout time.Duration

	mu         sync.Mutex
	progressed time.Time
}

func (tc *timeoutConn) Read(b []byte) (n int, err error) {
	return tc.withDeadline(func(deadline time.Time) (n int, err error) {
		err = tc.SetReadDeadline(deadline)
		if err != nil {
			return 0, err
		}

		return tc.conn.Read(b)
	})
}

func (tc *timeoutConn) Write(b []byte) (n int, err error) {
	return tc.withDeadline(func(deadline time.Time) (n int, err error) {
		err = tc.SetWriteDeadline(deadline)
		if err != nil {
			return 0, err
		}

		return tc.conn.Write(b)
	})
}

// withDeadline ensures that Read/Write only return with timeout when neither have made progress for tc.timeout.
func (tc *timeoutConn) withDeadline(op func(deadline time.Time) (n int, err error)) (n int, err error) {
	started := time.Now()
	deadline := started.Add(tc.timeout)

	for {
		n, err = op(deadline)
		finished := time.Now()

		tc.mu.Lock()
		// did we make progress?
		if n > 0 {
			// update progress time
			tc.progressed = finished
			tc.mu.Unlock()
			break
		}
		lastProgress := tc.progressed
		tc.mu.Unlock()

		// was it a timeout?
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			// check whether something made progress
			sinceProgress := finished.Sub(lastProgress)
			if sinceProgress < 0 {
				sinceProgress = 0
			}

			// since something made progress, setup a new deadline
			if sinceProgress < tc.timeout {
				deadline = finished.Add(sinceProgress)
				continue
			}
		}

		// some other error occurred
		break
	}
	return n, err
}

func (tc *timeoutConn) Close() error {
	return tc.conn.Close()
}

func (tc *timeoutConn) LocalAddr() net.Addr {
	return tc.conn.LocalAddr()
}

func (tc *timeoutConn) RemoteAddr() net.Addr {
	return tc.conn.RemoteAddr()
}

func (tc *timeoutConn) SetDeadline(t time.Time) error {
	return tc.conn.SetDeadline(t)
}

func (tc *timeoutConn) SetReadDeadline(t time.Time) error {
	return tc.conn.SetReadDeadline(t)
}

func (tc *timeoutConn) SetWriteDeadline(t time.Time) error {
	return tc.conn.SetWriteDeadline(t)
}
