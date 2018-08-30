// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

// NextKey returns the successive key
func NextKey(key Key) Key {
	return append(append(key[:0:0], key...), 0)
}

// CloneKey creates a copy of key
func CloneKey(key Key) Key { return append(key[:0:0], key...) }

// CloneValue creates a copy of value
func CloneValue(value Value) Value { return append(value[:0:0], value...) }

// CloneItem creates a deep copy of item
func CloneItem(item ListItem) ListItem {
	return ListItem{
		Key:   CloneKey(item.Key),
		Value: CloneValue(item.Value),
	}
}

// CloneItems creates a deep copy of items
func CloneItems(items Items) Items {
	var result = make(Items, len(items))
	for i, item := range items {
		result[i] = CloneItem(item)
	}
	return result
}
