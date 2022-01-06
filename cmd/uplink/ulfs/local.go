// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulloc"
)

// Local implements something close to a filesystem but backed by the local disk.
type Local struct{}

// NewLocal constructs a Local filesystem.
func NewLocal() *Local {
	return &Local{}
}

// Open returns a read ReadHandle for the given local path.
func (l *Local) Open(ctx context.Context, path string) (MultiReadHandle, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return newOSMultiReadHandle(fh)
}

// Create makes any directories necessary to create a file at path and returns a WriteHandle.
func (l *Local) Create(ctx context.Context, path string) (MultiWriteHandle, error) {
	fi, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errs.Wrap(err)
	} else if err == nil && fi.IsDir() {
		return nil, errs.New("path exists as a directory already")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, errs.Wrap(err)
	}

	// TODO: atomic rename
	fh, err := os.Create(path)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return newOSMultiWriteHandle(fh), nil
}

// Move moves file to provided path.
func (l *Local) Move(ctx context.Context, oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// Remove unlinks the file at the path. It is not an error if the file does not exist.
func (l *Local) Remove(ctx context.Context, path string, opts *RemoveOptions) error {
	if opts.isPending() {
		return nil
	}

	if err := os.Remove(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}

// List returns an ObjectIterator listing files and directories that have string prefix
// with the provided path.
func (l *Local) List(ctx context.Context, path string, opts *ListOptions) (ObjectIterator, error) {
	if opts.isPending() {
		return emptyObjectIterator{}, nil
	}

	prefix := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		prefix = path[:idx+1]
	}

	prefix, err := filepath.Abs(prefix)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	prefix += string(filepath.Separator)

	var files []os.FileInfo
	if opts.isRecursive() {
		err = filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				rel, err := filepath.Rel(prefix, path)
				if err != nil {
					return err
				}
				files = append(files, &namedFileInfo{
					FileInfo: info,
					name:     rel,
				})
			}
			return nil
		})
	} else {
		files, err = ioutil.ReadDir(prefix)
	}
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir() && files[j].IsDir() {
			return files[i].Name() < files[j].Name()
		} else if files[i].IsDir() {
			return true
		} else {
			return false
		}
	})

	var trim ulloc.Location
	if !opts.isRecursive() {
		trim = ulloc.NewLocal(prefix)
	}

	return &filteredObjectIterator{
		trim:   trim,
		filter: ulloc.NewLocal(prefix),
		iter: &fileinfoObjectIterator{
			base:  prefix,
			files: files,
		},
	}, nil
}

// IsLocalDir returns true if the path is a directory.
func (l *Local) IsLocalDir(ctx context.Context, path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// Stat returns an ObjectInfo describing the provided path.
func (l *Local) Stat(ctx context.Context, path string) (*ObjectInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Loc:           ulloc.NewLocal(path),
		Created:       fi.ModTime(),
		ContentLength: fi.Size(),
	}, nil
}

type namedFileInfo struct {
	os.FileInfo
	name string
}

func (n *namedFileInfo) Name() string { return n.name }

type fileinfoObjectIterator struct {
	base    string
	files   []os.FileInfo
	current os.FileInfo
}

func (fi *fileinfoObjectIterator) Next() bool {
	if len(fi.files) == 0 {
		return false
	}
	fi.current, fi.files = fi.files[0], fi.files[1:]
	return true
}

func (fi *fileinfoObjectIterator) Err() error { return nil }

func (fi *fileinfoObjectIterator) Item() ObjectInfo {
	name := filepath.Join(fi.base, fi.current.Name())
	isDir := fi.current.IsDir()
	if isDir {
		name += string(filepath.Separator)
	}
	return ObjectInfo{
		Loc:           ulloc.NewLocal(name),
		IsPrefix:      isDir,
		Created:       fi.current.ModTime(), // TODO: use real crtime
		ContentLength: fi.current.Size(),
	}
}
