// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import "storj.io/storj/cmd/uplink/ulloc"

// filteredObjectIterator removes any iteration entries that do not begin with the filter.
// all entries must begin with the trim string which is removed before checking for the
// filter.
type filteredObjectIterator struct {
	trim   ulloc.Location
	filter ulloc.Location
	iter   ObjectIterator
}

func (f *filteredObjectIterator) Next() bool {
	for {
		if !f.iter.Next() {
			return false
		}
		loc := f.iter.Item().Loc
		if !loc.HasPrefix(f.trim) {
			return false
		}
		if loc.HasPrefix(f.filter.AsDirectoryish()) || loc == f.filter {
			return true
		}
	}
}

func (f *filteredObjectIterator) Err() error { return f.iter.Err() }

func (f *filteredObjectIterator) Item() ObjectInfo {
	item := f.iter.Item()
	item.Loc = item.Loc.RemovePrefix(f.trim)
	return item
}

// emptyObjectIterator is an objectIterator that has no objects.
type emptyObjectIterator struct{}

func (emptyObjectIterator) Next() bool       { return false }
func (emptyObjectIterator) Err() error       { return nil }
func (emptyObjectIterator) Item() ObjectInfo { return ObjectInfo{} }
