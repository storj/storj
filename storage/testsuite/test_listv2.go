package testsuite

import (
	"math/rand"
	"sort"
	"testing"

	"storj.io/storj/storage"
)

func testListV2(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("sample.jpg", "1", false),
		newItem("music/a-song1.mp3", "2", false),
		newItem("music/a-song2.mp3", "3", false),
		newItem("music/my-album/song3.mp3", "4", false),
		newItem("music/my-album/song4.mp3", "5", false),
		newItem("music/z-song5.mp3", "6", false),
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
		t.Skip("broken")
		got, more, err := storage.ListV2(store, storage.ListOptions{
			Recursive: true,
		})
		if more != false {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, items)
	})

	t.Run("music", func(t *testing.T) {
		t.Skip("broken")
		got, more, err := storage.ListV2(store,
			storage.ListOptions{
				Prefix: storage.Key("music/"),
			})
		if more != false {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("a-song1.mp3", "2", false),
			newItem("a-song2.mp3", "3", false),
			newItem("my-album/", "", true),
			newItem("z-song5.mp3", "6", false),
		})
	})

	t.Run("all non-recursive", func(t *testing.T) {
		t.Skip("broken")
		got, more, err := storage.ListV2(store,
			storage.ListOptions{})
		if more != false {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("sample.jpg", "1", false),
			newItem("music/", "", true),
			newItem("videos/", "", true),
		})
	})

	t.Run("end before 2", func(t *testing.T) {
		t.Skip("broken")
		got, more, err := storage.ListV2(store,
			storage.ListOptions{
				EndBefore: storage.Key("music/z-song5.mp3"),
				Limit:     2,
			})
		if more != false {
			t.Errorf("more %v", more)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, got, storage.Items{
			newItem("music/a-song2.mp3", "3", false),
			newItem("music/my-album/", "", true),
		})
	})
}
