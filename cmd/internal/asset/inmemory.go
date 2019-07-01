// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path"
	"time"
)

var _ http.FileSystem = (*InmemoryFileSystem)(nil)

// InmemoryFileSystem defines an inmemory http.FileSystem
type InmemoryFileSystem struct {
	Root  *Asset
	Index map[string]*Asset
}

// Inmemory creates an InmemoryFileSystem from
func Inmemory(root *Asset) *InmemoryFileSystem {
	fs := &InmemoryFileSystem{}
	fs.Root = root
	fs.Index = map[string]*Asset{}
	fs.reindex("/", "", root)
	return fs
}

// reindex inserts a node to the index
func (fs *InmemoryFileSystem) reindex(prefix, name string, file *Asset) {
	fs.Index[path.Join(prefix, name)] = file
	for _, child := range file.Children {
		fs.reindex(path.Join(prefix, name), child.Name, child)
	}
}

// Open opens the file at the specified path.
func (fs *InmemoryFileSystem) Open(path string) (http.File, error) {
	asset, ok := fs.Index[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return asset.File(), nil
}

// File opens the particular asset as a file.
func (asset *Asset) File() *File {
	return &File{*bytes.NewReader(asset.Data), asset}
}

// File defines a readable file
type File struct {
	bytes.Reader
	*Asset
}

// Readdir reads all file infos from the directory.
func (file *File) Readdir(count int) ([]os.FileInfo, error) {
	if !file.Mode.IsDir() {
		return nil, errors.New("not a directory")
	}

	if count > len(file.Children) {
		count = len(file.Children)
	}

	infos := make([]os.FileInfo, 0, count)
	for _, child := range file.Children {
		infos = append(infos, child.stat())
	}

	return infos, nil
}

func (asset *Asset) stat() FileInfo {
	return FileInfo{
		name:    asset.Name,
		size:    int64(len(asset.Data)),
		mode:    asset.Mode,
		modTime: asset.ModTime,
	}
}

// Stat returns stats about the file.
func (file *File) Stat() (os.FileInfo, error) { return file.stat(), nil }

// Close closes the file.
func (file *File) Close() error { return nil }

// FileInfo implements file info.
type FileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name implements os.FileInfo
func (info FileInfo) Name() string { return info.name }

// Size implements os.FileInfo
func (info FileInfo) Size() int64 { return info.size }

// Mode implements os.FileInfo
func (info FileInfo) Mode() os.FileMode { return info.mode }

// ModTime implements os.FileInfo
func (info FileInfo) ModTime() time.Time { return info.modTime }

// IsDir implements os.FileInfo
func (info FileInfo) IsDir() bool { return info.mode.IsDir() }

// Sys implements os.FileInfo
func (info FileInfo) Sys() interface{} { return nil }
