// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"bytes"
	"testing"

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
		store.Delete(item.Key)
	}
}

func testKeysSorted(t *testing.T, keys storage.Keys) {
	t.Helper()
	if len(keys) == 0 {
		return
	}

	a := keys[0]
	for _, b := range keys[1:] {
		if b.Less(a) {
			t.Fatalf("unsorted order: %v", keys)
		}
	}
}

func checkIterator(t *testing.T, items storage.Items) func(it storage.Iterator) error {
	t.Helper()
	return func(it storage.Iterator) error {
		t.Helper()

		var got storage.ListItem
		maxErrors := 5
		for i, exp := range items {
			if !it.Next(&got) {
				t.Fatalf("%d: finished early", i)
			}

			if !got.Key.Equal(exp.Key) || !bytes.Equal(got.Value, exp.Value) || got.IsPrefix != exp.IsPrefix {
				t.Errorf("%d: mismatch {%q,%q,%v} expected {{%q,%q,%v}", i,
					got.Key, got.Value, got.IsPrefix,
					exp.Key, exp.Value, exp.IsPrefix)
				maxErrors--
				if maxErrors <= 0 {
					t.Fatal("too many errors")
					return nil
				}
			}
		}

		if it.Next(&got) {
			t.Fatalf("%d: too many, got {%q,%q,%v}", len(items),
				got.Key, got.Value, got.IsPrefix)
		}
		return nil
	}
}

func checkItems(t *testing.T, gotItems, expItems storage.Items) {
	t.Helper()

	maxErrors := 5
	n := len(gotItems)
	if n > len(expItems) {
		n = len(expItems)
	}

	for i, exp := range expItems[:n] {
		got := gotItems[i]
		if !got.Key.Equal(exp.Key) || !bytes.Equal(got.Value, exp.Value) || got.IsPrefix != exp.IsPrefix {
			t.Errorf("%d: mismatch {%q,%q,%v} exp {%q,%q,%v}", i,
				got.Key, got.Value, got.IsPrefix,
				exp.Key, exp.Value, exp.IsPrefix)
			maxErrors--
			if maxErrors <= 0 {
				break
			}
		}
	}

	if len(gotItems) != len(expItems) {
		t.Fatalf(" : invalid count, got %d exp %d", len(gotItems), len(expItems))
	}
}
