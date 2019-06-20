// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset

import (
	"bytes"
	"errors"
	"os"
	"time"
)

// InmemoryFileSystem defines an inmemory http.FileSystem
type InmemoryFileSystem struct {
	Root  *Node
	Index map[string]*Node
}

// Inmemory creates an InmemoryFileSystem from
func Inmemory(asset *Asset) *InmemoryFileSystem {
	return InmemoryFromNode(asset.Node())
}

// InmemoryFromNode creates an
func InmemoryFromNode(root *Node) *InmemoryFileSystem {
	fs := &InmemoryFileSystem{}
	fs.Root = root
	fs.Index = map[string]*Node{}
	fs.reindex("", "", root)
	return fs
}

// reindex inserts a node to the index
func (fs *InmemoryFileSystem) reindex(prefix, name string, node *Node) {
	fs.Index[prefix+"/"+name] = node
	for _, child := range node.Children {
		fs.reindex(prefix+"/"+name, child.Name(), child)
	}
}

// Node defines a file system node for InmemoryFileSystem
type Node struct {
	Name     string
	Size     int64
	Mode     os.FileMode
	ModTime  time.Time
	Data     []byte
	Children []*Node
	Lookup   map[string]*Node
}

// File opens the particular node as a file.
func (node *Node) File() *File {
	return &File{*bytes.NewReader(node.Data), node}
}

// File defines a readable file
type File struct {
	bytes.Reader
	*Node
}

// Readdir reads all file infos from the directory.
func (file *File) Readdir() ([]os.FileInfo, error) {
	if !file.IsDir() {
		return nil, errors.New("not a directory")
	}

	infos := []os.FileInfo{}
	for _, child := range file.Children {
		infos = append(infos, FileInfo{child})
	}

	return nil, nil
}

// Stat returns stats about the file.
func (file *File) Stat() (os.FileInfo, error) { return file.FileInfo, nil }

// Close closes the file.
func (file *File) Close() error { return nil }

// FileInfo implements file info.
type FileInfo struct{ *Node }

// Name implements os.FileInfo
func (info FileInfo) Name() string { return info.Node.Name }

// Size implements os.FileInfo
func (info FileInfo) Size() int64 { return info.Node.Size }

// Mode implements os.FileInfo
func (info FileInfo) Mode() os.FileMode { return info.Node.Mode }

// ModTime implements os.FileInfo
func (info FileInfo) ModTime() time.Time { return info.Node.ModTime }

// IsDir implements os.FileInfo
func (info FileInfo) IsDir() bool { return info.Node.Mode.IsDir() }

// Sys implements os.FileInfo
func (info FileInfo) Sys() interface{} { return nil }
