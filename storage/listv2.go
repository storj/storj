// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import "errors"

// More indicates if the result was truncated. If false
// then the result []ListItem includes all requested keys.
// If true then the caller must call List again to get more
// results by setting `StartAfter` or `EndBefore` appropriately.
type More bool

// ListOptions are items that are optional for the LIST method
type ListOptions struct {
	Prefix       Key
	StartAfter   Key
	EndBefore    Key
	Recursive    bool
	IncludeValue bool
	Limit        Limit
}

// ListV2 lists all keys corresponding to ListOptions
func ListV2(store KeyValueStore, opts ListOptions) (result Items, more More, err error) {
	if opts.StartAfter == nil && opts.EndBefore != nil {
		return nil, false, errors.New("not implemented")
	}

	more = More(true)
	limit := opts.Limit
	if limit == 0 {
		limit = Limit(1 << 31)
	}

	iterate := func(it Iterator) error {
		var item ListItem
		for ; limit > 0; limit-- {
			if !it.Next(&item) {
				more = false
				return nil
			}
			if opts.EndBefore != nil && !item.Key.Less(opts.EndBefore) {
				more = false
				return nil
			}

			if opts.IncludeValue {
				result = append(result, ListItem{
					Key:      CloneKey(item.Key[len(opts.Prefix):]),
					Value:    CloneValue(item.Value),
					IsPrefix: item.IsPrefix,
				})
			} else {
				result = append(result, ListItem{
					Key:      CloneKey(item.Key[len(opts.Prefix):]),
					IsPrefix: item.IsPrefix,
				})
			}
		}
		return nil
	}

	first := NextKey(opts.StartAfter)
	if opts.Recursive {
		err = store.IterateAll(opts.Prefix, first, iterate)
	} else {
		err = store.Iterate(opts.Prefix, first, '/', iterate)
	}
	return result, more, err
}
