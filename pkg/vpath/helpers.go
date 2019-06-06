package vpath

import (
	"strings"
)

// pathWalker efficiently pops components off of a path and keeps track of the
// amount of path consumed and remaining.
type pathWalker struct {
	path     string
	consumed int
}

// newPathWalker constructs a pathWalker from a path.
func newPathWalker(path string) pathWalker {
	return pathWalker{path: path}
}

// Consumed reports how much of the path has been consumed.
func (w pathWalker) Consumed() string { return w.path[:w.consumed] }

// Remaining reports how much of the path is remaining.
func (w pathWalker) Remaining() string { return w.path[w.consumed:] }

// Empty reports if the path has been fully consumed.
func (w pathWalker) Empty() bool { return w.Remaining() == "" }

// Next returns the first component of the path, consuming it.
func (w *pathWalker) Next() string {
	rem := w.Remaining()
	if index := strings.IndexByte(rem, '/'); index == -1 {
		w.consumed += len(rem)
		return rem
	} else {
		w.consumed += index + 1
		return rem[:index]
	}
}
