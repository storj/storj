// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"testing"
	"time"

	"github.com/zeebo/assert"

	"storj.io/common/testrand"
)

func TestRestoreTimeManager(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	rtm := NewRestoreTimeManager(dir)
	now := time.Unix(time.Now().Unix(), 0) // truncate to the second like the implementation

	sat := testrand.NodeID()

	// first call should implicitly set the time to now
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now).Equal(now))

	// it should stay that value until we set it t into the future
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now.Add(time.Second)).Equal(now))
	assert.NoError(t, rtm.SetRestoreTime(ctx, sat, now.Add(time.Second)))
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now.Add(time.Second)).Equal(now.Add(time.Second)))
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now.Add(2*time.Second)).Equal(now.Add(time.Second)))

	// we shouldn't be able to set it into the past
	assert.NoError(t, rtm.SetRestoreTime(ctx, sat, now))
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now).Equal(now.Add(time.Second)))
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now.Add(time.Second)).Equal(now.Add(time.Second)))
	assert.True(t, rtm.GetRestoreTime(ctx, sat, now.Add(2*time.Second)).Equal(now.Add(time.Second)))
}
