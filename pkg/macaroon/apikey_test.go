// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
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
		Op:            ActionRead,
		Time:          now,
		EncryptedPath: []byte("a-test-path"),
	}
	action2 := Action{
		Op:            ActionRead,
		Time:          now,
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
		Op:   ActionWrite,
		Time: now,
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
		timestampToTest time.Time
		errClass        *errs.Class
	}{
		{key, time.Time{}, &Error},
		{notBeforeMinuteFromNow, time.Time{}, &Error},
		{notAfterMinuteAgo, time.Time{}, &Error},

		{key, now, nil},
		{notBeforeMinuteFromNow, now, &ErrUnauthorized},
		{notAfterMinuteAgo, now, &ErrUnauthorized},

		{key, twoMinutesAgo, nil},
		{notBeforeMinuteFromNow, twoMinutesAgo, &ErrUnauthorized},
		{notAfterMinuteAgo, twoMinutesAgo, nil},

		{key, twoMinutesFromNow, nil},
		{notBeforeMinuteFromNow, twoMinutesFromNow, nil},
		{notAfterMinuteAgo, twoMinutesFromNow, &ErrUnauthorized},
	} {
		err := test.keyToTest.Check(secret, Action{
			Op:   ActionRead,
			Time: test.timestampToTest,
		}, nil)
		if test.errClass == nil {
			require.NoError(t, err, fmt.Sprintf("test #%d", i+1))
		} else {
			require.False(t, !test.errClass.Has(err), fmt.Sprintf("test #%d", i+1))
		}
	}
}
