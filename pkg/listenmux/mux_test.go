// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package listenmux

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

type fakeListener []*prefixConn

func (fl *fakeListener) Close() error   { return nil }
func (fl *fakeListener) Addr() net.Addr { return nil }

func (fl *fakeListener) Accept() (c net.Conn, err error) {
	if len(*fl) == 0 {
		return nil, nil
	}
	c, *fl = (*fl)[0], (*fl)[1:]
	return c, nil
}

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

	lis := &fakeListener{
		newPrefixConn([]byte("prefix1data1"), nil),
		newPrefixConn([]byte("prefix2data2"), nil),
		newPrefixConn([]byte("prefix3data3"), nil),
	}

	mux := New(lis, len("prefixN"))

	var muxGroup errgroup.Group
	muxGroup.Go(func() error { return mux.Run(ctx) })

	var lisGroup errgroup.Group
	lisGroup.Go(expect(mux.Route("prefix1"), "data1"))
	lisGroup.Go(expect(mux.Route("prefix2"), "data2"))
	lisGroup.Go(expect(mux.Default(), "prefix3data3"))
	require.NoError(t, lisGroup.Wait())

	cancel()
	require.Equal(t, context.Canceled, muxGroup.Wait())
}
