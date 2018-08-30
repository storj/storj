package storage

// ListKeys returns keys starting from first and upto limit
func ListKeys(store KeyValueStore, first Key, limit Limit) (Keys, error) {
	var keys Keys

	err := store.IterateAll(nil, first, func(it Iterator) error {
		var item ListItem
		for ; limit > 0 && it.Next(&item); limit-- {
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}
