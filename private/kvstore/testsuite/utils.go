// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/private/kvstore"
)

func newItem(key, value string, isPrefix bool) kvstore.Item {
	return kvstore.Item{
		Key:      kvstore.Key(key),
		Value:    kvstore.Value(value),
		IsPrefix: isPrefix,
	}
}

func cleanupItems(t testing.TB, ctx *testcontext.Context, store kvstore.Store, items kvstore.Items) {
	for _, item := range items {
		_ = store.Delete(ctx, item.Key)
	}
}
