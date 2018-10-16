// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"storj.io/storj/storage"
)

func newItem(key, value string, isPrefix bool) storage.ListItem {
	return storage.ListItem{
		Key:      storage.Key(key),
		Value:    storage.Value(value),
		IsPrefix: isPrefix,
	}
}

func cleanupItems(store storage.KeyValueStore, items storage.Items) {
	for _, item := range items {
		_ = store.Delete(item.Key)
	}
}

type iterationTest struct {
	Name     string
	Options  storage.IterateOptions
	Expected storage.Items
}

func testIterations(t *testing.T, store storage.KeyValueStore, tests []iterationTest) {
	t.Helper()
	for _, test := range tests {
		collect := &collector{}
		err := store.Iterate(test.Options, collect.include)
		if err != nil {
			t.Errorf("%s: %v", test.Name, err)
			continue
		}
		if diff := cmp.Diff(test.Expected, collect.Items, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%s: (-want +got)\n%s", test.Name, diff)
		}
	}
}

type collector struct {
	Items storage.Items
}

func (collect *collector) include(it storage.Iterator) error {
	var item storage.ListItem
	for it.Next(&item) {
		collect.Items = append(collect.Items, storage.CloneItem(item))
	}
	return nil
}
