// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package debounce

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDebounceCount(t *testing.T) {
	d := NewDebouncer(time.Minute, 3)

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC)
	}
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}))

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 30, 0, 1, time.UTC)
	}
	require.ErrorIs(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}), ErrDuplicateMessage)
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{1}))

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 30, 0, 2, time.UTC)
	}
	require.ErrorIs(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}), ErrDuplicateMessage)

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 30, 0, 3, time.UTC)
	}
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}))
}

func TestDebounceAge(t *testing.T) {
	d := NewDebouncer(time.Minute, 3)

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC)
	}
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}))

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 31, 0, 0, time.UTC)
	}
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{1}))

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 31, 0, 1, time.UTC)
	}
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}))

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 32, 0, 1, time.UTC)
	}
	require.ErrorIs(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}), ErrDuplicateMessage)

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 32, 0, 2, time.UTC)
	}
	require.NoError(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}))

	timeNow = func() time.Time {
		return time.Date(2000, 2, 1, 12, 32, 0, 3, time.UTC)
	}
	require.ErrorIs(t, d.ResponderFirstMessageValidator(&net.TCPAddr{}, []byte{0}), ErrDuplicateMessage)
}
