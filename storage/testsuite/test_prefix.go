// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func testPrefix(t *testing.T, ctx *testcontext.Context, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("x-a", "a", false),
		newItem("x-b/1", "b/1", false),
		newItem("x-b/2", "b/2", false),
		newItem("x-b/3", "b/3", false),
		newItem("y-c", "c", false),
		newItem("y-c/", "c/", false),
		newItem("y-c//", "c//", false),
		newItem("y-c/1", "c/1", false),
		newItem("y-g", "g", false),
		newItem("y-h", "h", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(t, ctx, store, items)

	if err := storage.PutAll(ctx, store, items...); err != nil {
		t.Fatalf("failed to setup: %v", err)
	}

	testIterations(t, ctx, store, []iterationTest{
		{"prefix x dash b slash",
			storage.IterateOptions{
				Prefix: storage.Key("x-"), First: storage.Key("x-b"),
				Recurse: true,
			}, storage.Items{
				newItem("x-b/1", "b/1", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/3", "b/3", false),
			}},
		{"prefix x dash b slash",
			storage.IterateOptions{
				Prefix: storage.Key("x-"), First: storage.Key("x-b"),
			}, storage.Items{
				newItem("x-b/", "", true),
			}},
		{"prefix y- slash",
			storage.IterateOptions{
				Prefix:  storage.Key("y-"),
				Recurse: true,
			}, storage.Items{
				newItem("y-c", "c", false),
				newItem("y-c/", "c/", false),
				newItem("y-c//", "c//", false),
				newItem("y-c/1", "c/1", false),
				newItem("y-g", "g", false),
				newItem("y-h", "h", false),
			}},
	})
}
