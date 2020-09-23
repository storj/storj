// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"os"

	"github.com/zeebo/errs"
)

var (
	// ErrFileSource is an error class for file source errors.
	ErrFileSource = errs.Class("file source")
)

// FileSource represents a trust source contained in a file on disk.
type FileSource struct {
	path string
}

// NewFileSource creates a new FileSource that loads a trust list from the
// given path.
func NewFileSource(path string) *FileSource {
	return &FileSource{
		path: path,
	}
}

// String implements the Source interface and returns the FileSource URL.
func (source *FileSource) String() string {
	return source.path
}

// Static implements the Source interface. It returns true.
func (source *FileSource) Static() bool { return true }

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

// LoadSatelliteURLList loads a list of Satellite URLs from a path on disk.
func LoadSatelliteURLList(ctx context.Context, path string) (_ []SatelliteURL, err error) {
	defer mon.Task()(&ctx)(&err)

	f, err := os.Open(path)
	if err != nil {
		return nil, ErrFileSource.Wrap(err)
	}
	defer func() { err = errs.Combine(err, f.Close()) }()

	urls, err := ParseSatelliteURLList(ctx, f)
	if err != nil {
		return nil, ErrFileSource.Wrap(err)
	}

	return urls, nil
}
