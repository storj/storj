// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ftp

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/lib/uplink"
)

// The virtual file is an example of how you can implement a purely virtual file
type virtualFile struct {
	ctx        context.Context
	object     *uplink.Object
	readOffset int64 // Reading offset
}

func (f *virtualFile) Close() error {
	return f.object.Close()
}

func (f *virtualFile) Read(buffer []byte) (byteCount int, err error) {
	defer mon.Task()(&f.ctx)(&err)
	bytesToRead := int64(len(buffer))
	if f.readOffset+bytesToRead > f.object.Meta.Size {
		bytesToRead = f.object.Meta.Size - f.readOffset
	}
	if bytesToRead <= 0 {
		return 0, io.EOF
	}
	reader, err := f.object.DownloadRange(f.ctx, f.readOffset, bytesToRead)
	if err != nil {
		return 0, err
	}
	defer func() { err = errs.Combine(err, reader.Close()) }()

	origOffset := f.readOffset
	for f.readOffset-origOffset <= bytesToRead {
		bytesRead, err := reader.Read(buffer[f.readOffset:])
		if err != nil {
			if err == io.EOF {
				f.readOffset += int64(bytesRead)
				break
			} else {
				return 0, err
			}
		}
		f.readOffset += int64(bytesRead)
	}

	if f.readOffset-origOffset != bytesToRead {
		return int(f.readOffset - origOffset), io.ErrUnexpectedEOF
	}
	return int(bytesToRead), io.EOF
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
	if f.isDir {
		return os.ModePerm | os.ModeDir
	}
	return os.ModePerm
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
