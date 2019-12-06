// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package listenmux

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"
)

func TestMux(t *testing.T) {
	expect := func(lis net.Listener, data string) func() error {
		return func() error {
			conn, err := lis.Accept()
			if err != nil {
				return err
			}

			buf := make([]byte, len(data))
			_, err = io.ReadFull(conn, buf)
			if err != nil {
				return err
			}

			require.Equal(t, data, string(buf))
			return nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis := newFakeListener(
		newPrefixConn([]byte("prefix1data1"), nil),
		newPrefixConn([]byte("prefix2data2"), nil),
		newPrefixConn([]byte("prefix3data3"), nil),
	)

	mux := New(lis, len("prefixN"))

	var lisGroup errgroup.Group
	lisGroup.Go(expect(mux.Route("prefix1"), "data1"))
	lisGroup.Go(expect(mux.Route("prefix2"), "data2"))
	lisGroup.Go(expect(mux.Default(), "prefix3data3"))

	var muxGroup errgroup.Group
	muxGroup.Go(func() error { return mux.Run(ctx) })

	require.NoError(t, lisGroup.Wait())

	cancel()
	require.Equal(t, nil, muxGroup.Wait())
}

func TestMuxAcceptError(t *testing.T) {
	err := errs.New("problem")
	mux := New(newErrorListener(err), 0)
	require.Equal(t, mux.Run(context.Background()), err)
}

//
// fake listener
//

type fakeListener struct {
	done  chan struct{}
	err   error
	conns []net.Conn
}

func (fl *fakeListener) Addr() net.Addr { return nil }

func (fl *fakeListener) Close() error {
	close(fl.done)
	return nil
}

func (fl *fakeListener) Accept() (c net.Conn, err error) {
	if fl.err != nil {
		return nil, fl.err
	}
	if len(fl.conns) == 0 {
		<-fl.done
		return nil, Closed
	}
	c, fl.conns = fl.conns[0], fl.conns[1:]
	return c, nil
}

func newFakeListener(conns ...net.Conn) *fakeListener {
	return &fakeListener{
		done:  make(chan struct{}),
		conns: conns,
	}
}

func newErrorListener(err error) *fakeListener {
	return &fakeListener{err: err}
}
