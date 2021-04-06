// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
)

// filesystemLocal implements something close to a filesystem but backed by the local disk.
type filesystemLocal struct{}

func (l *filesystemLocal) abs(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errs.Wrap(err)
	}
	if strings.HasSuffix(path, string(filepath.Separator)) &&
		!strings.HasSuffix(abs, string(filepath.Separator)) {
		abs += string(filepath.Separator)
	}
	return abs, nil
}

func (l *filesystemLocal) Open(ctx context.Context, path string) (readHandle, error) {
	path, err := l.abs(path)
	if err != nil {
		return nil, err
	}

	fh, err := os.Open(path)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return newOSReadHandle(fh)
}

func (l *filesystemLocal) Create(ctx context.Context, path string) (writeHandle, error) {
	path, err := l.abs(path)
	if err != nil {
		return nil, err
	}

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
	return newOSWriteHandle(fh), nil
}

func (l *filesystemLocal) ListObjects(ctx context.Context, path string, recursive bool) (objectIterator, error) {
	path, err := l.abs(path)
	if err != nil {
		return nil, err
	}

	prefix := path
	if idx := strings.LastIndexByte(path, filepath.Separator); idx >= 0 {
		prefix = path[:idx+1]
	}

	var files []os.FileInfo
	if recursive {
		err = filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				files = append(files, &namedFileInfo{
					FileInfo: info,
					name:     path[len(prefix):],
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

	trim := prefix
	if recursive {
		trim = ""
	}

	return &filteredObjectIterator{
		trim:   trim,
		filter: path,
		iter: &fileinfoObjectIterator{
			base:  prefix,
			files: files,
		},
	}, nil
}

func (l *filesystemLocal) IsLocalDir(ctx context.Context, path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
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

func (fi *fileinfoObjectIterator) Item() objectInfo {
	name := filepath.Join(fi.base, fi.current.Name())
	isDir := fi.current.IsDir()
	if isDir {
		name += string(filepath.Separator)
	}

	// TODO(jeff): is this the right thing to do on windows? is there more to do?
	// convert the paths to be forward slash based because keys are supposed to always be remote
	if filepath.Separator != '/' {
		name = strings.ReplaceAll(name, string(filepath.Separator), "/")
	}

	return objectInfo{
		Loc:           Location{path: name},
		IsPrefix:      isDir,
		Created:       fi.current.ModTime(), // TODO: use real crtime
		ContentLength: fi.current.Size(),
	}
}
