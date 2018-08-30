package storage

func List(store KeyValueStore, first Key, limit Limit) (Keys, error) {
	var keys Keys

	err := store.IterateAll(nil, first, func(it Iterator) error {
		var item ListItem
		for it.Next(&item) {
			keys = append(keys, CloneKey(item.Key))
		}
		return nil
	})

	return keys, err
}
