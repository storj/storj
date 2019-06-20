// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"strings"
)

//
// To avoid confusion about when paths are encrypted, unencrypted, empty or
// non existent, we create some wrapper types so that the compiler will complain
// if someone attempts to use one in the wrong context.
//

// Unencrypted is an opaque type representing an unencrypted path.
type Unencrypted struct {
	raw   string
	valid bool
}

// Encrypted is an opaque type representing an encrypted path.
type Encrypted struct {
	raw   string
	valid bool
}

//
// unencrypted paths
//

// NewUnencrypted takes a raw unencrypted path and returns it wrapped.
func NewUnencrypted(raw string) Unencrypted {
	return Unencrypted{raw: raw, valid: true}
}

// Valid returns if the unencrypted path is valid, which is different from being empty.
func (path Unencrypted) Valid() bool {
	return path.valid
}

// Raw returns the original raw path for the Unencrypted.
func (path Unencrypted) Raw() string {
	return path.raw
}

// String returns a human readable form of the Unencrypted.
func (path Unencrypted) String() string {
	if !path.valid {
		return "<unencrypted-invalid-path>"
	}
	return path.Raw()
}

// Consume attempts to remove the prefix from the Unencrypted path. The
// resultant path is invalid if the prefix could not be consumed. The
// invalid prefix can always be consumed.
func (path Unencrypted) Consume(prefix Unencrypted) Unencrypted {
	if !prefix.Valid() || !path.Valid() {
		return path
	} else if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewUnencrypted(path.raw[len(prefix.raw):])
	}
	return Unencrypted{}
}

// Iterator returns an iterator over the components of the Unencrypted.
func (path Unencrypted) Iterator() Iterator {
	return Iterator{raw: path.raw, lastEmpty: path.Valid()}
}

// Less returns true if 'path' should be sorted earlier than 'other'
func (path Unencrypted) Less(other Unencrypted) bool {
	if !other.valid {
		return false
	}
	if !path.valid {
		return true
	}
	return path.raw < other.raw
}

//
// encrypted path
//

// NewEncrypted takes a raw encrypted path and returns it wrapped.
func NewEncrypted(raw string) Encrypted {
	return Encrypted{raw: raw, valid: true}
}

// Valid returns if the encrypted path is valid, which is different from being empty.
func (path Encrypted) Valid() bool {
	return path.valid
}

// Raw returns the original path for the Encrypted.
func (path Encrypted) Raw() string {
	return path.raw
}

// String returns a human readable form of the Encrypted.
func (path Encrypted) String() string {
	if !path.valid {
		return "<encrypted-invalid-path>"
	}
	return path.Raw()
}

// Consume attempts to remove the prefix from the Encrypted path. The
// resultant path is invalid if the prefix could not be consumed. The
// invalid prefix can always be consumed.
func (path Encrypted) Consume(prefix Encrypted) Encrypted {
	if !prefix.Valid() || !path.Valid() {
		return path
	} else if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewEncrypted(path.raw[len(prefix.raw):])
	}
	return Encrypted{}
}

// Iterator returns an iterator over the components of the Encrypted.
func (path Encrypted) Iterator() Iterator {
	return Iterator{raw: path.raw, lastEmpty: path.Valid()}
}

// Less returns true if 'path' should be sorted earlier than 'other'
func (path Encrypted) Less(other Encrypted) bool {
	if !other.valid {
		return false
	}
	if !path.valid {
		return true
	}
	return path.raw < other.raw
}

//
// path component iteration
//

// Iterator allows one to efficiently iterate over components of a path.
type Iterator struct {
	raw         string
	consumed    int
	hasConsumed bool
	lastEmpty   bool
}

// NewIterator returns an Iterator for components of the provided raw path.
func NewIterator(raw string) Iterator {
	return Iterator{raw: raw, lastEmpty: true}
}

// Consumed reports how much of the path has been consumed (if any).
func (pi Iterator) Consumed() (string, bool) { return pi.raw[:pi.consumed], pi.hasConsumed }

// Remaining reports how much of the path is remaining.
func (pi Iterator) Remaining() string { return pi.raw[pi.consumed:] }

// Done reports if the path has been fully consumed.
func (pi Iterator) Done() bool { return len(pi.raw) == pi.consumed && !pi.lastEmpty }

// Next returns the first component of the path, consuming it.
func (pi *Iterator) Next() string {
	if pi.Done() {
		return ""
	}
	pi.hasConsumed = true

	rem := pi.Remaining()
	if index := strings.IndexByte(rem, '/'); index == -1 {
		pi.consumed += len(rem)
		pi.lastEmpty = false
		return rem
	} else {
		pi.consumed += index + 1
		pi.lastEmpty = index == len(rem)-1
		return rem[:index]
	}
}
