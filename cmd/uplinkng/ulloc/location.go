// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulloc

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
)

// Location represets a local path, a remote object, or stdin/stdout.
type Location struct {
	bucket string // if nonempty, is remote
	loc    string // key or path
	std    bool   // if refers to stdin/stdout
}

// CleanPath is used to normalize all the filepath separators, remove
// any .. or . components, and keep the trailing slash if necessary.
func CleanPath(path string) string {
	// convert path to only filepath.Separator
	path = filepath.FromSlash(path)

	// now we can use the filepath.Clean routine
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		cleaned = ""
	}

	// convert all slashes to forward slashes from now on
	cleaned = filepath.ToSlash(cleaned)

	// if cleaned at this point is either the current working
	// directory (meaning the empty string) or the root directory
	// meaning (from the docs of filepath.Clean) ends with a "/",
	// then we don't need to add a slash, so return now.
	if cleaned == "" || strings.HasSuffix(cleaned, "/") {
		return cleaned
	}

	// if the original passed in path ended with a slash, clean should, too.
	if strings.HasSuffix(path, string(filepath.Separator)) {
		cleaned += "/"
	}

	return cleaned
}

// NewLocal returns a new Location that refers to a local path.
func NewLocal(path string) Location {
	return Location{loc: CleanPath(path)}
}

// NewRemote returns a new location that refers to a remote path.
func NewRemote(bucket, key string) Location {
	return Location{bucket: bucket, loc: key}
}

// NewStd returns a new location that refers to stdin or stdout.
func NewStd() Location {
	return Location{loc: "-", std: true}
}

// Parse turns the string form of the location into the structured Location
// value and an error if it is unable to or the location is invalid.
func Parse(location string) (p Location, err error) {
	if location == "-" {
		return NewStd(), nil
	}

	// Locations, Chapter 2, Verses 9 to 21.
	//
	// And the Devs spake, saying,
	// First shalt thou find the Special Prefix "sj:".
	// Then, shalt thou count two slashes, no more, no less.
	// Two shall be the number thou shalt count,
	// and the number of the counting shall be two.
	// Three shalt thou not count, nor either count thou one,
	// excepting that thou then proceed to two.
	// Four is right out!
	// Once the number two, being the second number, be reached,
	// then interpret thou thy location as a remote location,
	// which being made of a bucket and key, shall split it.

	if strings.HasPrefix(location, "sj://") || strings.HasPrefix(location, "s3://") {
		trimmed := location[5:]                // remove the scheme
		idx := strings.IndexByte(trimmed, '/') // find the bucket index

		// handles sj:// or sj:///foo
		if len(trimmed) == 0 || idx == 0 {
			return Location{}, errs.New("invalid path: empty bucket in path: %q", location)
		}

		var bucket, key string
		if idx == -1 { // handles sj://foo
			bucket, key = trimmed, ""
		} else { // handles sj://foo/bar
			bucket, key = trimmed[:idx], trimmed[idx+1:]
		}

		return Location{bucket: bucket, loc: key}, nil
	}

	return NewLocal(location), nil
}

// Loc returns either the key or path associated with the location.
func (p Location) Loc() string { return p.loc }

// Std returns true if the location refers to stdin/stdout.
func (p Location) Std() bool { return p.std }

// Remote returns true if the location is remote.
func (p Location) Remote() bool { return !p.Std() && p.bucket != "" }

// Local returns true if the location is local.
func (p Location) Local() bool { return !p.Std() && p.bucket == "" }

// String returns the string form of the location.
func (p Location) String() string {
	if p.Std() {
		return "-"
	} else if p.Remote() {
		return fmt.Sprintf("sj://%s/%s", p.bucket, p.loc)
	}
	return p.loc
}

// Parent returns the section of the key or path up to and including the final slash.
func (p Location) Parent() string {
	if p.Std() {
		return ""
	} else if idx := strings.LastIndexByte(p.loc, '/'); idx >= 0 {
		return p.loc[:idx+1]
	}
	return ""
}

