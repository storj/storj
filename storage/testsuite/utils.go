// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func newItem(key, value string, isPrefix bool) storage.ListItem {
	return storage.ListItem{
		Key:      storage.Key(key),
		Value:    storage.Value(value),
		IsPrefix: isPrefix,
	}
}

func cleanupItems(t testing.TB, ctx *testcontext.Context, store storage.KeyValueStore, items storage.Items) {
	bulkDeleter, ok := store.(BulkDeleter)
	if ok {
		err := bulkDeleter.BulkDelete(ctx, items)
		if err != nil {
			t.Fatalf("could not do bulk cleanup of items: %v", err)
		}
	} else {
		for _, item := range items {
			_ = store.Delete(ctx, item.Key)
		}
	}
}

// BulkImporter identifies KV storage facilities that can do bulk importing of items more
// efficiently than inserting one-by-one.
type BulkImporter interface {
	BulkImport(context.Context, storage.Iterator) error
}

// BulkDeleter identifies KV storage facilities that can delete multiple items efficiently.
type BulkDeleter interface {
	BulkDelete(context.Context, storage.Items) error
}

// BulkCleaner identifies KV storage facilities that can delete all items efficiently.
type BulkCleaner interface {
	BulkDeleteAll(ctx context.Context) error
}

type iterationTest struct {
	Name     string
	Options  storage.IterateOptions
	Expected storage.Items
}

func testIterations(t *testing.T, ctx *testcontext.Context, store storage.KeyValueStore, tests []iterationTest) {
	t.Helper()
	for _, test := range tests {
		items, err := iterateItems(ctx, store, test.Options, -1)
		if err != nil {
			t.Errorf("%s: %v", test.Name, err)
			continue
		}
		if diff := cmp.Diff(test.Expected, items, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%s: (-want +got)\n%s", test.Name, diff)
		}
	}
}

func isEmptyKVStore(tb testing.TB, ctx *testcontext.Context, store storage.KeyValueStore) bool {
	tb.Helper()
	keys, err := store.List(ctx, storage.Key(""), 1)
	if err != nil {
		tb.Fatalf("Failed to check if KeyValueStore is empty: %v", err)
	}
	return len(keys) == 0
}

type collector struct {
	Items storage.Items
	Limit int
}

func (collect *collector) include(ctx context.Context, it storage.Iterator) error {
	var item storage.ListItem
	for (collect.Limit < 0 || len(collect.Items) < collect.Limit) && it.Next(ctx, &item) {
		collect.Items = append(collect.Items, storage.CloneItem(item))
	}
	return nil
}

func iterateItems(ctx *testcontext.Context, store storage.KeyValueStore, opts storage.IterateOptions, limit int) (storage.Items, error) {
	collect := &collector{Limit: limit}
	err := store.Iterate(ctx, opts, collect.include)
	if err != nil {
		return nil, err
	}
	return collect.Items, nil
}
