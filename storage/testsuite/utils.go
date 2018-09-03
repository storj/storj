// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"testing"

	"storj.io/storj/storage"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	Recurse  bool
	Reverse  bool
	Prefix   storage.Key
	First    storage.Key
	Expected storage.Items
}

func testIterations(t *testing.T, store storage.KeyValueStore, tests []iterationTest) {
	t.Helper()
	for _, test := range tests {
		var err error
		collect := &collector{}
		if test.Recurse {
			if !test.Reverse {
				err = store.IterateAll(test.Prefix, test.First, collect.include)
			} else {
				err = store.IterateReverseAll(test.Prefix, test.First, collect.include)
			}
		} else {
			if !test.Reverse {
				err = store.Iterate(test.Prefix, test.First, '/', collect.include)
			} else {
				err = store.IterateReverse(test.Prefix, test.First, '/', collect.include)
			}
		}
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
