// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"context"
)

// ListOptions are items that are optional for the LIST method.
type ListOptions struct {
	Prefix       Key
	StartAfter   Key // StartAfter is relative to Prefix
	Recursive    bool
	IncludeValue bool
	Limit        int
}

// ListV2 lists all keys corresponding to ListOptions.
// limit is capped to LookupLimit.
//
// more indicates if the result was truncated. If false
// then the result []ListItem includes all requested keys.
// If true then the caller must call List again to get more
// results by setting `StartAfter` appropriately.
func ListV2(ctx context.Context, store KeyValueStore, opts ListOptions) (result Items, more bool, err error) {
	more, err = ListV2Iterate(ctx, store, opts, func(ctx context.Context, item *ListItem) error {
		if opts.IncludeValue {
			result = append(result, ListItem{
				Key:      CloneKey(item.Key),
				Value:    CloneValue(item.Value),
				IsPrefix: item.IsPrefix,
			})
		} else {
			result = append(result, ListItem{
				Key:      CloneKey(item.Key),
				IsPrefix: item.IsPrefix,
			})
		}
		return nil
	})
	return result, more, err
}

// ListV2Iterate lists all keys corresponding to ListOptions.
// limit is capped to LookupLimit.
//
// more indicates if the result was truncated. If false
// then the result []ListItem includes all requested keys.
// If true then the caller must call List again to get more
// results by setting `StartAfter` appropriately.
//
// The opts.IncludeValue is ignored for this func.
// The callback item will be reused for next calls.
// If the user needs the preserve the value, it must call storage.CloneValue or storage.CloneKey.
func ListV2Iterate(ctx context.Context, store KeyValueStore, opts ListOptions, fn func(context.Context, *ListItem) error) (more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	limit := opts.Limit
	if limit <= 0 || limit > store.LookupLimit() {
		limit = store.LookupLimit()
	}

	more = true

	first := opts.StartAfter
	iterate := func(ctx context.Context, it Iterator) error {
		var item ListItem
		skipFirst := true
		for ; limit > 0; limit-- {
			if !it.Next(ctx, &item) {
				more = false
				return nil
			}

			relativeKey := item.Key[len(opts.Prefix):]
			if skipFirst {
				skipFirst = false
				if relativeKey.Equal(first) {
					// skip the first element in iteration
					// if it matches the search key
					limit++
					continue
				}
			}

			task := mon.TaskNamed("handling_item")(nil)
			item.Key = relativeKey
			err := fn(ctx, &item)
			task(nil)

			if err != nil {
				return err
			}
		}

		// we still need to consume one item for the more flag
		more = it.Next(ctx, &item)
		return nil
	}

	var firstFull Key
	if !opts.StartAfter.IsZero() {
		firstFull = joinKey(opts.Prefix, opts.StartAfter)
	}
	err = store.Iterate(ctx, IterateOptions{
		Prefix:  opts.Prefix,
		First:   firstFull,
		Recurse: opts.Recursive,
		Limit:   limit,
	}, iterate)

	return more, err
}

func joinKey(a, b Key) Key {
	return append(append(Key{}, a...), b...)
}
