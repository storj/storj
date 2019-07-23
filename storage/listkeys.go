// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"context"
)

// ListKeys returns keys starting from first and upto limit
// limit is capped to LookupLimit
func ListKeys(ctx context.Context, store KeyValueStore, first Key, limit int) (_ Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	if limit <= 0 || limit > LookupLimit {
		limit = LookupLimit
	}

	keys := make(Keys, 0, limit)
	err = store.Iterate(ctx, IterateOptions{
		First:   first,
		Recurse: true,
	}, func(ctx context.Context, it Iterator) error {
		var item ListItem
		for ; limit > 0 && it.Next(ctx, &item); limit-- {
			if item.Key == nil {
				panic("nil key")
			}
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}
