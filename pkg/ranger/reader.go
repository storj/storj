// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"io"
	"io/ioutil"

	"storj.io/storj/internal/pkg/readcloser"
)

// A Ranger is a flexible data stream type that allows for more effective
// pipelining during seeking. A Ranger can return multiple parallel Readers for
// any subranges.
type Ranger interface {
	Size() int64
	Range(offset, length int64) io.ReadCloser
}

// A RangerCloser is a Ranger that must be closed when finished
type RangerCloser interface {
	Ranger
	io.Closer
}

// NopCloser makes an existing Ranger function as a RangerCloser
// with a no-op for Close()
func NopCloser(r Ranger) RangerCloser {
	return struct {
		Ranger
		io.Closer
	}{
		Ranger: r,
		Closer: ioutil.NopCloser(nil),
	}
}

// ByteRanger turns a byte slice into a Ranger
type ByteRanger []byte

// Size implements Ranger.Size
func (b ByteRanger) Size() int64 { return int64(len(b)) }

// Range implements Ranger.Range
func (b ByteRanger) Range(offset, length int64) io.ReadCloser {
	if offset < 0 {
		return readcloser.FatalReadCloser(Error.New("negative offset"))
	}
	if length < 0 {
		return readcloser.FatalReadCloser(Error.New("negative length"))
	}
	if offset+length > int64(len(b)) {
		return readcloser.FatalReadCloser(Error.New("buffer runoff"))
	}

	return ioutil.NopCloser(bytes.NewReader(b[offset : offset+length]))
}

type concatReader struct {
	r1 Ranger
	r2 Ranger
}

func (c *concatReader) Size() int64 {
	return c.r1.Size() + c.r2.Size()
}

func (c *concatReader) Range(offset, length int64) io.ReadCloser {
	r1Size := c.r1.Size()
	if offset+length <= r1Size {
		return c.r1.Range(offset, length)
	}
	if offset >= r1Size {
		return c.r2.Range(offset-r1Size, length)
	}
	return readcloser.MultiReadCloser(
		c.r1.Range(offset, r1Size-offset),
		readcloser.LazyReadCloser(func() io.ReadCloser {
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

func (s *subrange) Range(offset, length int64) io.ReadCloser {
	return s.r.Range(offset+s.offset, length)
}
