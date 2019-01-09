// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"io"
	"io/ioutil"
	"math"
	"sync"
)

type tee struct {
	buffer ReadAtWriteAtCloser

	mu     sync.Mutex
	nodata sync.Cond

	write int64
	limit int64

	writerDone bool
	writerErr  error
}

// NewTeeFile returns a tee that uses file-system to offload memory
func NewTeeFile(tempdir string, readers int) (PipeReaderAt, PipeWriter, error) {
	tempfile, err := ioutil.TempFile(tempdir, "tee")
	if err != nil {
		return nil, nil, err
	}

	handles := int64(readers + 1) // +1 for the writer
	tee := &tee{
		buffer: &offsetFile{
			file: tempfile,
			open: &handles,
		},
		limit: math.MaxInt64,
	}
	tee.nodata.L = &tee.mu

	return teeReader{tee}, teeWriter{tee}, nil
}

// NewTeeMemory returns a tee that uses an in-memory buffer
func NewTeeMemory(bufSize int64) (PipeReaderAt, PipeWriter, error) {
	tee := &tee{
		buffer: make(memory, bufSize),
		limit:  bufSize,
	}
	tee.nodata.L = &tee.mu

	return teeReader{tee}, teeWriter{tee}, nil
}

type teeReader struct{ tee *tee }
type teeWriter struct{ tee *tee }

func (reader teeReader) ReadAt(data []byte, off int64) (n int, err error) {
	tee := reader.tee

	// have we run out of the limit
	if off+int64(len(data)) >= tee.limit {
		return 0, io.ErrUnexpectedEOF
	}

	tee.mu.Lock()

	// fail fast on writer error
	if tee.writerErr != nil && tee.writerErr != io.EOF {
		tee.mu.Unlock()
		return 0, tee.writerErr
	}

	// wait until we have all the data to read
	for off+int64(len(data)) > tee.write {
		// has the writer finished?
		if tee.writerDone {
			tee.mu.Unlock()
			return 0, tee.writerErr
		}
		// ok, lets wait
		tee.nodata.Wait()
	}

	tee.mu.Unlock()

	// read data
	readAmount, err := tee.buffer.ReadAt(data, off)

	done := off+int64(readAmount) >= tee.limit

	if err == nil && done {
		err = io.EOF
	}

	return readAmount, err
}

// Write writes to the buffer returning io.ErrClosedPipe when limit is reached
func (writer teeWriter) Write(data []byte) (n int, err error) {
	tee := writer.tee
	tee.mu.Lock()

	// have we closed already
	if tee.writerDone {
		tee.mu.Unlock()
		return 0, io.ErrClosedPipe
	}

	// check how much do they want to write
	canWrite := tee.limit - tee.write

	// no more room to write
	if canWrite == 0 {
		tee.mu.Unlock()
		return 0, io.ErrClosedPipe
	}

	// figure out how much to write
	toWrite := int64(len(data))
	if toWrite > canWrite {
		toWrite = canWrite
	}

	writeAt := tee.write
	tee.mu.Unlock()

	// write data to buffer
	writeAmount, err := tee.buffer.WriteAt(data[:toWrite], writeAt)

	tee.mu.Lock()
	// update writing head
	tee.write += int64(writeAmount)
	// wake up reader
	tee.nodata.Broadcast()
	// check whether we have finished
	done := tee.write >= tee.limit
	tee.mu.Unlock()

	if err == nil && done {
		err = io.ErrClosedPipe
	}

	return writeAmount, err
}

// Close implements io.Reader Close
func (reader teeReader) Close() error { return reader.CloseWithError(nil) }

// Close implements io.Writer Close
func (writer teeWriter) Close() error { return writer.CloseWithError(nil) }

// CloseWithError implements closing with error
func (reader teeReader) CloseWithError(err error) error {
	return reader.tee.buffer.Close()
}

// CloseWithError implements closing with error
func (writer teeWriter) CloseWithError(err error) error {
	if err == nil {
		err = io.EOF
	}

	tee := writer.tee
	tee.mu.Lock()
	if tee.writerDone {
		tee.mu.Unlock()
		return io.ErrClosedPipe
	}
	tee.writerDone = true
	tee.writerErr = err
	tee.nodata.Broadcast()
	tee.mu.Unlock()

	return tee.buffer.Close()
}
