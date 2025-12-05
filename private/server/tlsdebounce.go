// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"net"

	"github.com/spacemonkeygo/tlshowdy"
)

type tlsDebouncer struct {
	net.Listener
	debouncer func(addr net.Addr, message []byte) error
}

func tlsDebounce(l net.Listener, debouncer func(addr net.Addr, message []byte) error) net.Listener {
	return &tlsDebouncer{
		Listener:  l,
		debouncer: debouncer,
	}
}

type tlsDebouncedConn struct {
	net.Conn
	prefixed  net.Conn
	debouncer func(addr net.Addr, message []byte) error
}

func (l *tlsDebouncer) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	return &tlsDebouncedConn{
		Conn:      conn,
		debouncer: l.debouncer,
	}, err
}

func (l *tlsDebouncedConn) Read(p []byte) (n int, err error) {
	if l.debouncer != nil {
		rr := tlshowdy.NewRecordingReader(l.Conn)
		_, err := tlshowdy.Read(rr)
		if err != nil {
			return 0, err
		}
		err = l.debouncer(l.RemoteAddr(), rr.Received)
		if err != nil {
			return 0, err
		}
		l.debouncer = nil
		l.prefixed = tlshowdy.NewPrefixConn(rr.Received, l.Conn)
	}
	return l.prefixed.Read(p)
}
