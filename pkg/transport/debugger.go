// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
)

var (
	mu    sync.Mutex
	alive []*debugConnection
)

func init() {
	go monitor()
}

func monitor() {
	file, err := ioutil.TempFile("", "dump.*.log")
	if err != nil {
		panic(err)
	}
	name := file.Name()
	file.Close()

	var result bytes.Buffer
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		result.Reset()

		mu.Lock()
		aliveCount := len(alive)
		for _, conn := range alive {
			fmt.Fprintf(&result, "\n%p %s R=%d W=%d\nSTACK:%s\n\n", conn, conn.id, atomic.LoadInt64(&conn.read), atomic.LoadInt64(&conn.write), conn.stack)
		}
		mu.Unlock()

		zap.S().Debugf("%s: alive connections: %d", name, aliveCount)
		ioutil.WriteFile(name, result.Bytes(), 0755)
	}
}

type diagnosticNode struct {
	id    storj.NodeID
	stack []byte
}

func (n *diagnosticNode) DialContext(ctx context.Context, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if conn == nil {
		return conn, err
	}

	dconn := &debugConnection{n.id, conn, 0, 0, n.stack}

	mu.Lock()
	alive = append(alive, dconn)
	mu.Unlock()

	return dconn, err
}

type debugConnection struct {
	id storj.NodeID
	net.Conn

	read  int64
	write int64

	stack []byte
}

func (conn *debugConnection) Read(b []byte) (n int, err error) {
	n, err = conn.Conn.Read(b)
	atomic.AddInt64(&conn.read, int64(n))
	return n, err
}

func (conn *debugConnection) Write(b []byte) (n int, err error) {
	n, err = conn.Conn.Write(b)
	atomic.AddInt64(&conn.write, int64(n))
	return n, err
}

func (conn *debugConnection) Close() error {
	mu.Lock()
	for i, dconn := range alive {
		if dconn == conn {
			alive = append(alive[:i], alive[i+1:]...)
			break
		}
	}
	mu.Unlock()
	return conn.Conn.Close()
}
