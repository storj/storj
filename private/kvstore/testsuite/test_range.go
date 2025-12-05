// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/kvstore"
)

func testRange(t *testing.T, ctx *testcontext.Context, store kvstore.Store) {
	err := store.Range(ctx, func(ctx context.Context, key kvstore.Key, value kvstore.Value) error {
		return errors.New("empty store")
	})
	require.NoError(t, err)

	items := kvstore.Items{
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

	if err := kvstore.PutAll(ctx, store, items...); err != nil {
		t.Fatalf("failed to setup: %v", err)
	}

	var output kvstore.Items
	err = store.Range(ctx, func(ctx context.Context, key kvstore.Key, value kvstore.Value) error {
		output = append(output, kvstore.Item{
			Key:   append([]byte{}, key...),
			Value: append([]byte{}, value...),
		})
		return nil
	})
	require.NoError(t, err)

	expected := kvstore.CloneItems(items)
	sort.Sort(expected)
	sort.Sort(output)

	require.EqualValues(t, expected, output)
}
