// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package fpiece

import (
	"errors"
	"io"
	"os"
)

// Chunk - Section of data to be concurrently read
type Chunk struct {
	file       *os.File
	start      int64
	final      int64
	currentPos int64
}

// NewChunk - Create Chunk
func NewChunk(file *os.File, offset int64, length int64) (*Chunk, error) {
	if length < 0 {
		return nil, errors.New("Invalid Length")
	}

	return &Chunk{file, offset, length + offset, offset}, nil
}

// Size - Get size of Chunk
func (f *Chunk) Size() int64 {
	return f.final - f.start
}

// Close - Close file associated with chunk
func (f *Chunk) Close() error {
	return f.file.Close()
}

// Read - Concurrently read from Chunk
func (f *Chunk) Read(b []byte) (n int, err error) {
	if f.currentPos >= f.final {
		return 0, io.EOF
	}

	var readLen int64 // starts at 0
	if f.final-f.currentPos > int64(len(b)) {
		readLen = int64(len(b))
	} else {
		readLen = f.final - f.currentPos
	}

	n, err = f.file.ReadAt(b[:readLen], f.currentPos)
	f.currentPos += int64(n)
	return n, err
}

// ReadAt - Concurrently read from Chunk at specific offset
func (f *Chunk) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 || off >= f.final-f.start {
		return 0, io.EOF
	}

	off += f.start

	if max := f.final - off; int64(len(p)) > max {
		p = p[:max]
		n, err = f.file.ReadAt(p, off)
		if err == nil {
			err = io.EOF
		}
		return n, err
	}
	return f.file.ReadAt(p, off)
}

// Write - Concurrently write to Chunk
func (f *Chunk) Write(b []byte) (n int, err error) {
	if f.currentPos >= f.final {
		return 0, io.EOF
	}

	var writeLen int64 // starts at 0
	if f.final-f.currentPos > int64(len(b)) {
		writeLen = int64(len(b))
	} else {
		writeLen = f.final - f.currentPos
	}

	n, err = f.file.WriteAt(b[:writeLen], f.currentPos)
	f.currentPos += int64(n)
	return n, err
}

// WriteAt - Concurrently write to Chunk at specific offset
func (f *Chunk) WriteAt(p []byte, off int64) (n int, err error) {
	if off < 0 || off >= f.final-f.start {
		return 0, io.EOF
	}

	off += f.start

	if max := f.final - off; int64(len(p)) > max {
		p = p[:max]
		n, err = f.file.WriteAt(p, off)
		if err == nil {
			err = io.EOF
		}
		return n, err
	}
	return f.file.WriteAt(p, off)
}

var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

// Seek - Seek to position of chunk for reading/writing at specific offset
func (f *Chunk) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, errWhence
	case io.SeekStart:
		offset += f.start
	case io.SeekCurrent:
		offset += f.currentPos
	case io.SeekEnd:
		offset += f.final
	}

	// Do not seek to where somewhere outside the chunk
	if offset < f.start || offset > f.final {
		return 0, errOffset
	}

	f.currentPos = offset
	return offset - f.start, nil
}
