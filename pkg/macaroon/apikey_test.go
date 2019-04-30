// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSerializeParseRestrictAndCheck(t *testing.T) {
	secret, err := NewSecret()
	require.NoError(t, err)
	key, err := NewAPIKey(secret)
	require.NoError(t, err)

	serialized, err := key.Serialize()
	require.NoError(t, err)
	parsedKey, err := ParseAPIKey(serialized)
	require.NoError(t, err)
	require.True(t, bytes.Equal(key.Head(), parsedKey.Head()))
	require.True(t, bytes.Equal(key.Tail(), parsedKey.Tail()))

	restricted, err := key.Restrict(Caveat{
		EncryptedPathPrefixes: [][]byte{[]byte("a-test-path")},
	})
	require.NoError(t, err)

	serialized, err = restricted.Serialize()
	require.NoError(t, err)
	parsedKey, err = ParseAPIKey(serialized)
	require.NoError(t, err)
	require.True(t, bytes.Equal(key.Head(), parsedKey.Head()))
	require.False(t, bytes.Equal(key.Tail(), parsedKey.Tail()))

	now := time.Now()
	action1 := Action{
		Op:            Action_READ,
		Time:          &now,
		EncryptedPath: []byte("a-test-path"),
	}
	action2 := Action{
		Op:            Action_READ,
		Time:          &now,
		EncryptedPath: []byte("another-test-path"),
	}

	require.NoError(t, key.Check(secret, action1, nil))
	require.NoError(t, key.Check(secret, action2, nil))
	require.NoError(t, parsedKey.Check(secret, action1, nil))
	err = parsedKey.Check(secret, action2, nil)
	require.True(t, ErrUnauthorized.Has(err), err)
}

func TestRevocation(t *testing.T) {
	secret, err := NewSecret()
	require.NoError(t, err)
	key, err := NewAPIKey(secret)
	require.NoError(t, err)

	restricted, err := key.Restrict(Caveat{
		DisallowReads: true,
	})
	require.NoError(t, err)

	now := time.Now()
	action := Action{
		Op:   Action_WRITE,
		Time: &now,
	}

	require.NoError(t, key.Check(secret, action, nil))
	require.NoError(t, restricted.Check(secret, action, nil))

	require.True(t, ErrRevoked.Has(key.Check(secret, action, [][]byte{restricted.Head()})))
	require.True(t, ErrRevoked.Has(restricted.Check(secret, action, [][]byte{restricted.Head()})))

	require.NoError(t, key.Check(secret, action, [][]byte{restricted.Tail()}))
	require.True(t, ErrRevoked.Has(restricted.Check(secret, action, [][]byte{restricted.Tail()})))
}
