package vpath

import (
	"strings"
)

type pathWalker struct {
	path     string
	consumed int
}

func newPathWalker(path string) pathWalker {
	return pathWalker{path: path}
}

func (w pathWalker) Consumed() string  { return w.path[:w.consumed] }
func (w pathWalker) Remaining() string { return w.path[w.consumed:] }
func (w pathWalker) Empty() bool       { return w.Remaining() == "" }

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
