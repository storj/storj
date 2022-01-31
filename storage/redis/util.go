// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"bytes"
	"context"
	"sort"

	"storj.io/storj/storage"
)

func escapeMatch(match []byte) []byte {
	start := 0
	escaped := []byte{}
	for i, b := range match {
		switch b {
		case '?', '*', '[', ']', '\\':
			escaped = append(escaped, match[start:i]...)
			escaped = append(escaped, '\\', b)
			start = i + 1
		}
	}
	if start == 0 {
		return match
	}

	return append(escaped, match[start:]...)
}

// sortAndCollapse sorts items and combines elements based on Delimiter.
// items will be reused and modified.
func sortAndCollapse(items storage.Items, prefix []byte) storage.Items {
	sort.Sort(items)
	result := items[:0]

	var currentPrefix []byte
	var prefixed bool
	for _, item := range items {
		if prefixed {
			if bytes.HasPrefix(item.Key, currentPrefix) {
				continue
			}
			prefixed = false
		}

		if p := bytes.IndexByte(item.Key[len(prefix):], storage.Delimiter); p >= 0 {
			currentPrefix = item.Key[:len(prefix)+p+1]
			prefixed = true
			result = append(result, storage.ListItem{
				Key:      currentPrefix,
				IsPrefix: true,
			})
		} else {
			result = append(result, item)
		}
	}

	return result
}

// StaticIterator implements an iterator over list of items.
type StaticIterator struct {
	Items storage.Items
	Index int
}

// Next returns the next item from the iterator.
func (it *StaticIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	if it.Index >= len(it.Items) {
		return false
	}
	*item = it.Items[it.Index]
	it.Index++
	return true
}
