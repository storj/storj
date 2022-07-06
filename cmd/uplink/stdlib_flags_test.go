// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseHumanDate(t *testing.T) {
	t.Run("parse relative date", func(t *testing.T) {
		parsed, err := parseHumanDate("+24h")
		require.NoError(t, err)
		require.Less(t, parsed.Unix(), time.Now().Add(25*time.Hour).Unix())
		require.Greater(t, parsed.Unix(), time.Now().Add(23*time.Hour).Unix())
	})

	t.Run("parse absolute date", func(t *testing.T) {
		parsed, err := parseHumanDate("2030-02-03T12:13:14+01:00")
		require.NoError(t, err)
		require.Equal(t, "2030-02-03T12:13:14+01:00", parsed.Format(time.RFC3339))
	})

	t.Run("parse nonsense", func(t *testing.T) {
		parsed, err := parseHumanDate("999999")
		require.Equal(t, time.Time{}, parsed)
		require.Error(t, err)
	})
}
