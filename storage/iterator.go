package storage

// IteratorFunc implements basic iterator
type IteratorFunc func(item *ListItem) bool

// Next prepares the next list item
// returns false when you reach final item
func (next IteratorFunc) Next(item *ListItem) bool { return next(item) }
