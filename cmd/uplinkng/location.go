// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

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

func parseLocation(location string) (p Location, err error) {
	if location == "-" {
		return Location{path: "-", key: "-"}, nil
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
