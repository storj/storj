// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/version"
)

func TestTryRunBinary(t *testing.T) {
	ctx := testcontext.New(t)
	log := zaptest.NewLogger(t)

	okprog := ctx.Compile("storj.io/storj/cmd/storagenode-updater/testdata/okprog")
	failprog := ctx.Compile("storj.io/storj/cmd/storagenode-updater/testdata/failprog")

	t.Run("ok", func(t *testing.T) {
		err := tryRunBinary(ctx, log, "ok", okprog)
		require.NoError(t, err)
	})
	t.Run("fail", func(t *testing.T) {
		err := tryRunBinary(ctx, log, "fail", failprog)
		require.Error(t, err)
		require.ErrorContains(t, err, "oh noes, this is broken")
		t.Log(err)
	})
}

func TestLastUpdateFailure(t *testing.T) {
	ctx := testcontext.New(t)
	log := zaptest.NewLogger(t)

	_, ok := loadLastUpdateFailure(ctx, log, ctx.Dir(), "example")
	require.False(t, ok)

	update := failedUpdate{
		Version: version.SemVer{
			Version: semver.Version{
				Major: 10,
				Minor: 11,
				Patch: 12,
				Pre:   []semver.PRVersion{{VersionStr: "pre"}},
				Build: []string{"linux"},
			},
		},
		Date:    time.Now(),
		Failure: "panic in executable",
	}

	saveLastUpdateFailure(ctx, log, ctx.Dir(), "example", update)

	loaded, ok := loadLastUpdateFailure(ctx, log, ctx.Dir(), "example")
	require.True(t, ok)

	// compare date separately due to monotonic time embedded in time.Time.
	require.True(t, update.Date.Equal(loaded.Date))
	loaded.Date = update.Date

	require.EqualValues(t, update, loaded)
}
