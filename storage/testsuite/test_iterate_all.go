// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/storj/storage"
)

func testIterateAll(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("a", "a", false),
		newItem("b/1", "b/1", false),
		newItem("b/2", "b/2", false),
		newItem("b/3", "b/3", false),
		newItem("c", "c", false),
		newItem("c/", "c/", false),
		newItem("c//", "c//", false),
		newItem("c/1", "c/1", false),
		newItem("g", "g", false),
		newItem("h", "h", false),
	}
	rand.Shuffle(len(items), items.Swap)
	defer cleanupItems(store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	testIterations(t, store, []iterationTest{
		{"no limits", true, false,
			nil, nil,
			storage.Items{
				newItem("a", "a", false),
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"no limits reverse", true, true,
			nil, nil,
			storage.Items{
				newItem("h", "h", false),
				newItem("g", "g", false),
				newItem("c/1", "c/1", false),
				newItem("c//", "c//", false),
				newItem("c/", "c/", false),
				newItem("c", "c", false),
				newItem("b/3", "b/3", false),
				newItem("b/2", "b/2", false),
				newItem("b/1", "b/1", false),
				newItem("a", "a", false),
			}},

		{"at a", true, false,
			nil, storage.Key("a"),
			storage.Items{
				newItem("a", "a", false),
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"at a reverse", true, true,
			nil, storage.Key("a"),
			storage.Items{
				newItem("a", "a", false),
			}},

		{"after a", true, false,
			nil, storage.NextKey(storage.Key("a")),
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"at b", true, false,
			nil, storage.Key("b"),
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after b", true, false,
			nil, storage.NextKey(storage.Key("b")),
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"at c", true, false,
			nil, storage.Key("c"),
			storage.Items{
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after c", true, false,
			nil, storage.NextKey(storage.Key("c")),
			storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"at e", true, false,
			nil, storage.Key("e"),
			storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"at e reverse", true, true,
			nil, storage.Key("e"),
			storage.Items{
				newItem("c/1", "c/1", false),
				newItem("c//", "c//", false),
				newItem("c/", "c/", false),
				newItem("c", "c", false),
				newItem("b/3", "b/3", false),
				newItem("b/2", "b/2", false),
				newItem("b/1", "b/1", false),
				newItem("a", "a", false),
			}},

		{"prefix b slash", true, false,
			storage.Key("b/"), nil,
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"prefix b slash at a", true, false,
			storage.Key("b/"), storage.Key("a"),
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"prefix b slash at b slash 2", true, false,
			storage.Key("b/"), storage.Key("b/2"),
			storage.Items{
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"reverse prefix b slash", true, true,
			storage.Key("b/"), nil,
			storage.Items{
				newItem("b/3", "b/3", false),
				newItem("b/2", "b/2", false),
				newItem("b/1", "b/1", false),
			}},
		{"reverse prefix b slash at b slash 2", true, true,
			storage.Key("b/"), storage.Key("b/2"),
			storage.Items{
				newItem("b/2", "b/2", false),
				newItem("b/1", "b/1", false),
			}},

		{"prefix c slash", true, false,
			storage.Key("c/"), nil,
			storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
			}},
		{"reverse prefix c slash", true, true,
			storage.Key("c/"), nil,
			storage.Items{
				newItem("c/1", "c/1", false),
				newItem("c//", "c//", false),
				newItem("c/", "c/", false),
			}},

		{"prefix c slash slash", true, false,
			storage.Key("c//"), nil,
			storage.Items{
				newItem("c//", "c//", false),
			}},
		{"reverse prefix c slash slash", true, true,
			storage.Key("c//"), nil,
			storage.Items{
				newItem("c//", "c//", false),
			}},
	})
}
