// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func testListV2(t *testing.T, ctx *testcontext.Context, store storage.KeyValueStore) {
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
	defer cleanupItems(t, ctx, store, items)

	if err := storage.PutAll(ctx, store, items...); err != nil {
		t.Fatalf("failed to setup: %v", err)
	}

	sort.Sort(items)

	type Test struct {
		Name     string
		Options  storage.ListOptions
		More     bool
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
		{"music recursive",
			storage.ListOptions{
				Recursive: true,
				Prefix:    storage.Key("music/"),
			},
			false, storage.Items{
				newItem("a-song1.mp3", "", false),
				newItem("a-song2.mp3", "", false),
				newItem("my-album/song3.mp3", "", false),
				newItem("my-album/song4.mp3", "", false),
				newItem("z-song5.mp3", "", false),
			},
		},
		{"all non-recursive without value (default)",
			storage.ListOptions{},
			false, storage.Items{
				newItem("music/", "", true),
				newItem("sample.jpg", "", false),
				newItem("videos/", "", true),
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
		{"start after 2 recursive",
			storage.ListOptions{
				Recursive:  true,
				StartAfter: storage.Key("music/a-song1.mp3"),
				Limit:      2,
			},
			true, storage.Items{
				newItem("music/a-song2.mp3", "", false),
				newItem("music/my-album/song3.mp3", "", false),
			},
		},
		{"start after non-existing 2 recursive",
			storage.ListOptions{
				Recursive:  true,
				StartAfter: storage.Key("music/a-song15.mp3"),
				Limit:      2,
			},
			true, storage.Items{
				newItem("music/a-song2.mp3", "", false),
				newItem("music/my-album/song3.mp3", "", false),
			},
		},
		{"start after 2",
			storage.ListOptions{
				Prefix:     storage.Key("music/"),
				StartAfter: storage.Key("a-song1.mp3"),
				Limit:      2,
			},
			true, storage.Items{
				newItem("a-song2.mp3", "", false),
				newItem("my-album/", "", true),
			},
		},
	}

	for _, test := range tests {
		got, more, err := storage.ListV2(ctx, store, test.Options)
		if err != nil {
			t.Errorf("%v: %v", test.Name, err)
			continue
		}
		if more != test.More {
			t.Errorf("%v: more %v expected %v", test.Name, more, test.More)
		}
		if diff := cmp.Diff(test.Expected, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%s: (-want +got)\n%s", test.Name, diff)
		}
	}
}
