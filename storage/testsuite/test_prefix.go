// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/storj/storage"
)

func testPrefix(t *testing.T, store storage.KeyValueStore) {
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
	defer cleanupItems(store, items)
	if err := storage.PutAll(store, items...); err != nil {
		t.Fatalf("failed to setup: %v", err)
	}

	testIterations(t, store, []iterationTest{
		{"prefix x dash b slash",
			storage.IterateOptions{
				Prefix: storage.Key("x-"), First: storage.Key("x-b"),
				Recurse: true,
			}, storage.Items{
				newItem("x-b/1", "b/1", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/3", "b/3", false),
			}},
		{"reverse prefix x dash b slash",
			storage.IterateOptions{
				Prefix: storage.Key("x-"), First: storage.Key("x-b/3"),
				Recurse: true, Reverse: true,
			}, storage.Items{
				newItem("x-b/3", "b/3", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/1", "b/1", false),
				newItem("x-a", "a", false),
			}},
		{"prefix x dash b slash",
			storage.IterateOptions{
				Prefix: storage.Key("x-"), First: storage.Key("x-b"),
			}, storage.Items{
				newItem("x-b/", "", true),
			}},
		{"reverse x dash b slash",
			storage.IterateOptions{
				Prefix: storage.Key("x-"), First: storage.Key("x-b/2"),
				Reverse: true,
			}, storage.Items{
				newItem("x-b/", "", true),
				newItem("x-a", "a", false),
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