// Base returns the last base component of the key or path not including the last slash.
func (p Location) Base() (string, bool) {
	if p.Std() {
		return "", false
	} else if idx := strings.LastIndexByte(p.loc, '/'); idx >= 0 {
		p.loc = p.loc[idx+1:]
	}
	return p.loc, len(p.loc) > 0
}

// RelativeTo returns the string that when appended to the location string
// will return a string equivalent to the passed in target location.
func (p Location) RelativeTo(target Location) (string, error) {
	if p.Std() || target.Std() {
		return "", errs.New("cannot create relative location for stdin/stdout")
	} else if target.Remote() != p.Remote() {
		return "", errs.New("cannot create remote and local relative location")
	} else if target.bucket != p.bucket {
		return "", errs.New("cannot change buckets in relative remote location")
	} else if !strings.HasPrefix(target.loc, p.loc) {
		return "", errs.New("cannot make relative location because keys are not prefixes")
	}
	idx := strings.LastIndexByte(p.loc, '/') + 1
	return target.loc[idx:], nil
}

// AppendKey adds the key to the end of the existing key, separating with the
// appropriate slash if necessary.
func (p Location) AppendKey(key string) Location {
	if p.Remote() {
		p.loc += key
		return p
	}

	// clean up the key so that it can't create a location beneath p.loc
	key = CleanPath("/" + key)[1:]
	p.loc = CleanPath(p.loc + key)

	return p
}

// HasPrefix returns true if the passed in Location is a prefix.
func (p Location) HasPrefix(pre Location) bool {
	if p.Std() {
		return pre.Std()
	} else if p.Remote() != pre.Remote() {
		return false
	} else if p.bucket != pre.bucket {
		return false
	}
	return strings.HasPrefix(p.loc, pre.loc)
}

// ListKeyName returns the full first component of the key after the provided
// prefix and a boolean indicating if the component is itself a prefix.
func (p Location) ListKeyName(prefix Location) (string, bool) {
	rem := p.loc[len(prefix.Parent()):]
	if idx := strings.IndexByte(rem, '/'); idx >= 0 {
		return rem[:idx+1], true
	}
	return rem, false
}

// RemovePrefix removes the prefix from the key or path in the location if they
// begin with it.
func (p Location) RemovePrefix(prefix Location) Location {
	if !p.HasPrefix(prefix) {
		return p
	}
	p.loc = strings.TrimPrefix(p.loc, prefix.loc)
	return p
}

// RemoteParts returns the bucket and key for the location and a bool indicating
// if those values are valid because the location is remote.
func (p Location) RemoteParts() (bucket, key string, ok bool) {
	return p.bucket, p.loc, p.Remote()
}

// LocalParts returns the path for the location and a bool indicating if that
// value is valid because the location is local.
func (p Location) LocalParts() (path string, ok bool) {
	return p.loc, p.Local()
}

// Directoryish returns if the location is syntatically directoryish, meaning
// that the location component is either empty or ends with a slash.
func (p Location) Directoryish() bool {
	return !p.Std() && (p.loc == "" || p.loc[len(p.loc)-1] == '/')
}

// AsDirectoryish appends a trailing slash to the location if it is not
// already directoryish.
func (p Location) AsDirectoryish() Location {
	if p.Directoryish() || p.Std() {
		return p
	}
	p.loc += "/"
	return p
}

// Undirectoryish removes any trailing slashes from the location.
func (p Location) Undirectoryish() Location {
	p.loc = strings.TrimRight(p.loc, "/")
	return p
}

// Less returns true if the location is less than the passed in location.
func (p Location) Less(q Location) bool {
	if !p.Remote() && q.Remote() {
		return true
	} else if !q.Remote() && p.Remote() {
		return false
	}

	if p.bucket < q.bucket {
		return true
	} else if q.bucket < p.bucket {
		return false
	}

	if p.loc < q.loc {
		return true
	} else if q.loc < p.loc {
		return false
	}

	return false
}
