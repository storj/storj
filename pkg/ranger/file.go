// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"context"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/zeebo/errs"
)

type fileRanger struct {
	path string
	size int64
}

// FileRanger returns a Ranger from a path.
func FileRanger(path string) (Ranger, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &fileRanger{path: path, size: info.Size()}, nil
}

func (rr *fileRanger) Size() int64 {
	return rr.size
}

func (rr *fileRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, Error.New("negative offset")
	}
	if length < 0 {
		return nil, Error.New("negative length")
	}
	if offset+length > rr.size {
		return nil, Error.New("range beyond end")
	}

	fh, err := os.Open(rr.path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	_, err = fh.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, fh.Close()))
	}

	return &fileReader{fh, length}, nil
}

// fileReader implements limit reader with io.EOF only on last read.
type fileReader struct {
	file      *os.File
	remaining int64
}

// Read reads from the underlying file.
func (reader *fileReader) Read(data []byte) (n int, err error) {
	if reader.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(data)) > reader.remaining {
		data = data[0:reader.remaining]
	}
	jitter()
	n, err = reader.file.Read(data)
	jitter()
	reader.remaining -= int64(n)
	return n, err
}

// Close closes the underlying file.
func (reader *fileReader) Close() error {
	return reader.file.Close()
}

func jitter() {
	time.Sleep(time.Duration(rand.Intn(3)*100) * time.Microsecond * 0)
}
