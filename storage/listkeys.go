// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

// ListKeys returns keys starting from first and upto limit
// limit is capped to LookupLimit
func ListKeys(store KeyValueStore, first Key, limit int) (Keys, error) {
	if limit <= 0 || limit > LookupLimit {
		limit = LookupLimit
	}

	keys := make(Keys, 0, limit)
	err := store.Iterate(IterateOptions{
		First:   first,
		Recurse: true,
	}, func(it Iterator) error {
		var item ListItem
		for ; limit > 0 && it.Next(&item); limit-- {
			if item.Key == nil {
				panic("nil key")
			}
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}

// ReverseListKeys returns keys starting from first and upto limit in reverse order
// limit is capped to LookupLimit
func ReverseListKeys(store KeyValueStore, first Key, limit int) (Keys, error) {
	if limit <= 0 || limit > LookupLimit {
		limit = LookupLimit
	}

	keys := make(Keys, 0, limit)
	err := store.Iterate(IterateOptions{
		First:   first,
		Recurse: true,
		Reverse: true,
	}, func(it Iterator) error {
		var item ListItem
		for ; limit > 0 && it.Next(&item); limit-- {
			if item.Key == nil {
				panic("nil key")
			}
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}
