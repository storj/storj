// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/storj/storage"
)

func testIterate(t *testing.T, store storage.KeyValueStore) {
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

	testIterations(t, store, []IterationTest{
		{"no limits", false, false,
			nil, nil,
			storage.Items{
				newItem("a", "a", false),
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"no limits reverse", false, true,
			nil, nil,
			storage.Items{
				newItem("h", "h", false),
				newItem("g", "g", false),
				newItem("c/", "", true),
				newItem("c", "c", false),
				newItem("b/", "", true),
				newItem("a", "a", false),
			}},

		{"at a", false, false,
			nil, storage.Key("a"),
			storage.Items{
				newItem("a", "a", false),
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"reverse at a", false, true,
			nil, storage.Key("a"),
			storage.Items{
				newItem("a", "a", false),
			}},

		{"after a", false, false,
			nil, storage.NextKey(storage.Key("a")),
			storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"at b", false, false,
			nil, storage.Key("b"),
			storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after b", false, false,
			nil, storage.NextKey(storage.Key("b")),
			storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after c", false, false,
			nil, storage.NextKey(storage.Key("c")),
			storage.Items{
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"at e", false, false,
			nil, storage.Key("e"),
			storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after e", false, false,
			nil, storage.NextKey(storage.Key("e")),
			storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"reverse after e", false, true,
			nil, storage.NextKey(storage.Key("e")),
			storage.Items{
				newItem("c/", "", true),
				newItem("c", "c", false),
				newItem("b/", "", true),
				newItem("a", "a", false),
			}},
		{"prefix b slash", false, false,
			storage.Key("b/"), nil,
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"prefix c slash", false, false,
			storage.Key("c/"), nil,
			storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "", true),
				newItem("c/1", "c/1", false),
			}},
		{"prefix c slash slash", false, false,
			storage.Key("c//"), nil,
			storage.Items{
				newItem("c//", "c//", false),
			}},
	})
}
