// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

var (
	closingReader = errs.Class("Error closing reader")
	closingWriter = errs.Class("Error closing writer")
)

// The virtual file is an example of how you can implement a purely virtual file
type virtualFile struct {
	ctx context.Context

	path   string
	cipher storj.CipherSuite
	size   int64

	reader io.ReadCloser
	writer io.WriteCloser
	bucket *uplink.Bucket

	writeOffset int64 // Writer offset
	readOffset  int64 // Reader offset
	flag        int   // determines read/write/append
}

func (f *virtualFile) Size() int64 {
	if f.writeOffset != 0 {
		return f.writeOffset
	}
	return 0
}

func (f *virtualFile) Close() (err error) {
	if f.writer != nil {
		err = errs.Combine(err, closingWriter.Wrap(f.writer.Close()))
		f.writer = nil
	}
	return err
}

func (f *virtualFile) Writer() (_ io.WriteCloser, err error) {
	if f.writer == nil {
		f.writer, err = f.bucket.NewWriter(f.ctx, f.path, &uplink.UploadOptions{})
	}
	return f.writer, err
}

func (f *virtualFile) Read(buffer []byte) (byteCount int, err error) {
	defer mon.Task()(&f.ctx)(&err)

	bytesToRead := int64(len(buffer))
	if f.readOffset+bytesToRead > f.Size() {
		bytesToRead = f.Size() - f.readOffset
	}
	if bytesToRead <= 0 {
		return 0, io.EOF
	}
	reader, err := f.bucket.DownloadRange(f.ctx, storj.Path(f.path), f.readOffset, bytesToRead)
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

func (f *virtualFile) Seek(n int64, w int) (_ int64, err error) {
	switch w {
	case os.SEEK_SET:
		f.readOffset = n
	case os.SEEK_CUR:
		f.readOffset += n
	case os.SEEK_END:
		f.readOffset = f.Size() - n
	default:
		err = fmt.Errorf("Bad seek whence")
	}
	return f.readOffset, err
}

func (f *virtualFile) Write(buffer []byte) (int, error) {
	writer, err := f.Writer()
	if err != nil {
		return 0, err
	}
	zap.S().Debug("Starts writting: ", f.path)
	written, err := writer.Write(buffer)
	if err != nil {
		return 0, err
	}
	f.readOffset += int64(written)
	zap.S().Debug("Stops writting: ", f.path)
	return written, nil
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
