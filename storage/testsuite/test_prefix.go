// Copyright (C) 2018 Storj Labs, Inc.
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

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	testIterations(t, store, []IterationTest{
		{"prefix x dash b slash", true, false,
			storage.Key("x-"), storage.Key("x-b"),
			storage.Items{
				newItem("x-b/1", "b/1", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/3", "b/3", false),
			}},
		{"reverse prefix x dash b slash", true, true,
			storage.Key("x-"), storage.Key("x-b/3"),
			storage.Items{
				newItem("x-b/3", "b/3", false),
				newItem("x-b/2", "b/2", false),
				newItem("x-b/1", "b/1", false),
				newItem("x-a", "a", false),
			}},
		{"prefix x dash b slash", false, false,
			storage.Key("x-"), storage.Key("x-b"),
			storage.Items{
				newItem("x-b/", "", true),
			}},
		{"reverse x dash b slash", false, true,
			storage.Key("x-"), storage.Key("x-b/2"),
			storage.Items{
				newItem("x-b/", "", true),
				newItem("x-a", "a", false),
			}},
		{"prefix y- slash", true, false,
			storage.Key("y-"), nil,
			storage.Items{
				newItem("y-c", "c", false),
				newItem("y-c/", "c/", false),
				newItem("y-c//", "c//", false),
				newItem("y-c/1", "c/1", false),
				newItem("y-g", "g", false),
				newItem("y-h", "h", false),
			}},
	})
}
