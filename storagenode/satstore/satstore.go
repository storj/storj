// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

var mon = monkit.Package()

// SatelliteStore is a helper that allows atomically writing satellite scoped files into a directory.
type SatelliteStore struct {
	dir string
	ext string
	mu  sync.Mutex
}

// NewSatelliteStore returns a SatelliteStore rooted at the given directory and will append the
// files with the given extension. The files will be named like `<dir>/<satellite id>.<ext>`.
func NewSatelliteStore(dir, ext string) *SatelliteStore {
	return &SatelliteStore{
		dir: dir,
		ext: ext,
	}
}

// Set atomically writes the data for the given satellite.
func (s *SatelliteStore) Set(ctx context.Context, satellite storj.NodeID, data []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	s.mu.Lock()
	defer s.mu.Unlock()

	dstPath := filepath.Join(s.dir, satellite.String()+"."+s.ext)
	tmpPath := dstPath + ".tmp"

	if err := os.MkdirAll(s.dir, 0777); err != nil {
		return errs.Wrap(err)
	}

	fh, err := os.Create(tmpPath)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	if _, err := fh.Write(data); err != nil {
		return errs.Wrap(err)
	}
	if err := fh.Sync(); err != nil {
		return errs.Wrap(err)
	}
	if err := fh.Close(); err != nil {
		return errs.Wrap(err)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		return errs.Wrap(err)
	}
	if dir, err := os.Open(s.dir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	return nil
}

// Range calls the callback for every satellite, recording any errors returned by either the
// callback or the iteration process into the returned error value.
func (s *SatelliteStore) Range(cb func(storj.NodeID, []byte) error) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return errs.Wrap(err)
	}

	suffix := "." + s.ext

	var eg errs.Group
	for _, entry := range entries {
		eg.Add(func() error {
			name, ok := strings.CutSuffix(entry.Name(), suffix)
			if !ok {
				return nil
			}
			satellite, err := storj.NodeIDFromString(name)
			if err != nil {
				return errs.Wrap(err)
			}
			data, err := s.getLocked(satellite)
			if err != nil {
				return errs.Wrap(err)
			}
			return errs.Wrap(cb(satellite, data))
		}())
	}
	return eg.Err()
}

// Get returns the data for the given satellite returning any errors. There will be an error if
// the satellite does not exist.
func (s *SatelliteStore) Get(ctx context.Context, satellite storj.NodeID) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getLocked(satellite)
}

func (s *SatelliteStore) getLocked(satellite storj.NodeID) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, satellite.String()+"."+s.ext))
	return data, errs.Wrap(err)
}
