// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func newItem(key, value string, isPrefix bool) storage.ListItem {
	return storage.ListItem{
		Key:      storage.Key(key),
		Value:    storage.Value(value),
		IsPrefix: isPrefix,
	}
}

func cleanupItems(t testing.TB, ctx *testcontext.Context, store storage.KeyValueStore, items storage.Items) {
	for _, item := range items {
		_ = store.Delete(ctx, item.Key)
	}
}
