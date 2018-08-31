// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

// ListKeys returns keys starting from first and upto limit
func ListKeys(store KeyValueStore, first Key, limit Limit) (Keys, error) {
	const unlimited = Limit(1 << 31)

	var keys Keys
	// TODO: this shouldn't be probably the case
	if limit == 0 {
		limit = unlimited
	}

	err := store.IterateAll(nil, first, func(it Iterator) error {
		var item ListItem
		for ; limit > 0 && it.Next(&item); limit-- {
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}

// ReverseListKeys returns keys starting from first and upto limit
func ReverseListKeys(store KeyValueStore, first Key, limit Limit) (Keys, error) {
	const unlimited = Limit(1 << 31)

	var keys Keys
	// TODO: this shouldn't be probably the case
	if limit == 0 {
		limit = unlimited
	}

	err := store.IterateReverseAll(nil, first, func(it Iterator) error {
		var item ListItem
		for ; limit > 0 && it.Next(&item); limit-- {
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}
