// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ftp

import (
	"io"
	"os"
	"time"
)

// The virtual file is an example of how you can implement a purely virtual file
type virtualFile struct {
	content    []byte // Content of the file
	readOffset int    // Reading offset
}

func (f *virtualFile) Close() error {
	return nil
}

func (f *virtualFile) Read(buffer []byte) (int, error) {
	n := copy(buffer, f.content[f.readOffset:])
	f.readOffset += n
	if n == 0 {
		return 0, io.EOF
	}

	return n, nil
}

func (f *virtualFile) Seek(n int64, w int) (int64, error) {
	return 0, nil
}

func (f *virtualFile) Write(buffer []byte) (int, error) {
	return 0, nil
}

type virtualFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (f virtualFileInfo) Name() string {
	return f.name
}

func (f virtualFileInfo) Size() int64 {
	return f.size
}

func (f virtualFileInfo) Mode() os.FileMode {
	return os.FileMode(0666)
}

func (f virtualFileInfo) IsDir() bool {
	return f.isDir
}

func (f virtualFileInfo) ModTime() time.Time {
	return f.modTime
}

func (f virtualFileInfo) Sys() interface{} {
	return nil
}
