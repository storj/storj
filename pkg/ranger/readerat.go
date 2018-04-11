// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"io"
)

type readerAtRanger struct {
	r    io.ReaderAt
	size int64
}

// ReaderAtRanger converts a ReaderAt with a given size to a Ranger
func ReaderAtRanger(r io.ReaderAt, size int64) Ranger {
	return &readerAtRanger{
		r:    r,
		size: size,
	}
}

func (r *readerAtRanger) Size() int64 {
	return r.size
}

type readerAtReader struct {
	r              io.ReaderAt
	offset, length int64
}

func (r *readerAtRanger) Range(offset, length int64) io.Reader {
	if offset < 0 {
		return FatalReader(Error.New("negative offset"))
	}
	if offset+length > r.size {
		return FatalReader(Error.New("buffer runoff"))
	}
	return &readerAtReader{r: r.r, offset: offset, length: length}
}

func (r *readerAtReader) Read(p []byte) (n int, err error) {
	if r.length == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.length {
		p = p[:r.length]
	}
	n, err = r.r.ReadAt(p, r.offset)
	r.offset += int64(n)
	r.length -= int64(n)
	return n, err
}
