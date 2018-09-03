// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"sort"
	"testing"

	"storj.io/storj/storage"
)

func testListV2(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("music/a-song1.mp3", "1", false),
		newItem("music/a-song2.mp3", "2", false),
		newItem("music/my-album/song3.mp3", "3", false),
		newItem("music/my-album/song4.mp3", "4", false),
		newItem("music/z-song5.mp3", "5", false),
		newItem("sample.jpg", "6", false),
		newItem("videos/movie.mkv", "7", false),
	}
	rand.Shuffle(len(items), items.Swap)

	defer cleanupItems(store, items)

	for _, item := range items {
		if err := store.Put(item.Key, item.Value); err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}
	sort.Sort(items)

	type Test struct {
		Name     string
		Options  storage.ListOptions
		More     storage.More
		Expected storage.Items
	}

	tests := []Test{
		{"all",
			storage.ListOptions{
				Recursive:    true,
				IncludeValue: true,
			},
			false, items,
		},

		{"music",
			storage.ListOptions{
				Prefix: storage.Key("music/"),
			},
			false, storage.Items{
				newItem("a-song1.mp3", "", false),
				newItem("a-song2.mp3", "", false),
				newItem("my-album/", "", true),
				newItem("z-song5.mp3", "", false),
			},
		},
		{"all non-recursive",
			storage.ListOptions{
				IncludeValue: true,
			},
			false, storage.Items{
				newItem("music/", "", true),
				newItem("sample.jpg", "6", false),
				newItem("videos/", "", true),
			},
		},
		{"end before 2 recursive",
			storage.ListOptions{
				Recursive: true,
				EndBefore: storage.Key("music/z-song5.mp3"),
				Limit:     2,
			},
			true, storage.Items{
				newItem("music/my-album/song3.mp3", "", false),
				newItem("music/my-album/song4.mp3", "", false),
			},
		},
		{"end before 2",
			storage.ListOptions{
				Prefix:    storage.Key("music/"),
				EndBefore: storage.Key("music/z-song5.mp3"),
				Limit:     2,
			},
			true, storage.Items{
				newItem("a-song2.mp3", "", false),
				newItem("my-album/", "", true),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, more, err := storage.ListV2(store, test.Options)
			if more != test.More {
				t.Errorf("more %v expected %v", more, test.More)
			}
			if err != nil {
				t.Fatal(err)
			}
			checkItems(t, got, test.Expected)
		})
	}
}
