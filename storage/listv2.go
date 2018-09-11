// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"errors"
)

// ListOptions are items that are optional for the LIST method
type ListOptions struct {
	Prefix       Key
	StartAfter   Key // StartAfter is relative to Prefix
	EndBefore    Key // EndBefore is relative to Prefix
	Recursive    bool
	IncludeValue bool
	Limit        int
}

// ListV2 lists all keys corresponding to ListOptions
// limit is capped to LookupLimit
//
// more indicates if the result was truncated. If false
// then the result []ListItem includes all requested keys.
// If true then the caller must call List again to get more
// results by setting `StartAfter` or `EndBefore` appropriately.
func ListV2(store KeyValueStore, opts ListOptions) (result Items, more bool, err error) {
	if !opts.StartAfter.IsZero() && !opts.EndBefore.IsZero() {
		return nil, false, errors.New("start-after and end-before cannot be combined")
	}

	limit := opts.Limit
	if limit <= 0 || limit > LookupLimit {
		limit = LookupLimit
	}

	more = true
	reverse := !opts.EndBefore.IsZero()

	var first Key
	if !reverse {
		first = opts.StartAfter
	} else {
		first = opts.EndBefore
	}

	iterate := func(it Iterator) error {
		var item ListItem
		skipFirst := true
		for ; limit > 0; limit-- {
			if !it.Next(&item) {
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

			if opts.IncludeValue {
				result = append(result, ListItem{
					Key:      CloneKey(relativeKey),
					Value:    CloneValue(item.Value),
					IsPrefix: item.IsPrefix,
				})
			} else {
				result = append(result, ListItem{
					Key:      CloneKey(relativeKey),
					IsPrefix: item.IsPrefix,
				})
			}
		}

		// we still need to consume one item for the more flag
		more = it.Next(&item)
		return nil
	}

	var firstFull Key
	if !reverse && !opts.StartAfter.IsZero() {
		firstFull = joinKey(opts.Prefix, opts.StartAfter)
	}
	if reverse && !opts.EndBefore.IsZero() {
		firstFull = joinKey(opts.Prefix, opts.EndBefore)
	}
	err = store.Iterate(IterateOptions{
		Prefix:  opts.Prefix,
		First:   firstFull,
		Reverse: reverse,
		Recurse: opts.Recursive,
	}, iterate)

	if reverse {
		result = ReverseItems(result)
	}

	return result, more, err
}

func joinKey(a, b Key) Key {
	return append(append(Key{}, a...), b...)
}
