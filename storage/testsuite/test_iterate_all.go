// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func testIterateAll(t *testing.T, ctx *testcontext.Context, store storage.KeyValueStore) {
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
	defer cleanupItems(t, ctx, store, items)

	if err := storage.PutAll(ctx, store, items...); err != nil {
		t.Fatalf("failed to setup: %v", err)
	}

	testIterations(t, ctx, store, []iterationTest{
		{"no limits",
			storage.IterateOptions{
				Recurse: true,
			}, storage.Items{
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
		{"no limits with non-nil first",
			storage.IterateOptions{
				Recurse: true,
				First:   storage.Key(""),
			}, storage.Items{
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

		{"at a",
			storage.IterateOptions{
				First:   storage.Key("a"),
				Recurse: true,
			}, storage.Items{
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

		{"after a",
			storage.IterateOptions{
				First:   storage.NextKey(storage.Key("a")),
				Recurse: true,
			}, storage.Items{
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

		{"at b",
			storage.IterateOptions{
				First:   storage.Key("b"),
				Recurse: true,
			}, storage.Items{
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
		{"after b",
			storage.IterateOptions{
				First:   storage.NextKey(storage.Key("b")),
				Recurse: true,
			}, storage.Items{
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

		{"at c",
			storage.IterateOptions{
				First:   storage.Key("c"),
				Recurse: true,
			}, storage.Items{
				newItem("c", "c", false),
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},
		{"after c",
			storage.IterateOptions{
				First:   storage.NextKey(storage.Key("c")),
				Recurse: true,
			}, storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"at e",
			storage.IterateOptions{
				First:   storage.Key("e"),
				Recurse: true,
			}, storage.Items{
				newItem("g", "g", false),
				newItem("h", "h", false),
			}},

		{"prefix b slash",
			storage.IterateOptions{
				Prefix:  storage.Key("b/"),
				Recurse: true,
			}, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"prefix b slash at a",
			storage.IterateOptions{
				Prefix: storage.Key("b/"), First: storage.Key("a"),
				Recurse: true,
			}, storage.Items{
				newItem("b/1", "b/1", false),
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},
		{"prefix b slash at b slash 2",
			storage.IterateOptions{
				Prefix: storage.Key("b/"), First: storage.Key("b/2"),
				Recurse: true,
			}, storage.Items{
				newItem("b/2", "b/2", false),
				newItem("b/3", "b/3", false),
			}},

		{"prefix c slash",
			storage.IterateOptions{
				Prefix:  storage.Key("c/"),
				Recurse: true,
			}, storage.Items{
				newItem("c/", "c/", false),
				newItem("c//", "c//", false),
				newItem("c/1", "c/1", false),
			}},

		{"prefix c slash slash",
			storage.IterateOptions{
				Prefix:  storage.Key("c//"),
				Recurse: true,
			}, storage.Items{
				newItem("c//", "c//", false),
			}},
	})
}
