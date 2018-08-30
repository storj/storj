package storage

import (
	"bytes"
	"sort"
)

func CloneKey(key Key) Key         { return append(key[:0:0], key...) }
func CloneValue(value Value) Value { return append(value[:0:0], value...) }

func SortAndCollapse(items Items, prefix []byte, delimiter byte) Items {
	sort.Sort(items)
	var result Items = items[:0]

	var currentPrefix []byte
	var prefixed bool
	for _, item := range items {
		if prefixed {
			if bytes.HasPrefix(item.Key, currentPrefix) {
				continue
			}
			prefixed = false
		}

		if p := bytes.IndexByte(item.Key[len(prefix):], delimiter); p >= 0 {
			currentPrefix = item.Key[:len(prefix)+p+1]
			prefixed = true
			result = append(result, ListItem{
				Key:      currentPrefix,
				IsPrefix: true,
			})
		} else {
			result = append(result, item)
		}
	}

	return result
}

type StaticIterator struct {
	Items Items
	Index int
}

func (it *StaticIterator) Next(item *ListItem) bool {
	if it.Index >= len(it.Items) {
		return false
	}
	*item = it.Items[it.Index]
	it.Index++
	return true
}
