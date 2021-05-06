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
	path   string
	bucket string
	key    string
	remote bool
}

// NewLocal returns a new Location that refers to a local path.
func NewLocal(path string) Location {
	return Location{path: path}
}

// NewRemote returns a new location that refers to a remote path.
func NewRemote(bucket, key string) Location {
	return Location{
		bucket: bucket,
		key:    key,
		remote: true,
	}
}

// NewStd returns a new location that refers to stdin or stdout.
func NewStd() Location {
	return Location{path: "-", key: "-"}
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

		return Location{bucket: bucket, key: key, remote: true}, nil
	}

	return Location{path: location, remote: false}, nil
}

// Std returns true if the location refers to stdin/stdout.
func (p Location) Std() bool { return p.path == "-" && p.key == "-" }

// Remote returns true if the location is remote.
func (p Location) Remote() bool { return !p.Std() && p.remote }

// Local returns true if the location is local.
func (p Location) Local() bool { return !p.Std() && !p.remote }

// String returns the string form of the location.
func (p Location) String() string {
	if p.Std() {
		return "-"
	} else if p.remote {
		return fmt.Sprintf("sj://%s/%s", p.bucket, p.key)
	}
	return p.path
}

// Key returns either the path or the object key.
func (p Location) Key() string {
	if p.remote {
		return p.key
	}
	return p.path
}

// SetKey sets the key portion of the location.
func (p Location) SetKey(s string) Location {
	if p.remote {
		p.key = s
	} else {
		p.path = s
	}
	return p
}

// Parent returns the section of the key up to and including the final slash.
func (p Location) Parent() string {
	if p.Std() {
		return ""
	} else if p.remote {
		if idx := strings.LastIndexByte(p.key, '/'); idx >= 0 {
			return p.key[:idx+1]
		}
		return ""
	}
	if idx := strings.LastIndexByte(p.path, filepath.Separator); idx >= 0 {
		return p.path[:idx+1]
	}
	return ""
}

// Base returns the last base component of the key.
func (p Location) Base() (string, bool) {
	if p.Std() {
		return "", false
	} else if p.remote {
		key := p.key
		if idx := strings.LastIndexByte(key, '/'); idx >= 0 {
			key = key[idx:]
		}
		return key, len(key) > 0
	}
	base := filepath.Base(p.path)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", false
	}
	return base, true
}

// RelativeTo returns the string that when appended to the location string
// will return a string equivalent to the passed in target location.
func (p Location) RelativeTo(target Location) (string, error) {
	if p.Std() || target.Std() {
		return "", errs.New("cannot create relative location for stdin/stdout")
	} else if target.remote != p.remote {
		return "", errs.New("cannot create remote and local relative location")
	} else if !target.remote {
		abs, err := filepath.Abs(p.path)
		if err != nil {
			return "", errs.Wrap(err)
		}
		rel, err := filepath.Rel(abs, target.path)
		if err != nil {
			return "", errs.Wrap(err)
		}
		return rel, nil
	} else if target.bucket != p.bucket {
		return "", errs.New("cannot change buckets in relative remote location")
	} else if !strings.HasPrefix(target.key, p.key) {
		return "", errs.New("cannot make relative location because keys are not prefixes")
	}
	return target.key[len(p.key):], nil
}

// AppendKey adds the key to the end of the existing key, separating with the
// appropriate slash if necessary.
func (p Location) AppendKey(key string) Location {
	if p.remote {
		p.key += key
		return p
	}

	// convert any / to the local filesystem slash if necessary
	if filepath.Separator != '/' {
		key = strings.ReplaceAll(key, "/", string(filepath.Separator))
	}

	// clean up issues with // or /../ or /./ etc.
	key = filepath.Clean(string(filepath.Separator) + key)[1:]

	p.path = filepath.Join(p.path, key)
	return p
}

// HasPrefix returns true if the passed in loc is a prefix.
func (p Location) HasPrefix(loc Location) bool {
	if p.Std() {
		return loc.Std()
	} else if p.remote != loc.remote {
		return false
	} else if !p.remote {
		return strings.HasPrefix(p.path, loc.path)
	} else if p.bucket != loc.bucket {
		return false
	}
	return strings.HasPrefix(p.key, loc.key)
}

// ListKeyName returns the full first component of the key after the provided
// prefix and a boolean indicating if the component is itself a prefix.
func (p Location) ListKeyName(prefix Location) (string, bool) {
	rem := p.Key()[len(prefix.Parent()):]
	if idx := strings.IndexByte(rem, '/'); idx >= 0 {
		return rem[:idx+1], true
	}
	return rem, false
}

// RemoveKeyPrefix removes the prefix from the key or path in the location if they
// begin with it.
func (p Location) RemoveKeyPrefix(prefix string) Location {
	if p.remote {
		p.key = strings.TrimPrefix(p.key, prefix)
	} else {
		p.path = strings.TrimPrefix(p.path, prefix)
	}
	return p
}

// RemoteParts returns the bucket and key for the location and a bool indicating
// if those values are valid because the location is remote.
func (p Location) RemoteParts() (bucket, key string, ok bool) {
	return p.bucket, p.key, p.Remote()
}

// LocalParts returns the path for the location and a bool indicating if that
// value is valid because the location is local.
func (p Location) LocalParts() (path string, ok bool) {
	return p.path, p.Local()
}

// Less returns true if the location is less than the passed in location.
func (p Location) Less(q Location) bool {
	if !p.remote && q.remote {
		return true
	} else if !q.remote && p.remote {
		return false
	}

	if p.bucket < q.bucket {
		return true
	} else if q.bucket < p.bucket {
		return false
	}

	if p.key < q.key {
		return true
	} else if q.key < p.key {
		return false
	}

	if p.path < q.path {
		return true
	} else if q.path < p.path {
		return false
	}

	return false
}
