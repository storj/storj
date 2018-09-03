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

	type Test struct {
		Name     string
		Prefix   storage.Key
		First    storage.Key
		Reverse  bool
		Expected storage.Items
	}

	tests := []Test{
		{"no limits",
			nil, nil, false,
			storage.Items{
				newItem("a", "a", false),
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"no limits reverse",
			nil, nil, true,
			storage.Items{
				newItem("h", "h", false),
				newItem("g", "g", false),
				newItem("c/", "", true),
				newItem("c", "c", false),
				newItem("b/", "", true),
				newItem("a", "a", false),
			}},

		{"at a",
			nil, storage.Key("a"), false,
			storage.Items{
				newItem("a", "a", false),
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"reverse at a",
			nil, storage.Key("a"), true,
			storage.Items{
				newItem("a", "a", false),
			}},

		{"after a",
			nil, storage.NextKey(storage.Key("a")), false,
			storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"at b",
			nil, storage.Key("b"), false,
			storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after b",
			nil, storage.NextKey(storage.Key("b")), false,
			storage.Items{
				newItem("b/", "", true),
				newItem("c", "c", false),
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after c",
			nil, storage.NextKey(storage.Key("c")), false,
			storage.Items{
				newItem("c/", "", true),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"at e",
			nil, storage.Key("e"), false,
			storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after e",
			nil, storage.NextKey(storage.Key("e")), false,
			storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"reverse after e",
			nil, storage.NextKey(storage.Key("e")), true,
			storage.Items{
				newItem("c/", "", true),
				newItem("c", "c", false),
				newItem("b/", "", true),
				newItem("a", "a", false),
			}},
		{"prefix b slash",
			storage.Key("b/"), nil, false,
			storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"prefix c slash",
			storage.Key("c/"), nil, false,
			storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "", true),
				newItem("c/1", "c/1", false),
			}},
		{"prefix c slash slash",
			storage.Key("c//"), nil, false,
			storage.Items{
				newItem("c//", "c//", false),
			}},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var err error
			if !test.Reverse {
				err = store.Iterate(test.Prefix, test.First, '/',
					checkIterator(t, test.Expected))
			} else {
				err = store.IterateReverse(test.Prefix, test.First, '/',
					checkIterator(t, test.Expected))
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}

}
