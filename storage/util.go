package storage

func CloneKey(key Key) Key         { return append(key[:0:0], key...) }
func CloneValue(value Value) Value { return append(value[:0:0], value...) }
