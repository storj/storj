// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmux

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcutil"
)

type Mux struct {
	base      net.Listener
	prefixLen int
	addr      net.Addr
	def       *listener

	mu       sync.Mutex
	routes   map[string]*listener
	unrouted map[net.Conn]struct{}
	sig      *drpcutil.Signal
}

func New(base net.Listener, prefixLen int) *Mux {
	addr := base.Addr()
	return &Mux{
		base:      base,
		prefixLen: prefixLen,
		addr:      addr,
		def:       newListener(addr),
		routes:    make(map[string]*listener),
		unrouted:  make(map[net.Conn]struct{}),
		sig:       drpcutil.NewSignal(),
	}
}

//
// set up the routes
//

func (m *Mux) Sig() *drpcutil.Signal { return m.sig }

func (m *Mux) Default() net.Listener { return m.def }

func (m *Mux) Route(prefix string) net.Listener {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(prefix) != m.prefixLen {
		panic(fmt.Sprintf("invalid prefix: has %d but needs %d bytes", len(prefix), m.prefixLen))
	}

	lis, ok := m.routes[prefix]
	if !ok {
		lis = newListener(m.addr)
		m.routes[prefix] = lis
		go m.monitorListener(prefix, lis)
	}
	return lis
}

//
// run the muxer
//

func (m *Mux) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go m.monitorBase()
	go m.monitorSignal()
	go m.monitorContext(ctx)

	<-m.sig.Signal()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, lis := range m.routes {
		<-lis.Sig().Signal()
	}

	return m.sig.Err()
}

func (m *Mux) monitorSignal() {
	<-m.sig.Signal()
	// TODO(jeff): do we care about this error?
	_ = m.base.Close()
}

func (m *Mux) monitorContext(ctx context.Context) {
	<-ctx.Done()
	m.sig.Set(ctx.Err())
}

func (m *Mux) monitorListener(prefix string, lis *listener) {
	select {
	case <-m.sig.Signal():
		lis.Sig().Set(m.sig.Err())
	case <-lis.Sig().Signal():
	}
	m.mu.Lock()
	delete(m.routes, prefix)
	m.mu.Unlock()
}

func (m *Mux) monitorBase() {
	defer m.sig.Set(drpc.InternalError.New("mux exited with no signal"))

	for {
		conn, err := m.base.Accept()
		switch {
		case err != nil:
			// TODO(jeff): temporary errors?
			m.sig.Set(err)
			return
		case conn == nil:
			<-m.sig.Signal()
			return
		}

		// TODO(jeff): a limit on the number of outstanding unrouted connections?

		m.mu.Lock()
		m.unrouted[conn] = struct{}{}
		m.mu.Unlock()

		go m.routeConn(conn)
	}
}

func (m *Mux) routeConn(conn net.Conn) {
	defer func() {
		m.mu.Lock()
		delete(m.unrouted, conn)
		m.mu.Unlock()
	}()

	buf := make([]byte, m.prefixLen)
	if _, err := io.ReadFull(conn, buf); err != nil {
		// TODO(jeff): how to handle this error?
		return
	}

	m.mu.Lock()
	lis, ok := m.routes[string(buf)]
	if !ok {
		lis = m.def
		conn = newPrefixConn(buf, conn)
	}
	m.mu.Unlock()

	// TODO(jeff): a timeout for the listener to get to the conn?

	select {
	case <-lis.Sig().Signal():
		// TODO(jeff): better way to signal to the caller the listener is closed?
		_ = conn.Close()
	case lis.Conns() <- conn:
	}
}
