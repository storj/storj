// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"net/url"
)

// FileSource represents a trust source contained in a file on disk
type FileSource struct {
	url  string
	path string
}

// NewFileSource creates a new FileSource that loads a trust list from the
// provide file:// URL. The URL must have either an empty source or be set to
// localhost. Furthermore, it must not contain user info, query values, or a
// fragment. The path must be non-empty.
func NewFileSource(fileURL string) (*FileSource, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return nil, Error.New("invalid file source %q: not a URL: %v", fileURL, err)
	}
	if u.Scheme != "file" {
		return nil, Error.New("invalid file source %q: scheme is not supported", fileURL)
	}
	if u.Host != "" && u.Host != "localhost" {
		return nil, Error.New(`invalid file source %q: host must be empty or "localhost"`, fileURL)
	}
	if u.User != nil {
		return nil, Error.New("invalid file source %q: user info is not allowed", fileURL)
	}
	if u.RawQuery != "" {
		return nil, Error.New("invalid file source %q: query values are not allowed", fileURL)
	}
	if u.Fragment != "" {
		return nil, Error.New("invalid file source %q: fragment is not allowed", fileURL)
	}
	if u.Path == "" {
		return nil, Error.New("invalid file source %q: path is missing", fileURL)
	}

	return &FileSource{
		url:  fileURL,
		path: u.Path,
	}, nil
}

// String implements the Source interface and returns the FileSource URL
func (source *FileSource) String() string {
	return source.url
}

// Fixed implements the Source interface. It returns true.
func (source *FileSource) Fixed() bool { return true }

// FetchEntries implements the Source interface and returns entries from a
// the file source on disk. The entries returned are authoritative.
func (source *FileSource) FetchEntries(ctx context.Context) (_ []Entry, err error) {
	urls, err := LoadSatelliteURLList(ctx, source.path)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, url := range urls {
		entries = append(entries, Entry{
			SatelliteURL:  url,
			Authoritative: true,
		})
	}
	return entries, nil
}
