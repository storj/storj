// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// Create a set
var storjScheme = map[string]struct{}{
	"sj": {},
	"s3": {},
}

// FPath is an OS independently path handling structure
type FPath struct {
	original string // the original URL or local path
	local    bool   // if local path
	bucket   string // only for Storj URL
	path     string // only for Storj URL - the path within the bucket, cleaned from duplicated slashes
}

// New creates new FPath from the given URL
func New(p string) (FPath, error) {
	fp := FPath{original: p}

	if filepath.IsAbs(p) {
		fp.local = true
		return fp, nil
	}

	var u *url.URL
	var err error
	// Loops to ensure URL is formatted correctly and does not get malformed during url.Parse()
	for {
		u, err = url.Parse(p)
		if err != nil {
			return fp, fmt.Errorf("malformed URL: %v, use format sj://bucket/", err)
		}
		// no scheme means local path
		if u.Scheme == "" {
			fp.local = true
			return fp, nil
		}
		// not a valid scheme (s3, sj)
		if _, validScheme := storjScheme[u.Scheme]; !validScheme {
			return fp, fmt.Errorf("unsupported URL scheme: %s, use format sj://bucket/", u.Scheme)
		}
		// empty url
		if u.Host == "" && u.Path == "" {
			return fp, errors.New("no bucket specified, use format sj://bucket/")
		}
		// u.host equals the bucket name, if existing url is valid
		if u.Host != "" {
			break
		}
		// remove additional / if url.Parse() corrects from sj:/bucket to sj:///bucket
		p = strings.Replace(u.String(), ":///", "://", 1)
	}
	// port was specified but is not necessary/allowed
	if u.Port() != "" {
		return fp, errors.New("port in Storj URL is not supported, use format sj://bucket/")
	}
	// set path information from url
	fp.bucket = u.Host
	if u.Path != "" {
		fp.path = strings.TrimLeft(path.Clean(u.Path), "/")
	}

	return fp, nil
}

// Join is appends the given segment to the path
func (p FPath) Join(segment string) FPath {
	if p.local {
		p.original = filepath.Join(p.original, segment)
		return p
	}

	p.original += "/" + segment
	p.path = path.Join(p.path, segment)
	return p
}

// Base returns the last segment of the path
func (p FPath) Base() string {
	if p.local {
		return filepath.Base(p.original)
	}
	if p.path == "" {
		return ""
	}
	return path.Base(p.path)
}

// Bucket returns the first segment of path
func (p FPath) Bucket() string {
	return p.bucket
}

// Path returns the URL path without the scheme
func (p FPath) Path() string {
	if p.local {
		return p.original
	}
	return p.path
}

// IsLocal returns whether the path refers to local or remote location
func (p FPath) IsLocal() bool {
	return p.local
}

// String returns the entire URL (untouched)
func (p FPath) String() string {
	return p.original
}
