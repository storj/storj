// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package listenmux

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListener(t *testing.T) {
	type addr struct{ net.Addr }
	type conn struct{ net.Conn }

	lis := newListener(addr{})

	{ // ensure the addr is the same we passed in
		require.Equal(t, lis.Addr(), addr{})
	}

	{ // ensure that we can accept a connection from the listener
		go func() { lis.Conns() <- conn{} }()
		c, err := lis.Accept()
		require.NoError(t, err)
		require.Equal(t, c, conn{})
	}

	{ // ensure that closing the listener is no problem
		require.NoError(t, lis.Close())
	}

	{ // ensure that accept after close returns the right error
		c, err := lis.Accept()
		require.Equal(t, err, Closed)
		require.Nil(t, c)
	}
}
