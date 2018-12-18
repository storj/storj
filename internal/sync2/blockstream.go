// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"fmt"
	"io/ioutil"
	"sync/atomic"
)

// MultiFilePipe is a multipipe backed by a single file
type MultiFilePipe struct {
	pipes    []filepipe
	refcount int64
}

type offsetFile struct {
	file     ReadWriteCloserAt
	offset   int64
	refcount *int64
}

// NewMultiPipeFile returns a new MultiFilePipe that is created in tempdir
// if tempdir == "" the fill will be created it into os.TempDir
func NewMultiPipeFile(tempdir string, blockCount, blockSize int64) (*MultiFilePipe, error) {
	tempfile, err := ioutil.TempFile(tempdir, "multifilepipe")
	if err != nil {
		return nil, err
	}

	err = tempfile.Truncate(blockCount * blockSize)
	if err != nil {
		closeErr := tempfile.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("%v/%v", err, closeErr)
		}
		return nil, err
	}

	multipipe := &MultiFilePipe{
		pipes:    make([]filepipe, blockCount),
		refcount: blockCount,
	}

	for i := range multipipe.pipes {
		pipe := &multipipe.pipes[i]
		pipe.file = offsetFile{
			file:     tempfile,
			offset:   int64(i) * blockSize,
			refcount: &multipipe.refcount,
		}
		pipe.limit = blockSize
		pipe.nodata.L = &pipe.mu
	}

	return multipipe, nil
}

// ReadAt implements io.ReaderAt methods
func (file offsetFile) ReadAt(data []byte, at int64) (amount int, err error) {
	return file.file.ReadAt(data, file.offset+at)
}

// WriteAt implements io.WriterAt methods
func (file offsetFile) WriteAt(data []byte, at int64) (amount int, err error) {
	return file.file.WriteAt(data, file.offset+at)
}

// Close implements io.Closer methods
func (file offsetFile) Close() error {
	if atomic.AddInt64(file.refcount, -1) == 0 {
		return file.Close()
	}
	return nil
}

// Pipe returns the two ends of a block stream pipe
func (multipipe *MultiFilePipe) Pipe(index int) (PipeReader, PipeWriter) {
	pipe := &multipipe.pipes[index]
	return filePipeReader{pipe}, filePipeWriter{pipe}
}
