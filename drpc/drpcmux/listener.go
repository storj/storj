// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmux

import (
	"net"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc/drpcutil"
)

type listener struct {
	addr  net.Addr
	conns chan net.Conn
	sig   *drpcutil.Signal
}

func newListener(addr net.Addr) *listener {
	return &listener{
		addr:  addr,
		conns: make(chan net.Conn),
		sig:   drpcutil.NewSignal(),
	}
}

func (l *listener) Conns() chan net.Conn  { return l.conns }
func (l *listener) Sig() *drpcutil.Signal { return l.sig }

// Accept waits for and returns the next connection to the listener.
func (l *listener) Accept() (conn net.Conn, err error) {
	if err, ok := l.sig.Get(); ok {
		return nil, err
	}
	select {
	case <-l.sig.Signal():
		return nil, l.sig.Err()
	case conn = <-l.conns:
		return conn, nil
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *listener) Close() error {
	l.sig.Set(errs.New("listener closed"))
	return nil
}

// Addr returns the listener's network address.
func (l *listener) Addr() net.Addr {
	return l.addr
}
