// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// FPath is an OS independently path handling structure
type FPath struct {
	local  bool   // if file is local
	scheme string // url scheme
	bucket string // set if remote scheme
	path   string // local file path or Storj path (without bucket), cleaned, with forward slashes
}

// New creates new FPath from the given URL
func New(url string) (p FPath, err error) {
	// Check for schema
	split := strings.SplitN(url, "://", 2)

	switch len(split) {
	case 1: // No scheme
		return parseLocalPath(split[0])
	case 2: // Has scheme
		return parseStorjURL(split[0], split[1])
	default: // Everything else is malformed
		return FPath{}, fmt.Errorf("malformed URL: %s", url)
	}
}

func parseStorjURL(scheme, bucketPath string) (FPath, error) {
	if scheme != "sj" {
		return FPath{}, fmt.Errorf("unsupported URL scheme: %s", scheme)
	}

	var p FPath
	p.scheme = scheme
	// Trim initial slash of the path and clean it, afterwards split on first slash
	split := strings.SplitN(path.Clean(strings.TrimLeft(bucketPath, "/")), "/", 2)
	if p.bucket == "." { // result from path.Clean("") or path.Clean("/")
		return FPath{}, fmt.Errorf("malformed URL: %s://%s", scheme, bucketPath)
	}

	p.bucket = split[0]
	if len(split) == 2 {
		p.path = split[1]
	}

	return p, nil
}

func parseLocalPath(path string) (FPath, error) {
	var p FPath

	p.local = true
	p.path = path

	// Check for Universal Naming Convention path (Windows)
	cprefix, err := regexp.Compile(`^\\\\\?\\(UNC\\)?`)
	if err != nil {
		return FPath{}, err
	}

	// If UNC prefix is present, omit further changes to the path
	if prefix := cprefix.FindString(p.path); prefix != "" {
		p.scheme = prefix
		p.path = strings.Replace(p.path, prefix, "", 1) // strip prefix
		return p, nil
	}

	if filepath.IsAbs(p.path) {
		return p, nil
	}

	// Ensure path is absolute
	p.path, err = filepath.Abs(p.path)
	if err != nil {
		return FPath{}, fmt.Errorf("cannot create absolute path for %s", p.path)
	}

	return p, nil
}

// Join is appends the given segment to the path
func (p FPath) Join(segment string) FPath {
	p.path = filepath.Join(p.path, segment)
	if !p.local {
		p.path = filepath.ToSlash(p.path)
	}
	return p
}

// Folder returns the parent folder of path
func (p FPath) Folder() string {
	return filepath.Dir(p.path)
}

// IsFolder returns if path is a folder
func (p FPath) IsFolder() bool {
	fileInfo, err := os.Stat(p.path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

// Base returns the last segment of the path
func (p FPath) Base() string {
	return filepath.Base(p.path)
}

// Bucket returns the first segment of path
func (p FPath) Bucket() string {
	return p.bucket
}

// BucketPath returns path prepended with the bucket name
func (p FPath) BucketPath() string {
	if !p.local && p.bucket != "" {
		return p.bucket + "/" + p.path
	}
	return ""
}

// Path returns the URL path without the scheme
func (p FPath) Path() string {
	return p.path
}

// IsLocal returns whether the path refers to local or remote location
func (p FPath) IsLocal() bool {
	return p.local
}

// HasScheme returns if the URL had a scheme
func (p FPath) HasScheme() bool {
	return p.scheme != ""
}

// Scheme returns the scheme of the URL
func (p FPath) Scheme() string {
	return p.scheme
}

// String returns the entire URL
func (p FPath) String() string {
	if p.HasScheme() {
		return p.scheme + "://" + p.BucketPath()
	}
	return p.path
}
