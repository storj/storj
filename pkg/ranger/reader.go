// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"io"
)

// A Ranger is a flexible data stream type that allows for more effective
// pipelining during seeking. A Ranger can return multiple parallel Readers for
// any subranges.
type Ranger interface {
	Size() int64
	Range(offset, length int64) io.Reader
}

// FatalReader returns a Reader that always fails with err.
func FatalReader(err error) io.Reader {
	return &fatalReader{Err: err}
}

type fatalReader struct {
	Err error
}

func (f *fatalReader) Read(p []byte) (n int, err error) {
	return 0, f.Err
}

// ByteRanger turns a byte slice into a Ranger
type ByteRanger []byte

func (b ByteRanger) Size() int64 { return int64(len(b)) }

func (b ByteRanger) Range(offset, length int64) io.Reader {
	if offset < 0 {
		return FatalReader(Error.New("negative offset"))
	}
	if offset+length > int64(len(b)) {
		return FatalReader(Error.New("buffer runoff"))
	}

	return bytes.NewReader(b[offset : offset+length])
}

type concatReader struct {
	r1 Ranger
	r2 Ranger
}

func (c *concatReader) Size() int64 {
	return c.r1.Size() + c.r2.Size()
}

func (c *concatReader) Range(offset, length int64) io.Reader {
	r1Size := c.r1.Size()
	if offset+length <= r1Size {
		return c.r1.Range(offset, length)
	}
	if offset >= r1Size {
		return c.r2.Range(offset-r1Size, length)
	}
	return io.MultiReader(
		c.r1.Range(offset, r1Size-offset),
		LazyReader(func() io.Reader {
			return c.r2.Range(0, length-(r1Size-offset))
		}))
}

func concat2(r1, r2 Ranger) Ranger {
	return &concatReader{r1: r1, r2: r2}
}

// Concat concatenates Rangers
func Concat(r ...Ranger) Ranger {
	switch len(r) {
	case 0:
		return ByteRanger(nil)
	case 1:
		return r[0]
	case 2:
		return concat2(r[0], r[1])
	default:
		mid := len(r) / 2
		return concat2(Concat(r[:mid]...), Concat(r[mid:]...))
	}
}

type lazyReader struct {
	fn func() io.Reader
	r  io.Reader
}

// LazyReader returns an Reader that doesn't initialize the backing Reader
// until the first Read.
func LazyReader(reader func() io.Reader) io.Reader {
	return &lazyReader{fn: reader}
}

func (l *lazyReader) Read(p []byte) (n int, err error) {
	if l.r == nil {
		l.r = l.fn()
		l.fn = nil
	}
	return l.r.Read(p)
}

type subrange struct {
	r              Ranger
	offset, length int64
}

// Subrange returns a subset of a Ranger.
func Subrange(data Ranger, offset, length int64) (Ranger, error) {
	dSize := data.Size()
	if offset < 0 || offset > dSize {
		return nil, Error.New("invalid offset")
	}
	if length+offset > dSize {
		return nil, Error.New("invalid length")
	}
	return &subrange{r: data, offset: offset, length: length}, nil
}

func (s *subrange) Size() int64 {
	return s.length
}

func (s *subrange) Range(offset, length int64) io.Reader {
	return s.r.Range(offset+s.offset, length)
}
