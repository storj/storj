// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"io"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
)

// filepipe is a file backed pipe
type filepipe struct {
	file     ReadWriteCloserAt
	refcount int32

	mu     sync.Mutex
	nodata sync.Cond
	read   int64
	write  int64
	limit  int64

	writerDone bool
	writerErr  error

	readerDone bool
	readerErr  error
}

// NewFilePipe returns a pipe that uses file-system to offload memory
func NewFilePipe(tempdir string) (PipeReader, PipeWriter, error) {
	tempfile, err := ioutil.TempFile(tempdir, "filepipe")
	if err != nil {
		return nil, nil, err
	}

	pipe := &filepipe{
		file:     tempfile,
		refcount: 2,
		limit:    0xFFFFFFFF,
	}
	pipe.nodata.L = &pipe.mu

	return filePipeReader{pipe}, filePipeWriter{pipe}, nil
}

type filePipeReader struct{ *filepipe }
type filePipeWriter struct{ *filepipe }

// Close implements io.Reader Close
func (pipe filePipeReader) Close() error { return pipe.CloseWithError(io.EOF) }

// Close implements io.Writer Close
func (pipe filePipeWriter) Close() error { return pipe.CloseWithError(io.EOF) }

// CloseWithError implements closing with error
func (pipe filePipeReader) CloseWithError(err error) error {
	pipe.mu.Lock()
	if pipe.readerDone {
		pipe.mu.Unlock()
		return os.ErrClosed
	}
	pipe.readerDone = true
	pipe.readerErr = err
	pipe.mu.Unlock()

	return pipe.closeFile()
}

// CloseWithError implements closing with error
func (pipe filePipeWriter) CloseWithError(err error) error {
	pipe.mu.Lock()
	if pipe.writerDone {
		pipe.mu.Unlock()
		return os.ErrClosed
	}
	pipe.writerDone = true
	pipe.writerErr = err
	pipe.nodata.Broadcast()
	pipe.mu.Unlock()

	return pipe.closeFile()
}

// closeFile closes one side of the pipe
func (pipe *filepipe) closeFile() error {
	if atomic.AddInt32(&pipe.refcount, -1) == 0 {
		return pipe.file.Close()
	}
	return nil
}

// Write writes to the pipe returning io.EOF when blockSize is reached
func (pipe filePipeWriter) Write(data []byte) (n int, err error) {
	pipe.mu.Lock()

	// has the reader finished?
	if pipe.readerDone {
		pipe.mu.Unlock()
		return 0, pipe.readerErr
	}

	// have we closed already
	if pipe.writerDone {
		pipe.mu.Unlock()
		return 0, os.ErrClosed
	}

	// check how much do they want to write
	canWrite := pipe.limit - pipe.write

	// no more room to write
	if canWrite == 0 {
		pipe.mu.Unlock()
		return 0, io.EOF
	}

	// figure out how much to write
	toWrite := int64(len(data))
	if toWrite > canWrite {
		toWrite = canWrite
	}

	writeAt := pipe.write
	pipe.mu.Unlock()

	// write data to file
	writeAmount, err := pipe.file.WriteAt(data[:toWrite], int64(writeAt))

	pipe.mu.Lock()
	// update writing head
	pipe.write += int64(writeAmount)
	// wake up reader
	pipe.nodata.Broadcast()
	// check whether we have finished
	done := pipe.write >= pipe.limit
	pipe.mu.Unlock()

	if err == nil && done {
		err = io.EOF
	}
	return writeAmount, err
}

// Read reads from the pipe returning io.EOF when writer is closed or blockSize is reached
func (pipe filePipeReader) Read(data []byte) (n int, err error) {
	pipe.mu.Lock()
	// wait until we have something to read
	for pipe.read >= pipe.write {
		// has the writer finished?
		if pipe.writerDone {
			pipe.mu.Unlock()
			return 0, pipe.writerErr
		}

		// have we closed already
		if pipe.readerDone {
			pipe.mu.Unlock()
			return 0, os.ErrClosed
		}

		// have we run out of the limit
		if pipe.read >= pipe.limit {
			pipe.mu.Unlock()
			return 0, io.EOF
		}

		// ok, lets wait
		pipe.nodata.Wait()
	}

	// how much there's available for reading
	canRead := pipe.write - pipe.read
	// how much do they want to read?
	toRead := int64(len(data))
	if toRead > canRead {
		toRead = canRead
	}
	readAt := pipe.read
	pipe.mu.Unlock()

	// read data
	readAmount, err := pipe.file.ReadAt(data[:toRead], int64(readAt))

	pipe.mu.Lock()
	// update info on how much we have read
	pipe.read += int64(readAmount)
	done := pipe.read >= pipe.limit
	pipe.mu.Unlock()

	if err == nil && done {
		err = io.EOF
	}
	return readAmount, err
}
