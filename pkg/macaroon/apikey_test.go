// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"bytes"
	"fmt"
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
}

func TestExpiration(t *testing.T) {
	secret, err := NewSecret()
	require.NoError(t, err)
	key, err := NewAPIKey(secret)
	require.NoError(t, err)

	now := time.Now()
	minuteAgo := now.Add(-time.Minute)
	minuteFromNow := now.Add(time.Minute)
	twoMinutesAgo := now.Add(-2 * time.Minute)
	twoMinutesFromNow := now.Add(2 * time.Minute)

	notBeforeMinuteFromNow, err := key.Restrict(Caveat{
		NotBefore: &minuteFromNow,
	})
	require.NoError(t, err)
	notAfterMinuteAgo, err := key.Restrict(Caveat{
		NotAfter: &minuteAgo,
	})
	require.NoError(t, err)

	for i, test := range []struct {
		keyToTest       *APIKey
		timestampToTest *time.Time
		authorized      bool
	}{
		{key, nil, false},
		{notBeforeMinuteFromNow, nil, false},
		{notAfterMinuteAgo, nil, false},

		{key, &now, true},
		{notBeforeMinuteFromNow, &now, false},
		{notAfterMinuteAgo, &now, false},

		{key, &twoMinutesAgo, true},
		{notBeforeMinuteFromNow, &twoMinutesAgo, false},
		{notAfterMinuteAgo, &twoMinutesAgo, true},

		{key, &twoMinutesFromNow, true},
		{notBeforeMinuteFromNow, &twoMinutesFromNow, true},
		{notAfterMinuteAgo, &twoMinutesFromNow, false},
	} {
		err := test.keyToTest.Check(secret, Action{
			Op:   Action_READ,
			Time: test.timestampToTest,
		}, nil)
		if test.authorized {
			require.NoError(t, err, fmt.Sprintf("test #%d", i+1))
		} else {
			require.False(t, !ErrUnauthorized.Has(err), fmt.Sprintf("test #%d", i+1))
		}
	}
}
