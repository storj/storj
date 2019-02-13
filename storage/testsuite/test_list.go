// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"storj.io/storj/storage"
)

func testList(t *testing.T, store storage.KeyValueStore) {
	items := storage.Items{
		newItem("path/0", "\x00\xFF\x00", false),
		newItem("path/1", "\x01\xFF\x01", false),
		newItem("path/2", "\x02\xFF\x02", false),
		newItem("path/3", "\x03\xFF\x03", false),
		newItem("path/4", "\x04\xFF\x04", false),
		newItem("path/5", "\x05\xFF\x05", false),
	}
	rand.Shuffle(len(items), items.Swap)

	defer cleanupItems(store, items)
	if err := storage.PutAll(store, items...); err != nil {
		t.Fatalf("failed to setup: %v", err)
	}

	type Test struct {
		Name     string
		Reverse  bool
		First    storage.Key
		Limit    int
		Expected storage.Keys
	}

	newKeys := func(xs ...string) storage.Keys {
		var keys storage.Keys
		for _, x := range xs {
			keys = append(keys, storage.Key(x))
		}
		return keys
	}

	tests := []Test{
		{"without key", false,
			nil, 3,
			newKeys("path/0", "path/1", "path/2")},
		{"without key, limit 0", false,
			nil, 0,
			newKeys("path/0", "path/1", "path/2", "path/3", "path/4", "path/5")},
		{"with key", false,
			storage.Key("path/2"), 3,
			newKeys("path/2", "path/3", "path/4")},
		{"without key 100", false,
			nil, 100,
			newKeys("path/0", "path/1", "path/2", "path/3", "path/4", "path/5")},
	}

	for _, test := range tests {
		var keys storage.Keys
		var err error
		keys, err = store.List(test.First, test.Limit)
		if err != nil {
			t.Errorf("%s: %s", test.Name, err)
			continue
		}
		if diff := cmp.Diff(test.Expected, keys, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%s: (-want +got)\n%s", test.Name, diff)
		}
	}
}
