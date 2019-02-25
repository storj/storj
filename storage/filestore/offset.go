// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"errors"
	"io"
	"os"
)

// blobBodyReader implements reader that is offset by blob header size
type blobBodyReader struct {
	file *os.File
}

func newBlobBodyReader(file *os.File) (blobBodyReader, error) {
	_, err := file.Seek(headerSize, io.SeekStart)
	return blobBodyReader{file}, err
}

func (r blobBodyReader) Read(p []byte) (n int, err error) {
	return r.file.Read(p)
}

func (r blobBodyReader) Close() error {
	return r.file.Close()
}

func (r blobBodyReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("invalid offset")
	}

	return r.file.ReadAt(p, off+headerSize)
}

func (r blobBodyReader) Seek(offset int64, whence int) (int64, error) {
	var pos int64
	var err error
	if whence == io.SeekStart {
		pos, err = r.file.Seek(offset+headerSize, io.SeekStart)
	} else {
		pos, err = r.file.Seek(offset, whence)
	}
	pos -= headerSize
	if err != nil {
		return -1, err
	}
	if pos < 0 {
		return -1, errors.New("invalid offset")
	}
	return pos, err
}

func (r blobBodyReader) Size() (n int64) {
	stat, _ := r.file.Stat()
	return stat.Size() - headerSize
}
