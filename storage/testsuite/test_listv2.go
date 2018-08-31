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

	t.Run("all", func(t *testing.T) {
		got, more, err := storage.ListV2(store, storage.ListOptions{
			Recursive:    true,
			IncludeValue: true,
		})
		if more {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, items)
	})

	t.Run("music", func(t *testing.T) {
		got, more, err := storage.ListV2(store,
			storage.ListOptions{
				Prefix: storage.Key("music/"),
			})
		if more {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("a-song1.mp3", "", false),
			newItem("a-song2.mp3", "", false),
			newItem("my-album/", "", true),
			newItem("z-song5.mp3", "", false),
		})
	})

	t.Run("all non-recursive", func(t *testing.T) {
		got, more, err := storage.ListV2(store,
			storage.ListOptions{
				IncludeValue: true,
			})
		if more {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("music/", "", true),
			newItem("sample.jpg", "6", false),
			newItem("videos/", "", true),
		})
	})

	t.Run("end before 2 recursive", func(t *testing.T) {
		got, more, err := storage.ListV2(store,
			storage.ListOptions{
				Recursive: true,
				EndBefore: storage.Key("music/z-song5.mp3"),
				Limit:     2,
			})
		if more {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("music/my-album/song3.mp3", "", false),
			newItem("music/my-album/song4.mp3", "", false),
		})
	})

	t.Run("end before 2", func(t *testing.T) {
		got, more, err := storage.ListV2(store,
			storage.ListOptions{
				Prefix:    storage.Key("music/"),
				EndBefore: storage.Key("music/z-song5.mp3"),
				Limit:     2,
			})
		if more {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("a-song2.mp3", "", true),
			newItem("my-album/", "", true),
		})
	})
}
