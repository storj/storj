// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseHumanDate(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tbilisi")
	require.NoError(t, err)

	t.Run("parse relative date", func(t *testing.T) {
		parsed, err := parseHumanDateNotBefore("+24h")
		require.NoError(t, err)
		require.Less(t, parsed.Unix(), time.Now().Add(25*time.Hour).Unix())
		require.Greater(t, parsed.Unix(), time.Now().Add(23*time.Hour).Unix())
	})

	t.Run("parse relative date with day", func(t *testing.T) {
		parsed, err := parseHumanDateNotBefore("+13d")
		require.NoError(t, err)
		require.Less(t, parsed.Unix(), time.Now().Add((13*24+1)*time.Hour).Unix())
		require.Greater(t, parsed.Unix(), time.Now().Add((13*24-1)*time.Hour).Unix())
	})

	t.Run("parse absolute full date", func(t *testing.T) {
		parsed, err := parseHumanDateNotBefore("2030-02-03T12:13:14+01:00")
		require.NoError(t, err)
		require.Equal(t, "2030-02-03T12:13:14+01:00", parsed.Format(time.RFC3339))
	})

	t.Run("parse absolute date without TZ", func(t *testing.T) {
		parsed, err := parseHumanDateInLocation("2030-02-03T12:13:14", loc, false)
		require.NoError(t, err)
		require.Equal(t, "2030-02-03T12:13:14+04:00", parsed.Format(time.RFC3339))

		parsed, err = parseHumanDateInLocation("2030-02-03T12:13:14", loc, true)
		require.NoError(t, err)
		require.Equal(t, "2030-02-03T12:13:14.999999999+04:00", parsed.Format(time.RFC3339Nano))
	})

	t.Run("parse absolute date without sec", func(t *testing.T) {
		parsed, err := parseHumanDateInLocation("2030-02-03T12:13", loc, false)
		require.NoError(t, err)
		require.Equal(t, "2030-02-03T12:13:00+04:00", parsed.Format(time.RFC3339))

		parsed, err = parseHumanDateInLocation("2030-02-03T12:13", loc, true)
		require.NoError(t, err)
		require.Equal(t, "2030-02-03T12:13:59.999999999+04:00", parsed.Format(time.RFC3339Nano))
	})

	t.Run("parse absolute date without hour", func(t *testing.T) {
		parsed, err := parseHumanDateInLocation("2030-03-31", loc, false)
		require.NoError(t, err)
		require.Equal(t, "2030-03-31T00:00:00+04:00", parsed.Format(time.RFC3339))

		parsed, err = parseHumanDateInLocation("2030-03-31", loc, true)
		require.NoError(t, err)
		require.Equal(t, "2030-03-31T23:59:59.999999999+04:00", parsed.Format(time.RFC3339Nano))
	})

	t.Run("parse nonsense", func(t *testing.T) {
		parsed, err := parseHumanDateNotBefore("999999")
		require.Equal(t, time.Time{}, parsed)
		require.Error(t, err)
	})
}
