// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest_test

import (
	"testing"

	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDatabase(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
	})
}
