// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"io"
	"io/ioutil"
	"sync"
	"sync/atomic"
)

type tee struct {
	buffer ReadAtWriteAtCloser
	open   *int64

	mu      sync.Mutex
	nodata  sync.Cond
	newdata sync.Cond

	maxRequired int64
	write       int64

	writerDone bool
	writerErr  error
}

// NewTeeFile returns a tee that uses file-system to offload memory
func NewTeeFile(readers int, tempdir string) (PipeReaderAt, PipeWriter, error) {
	tempfile, err := ioutil.TempFile(tempdir, "tee")
	if err != nil {
		return nil, nil, err
	}

	handles := int64(readers + 1) // +1 for the writer

	buffer := &offsetFile{
		file: tempfile,
		open: &handles,
	}

	return newTee(buffer, &handles)
}

func newTee(buffer ReadAtWriteAtCloser, open *int64) (PipeReaderAt, PipeWriter, error) {
	tee := &tee{
		buffer: buffer,
		open:   open,
	}
	tee.nodata.L = &tee.mu
	tee.newdata.L = &tee.mu

	return teeReader{tee}, teeWriter{tee}, nil
}

type teeReader struct{ tee *tee }
type teeWriter struct{ tee *tee }

// Read reads from the tee returning io.EOF when writer is closed or bufSize is reached.
//
// It will block if the writer has not provided the data yet.
func (reader teeReader) ReadAt(data []byte, off int64) (n int, err error) {
	end := off + int64(len(data))
	tee := reader.tee

	tee.mu.Lock()

	// fail fast on writer error
	if tee.writerErr != nil && tee.writerErr != io.EOF {
		tee.mu.Unlock()
		return 0, tee.writerErr
	}

	if end > tee.maxRequired {
		tee.maxRequired = end
		tee.newdata.Broadcast()
	}

	// wait until we have all the data to read
	for end > tee.write {
		// has the writer finished?
		if tee.writerDone {
			tee.mu.Unlock()
			return 0, tee.writerErr
		}

		// ok, let's wait
		tee.nodata.Wait()
	}

	tee.mu.Unlock()

	// read data
	return tee.buffer.ReadAt(data, off)
}

// Write writes to the buffer returning io.ErrClosedPipe when limit is reached
//
// It will block until at least one reader require the data.
func (writer teeWriter) Write(data []byte) (n int, err error) {
	tee := writer.tee
	tee.mu.Lock()

	// have we closed already
	if tee.writerDone {
		tee.mu.Unlock()
		return 0, io.ErrClosedPipe
	}

	for tee.write > tee.maxRequired {
		// are all readers already closed?
		if atomic.LoadInt64(tee.open) <= 1 {
			tee.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		// wait until new data is required by any reader
		tee.newdata.Wait()
	}

	writeAt := tee.write
	tee.mu.Unlock()

	// write data to buffer
	writeAmount, err := tee.buffer.WriteAt(data, writeAt)

	tee.mu.Lock()
	// update writing head
	tee.write += int64(writeAmount)
	// wake up reader
	tee.nodata.Broadcast()
	tee.mu.Unlock()

	return writeAmount, err
}

// Close implements io.Reader Close
func (reader teeReader) Close() error { return reader.CloseWithError(nil) }

// Close implements io.Writer Close
func (writer teeWriter) Close() error { return writer.CloseWithError(nil) }

// CloseWithError implements closing with error
func (reader teeReader) CloseWithError(reason error) error {
	tee := reader.tee
	err := tee.buffer.Close()

	tee.mu.Lock()
	tee.newdata.Broadcast()
	tee.mu.Unlock()

	return err
}

// CloseWithError implements closing with error
func (writer teeWriter) CloseWithError(reason error) error {
	if reason == nil {
		reason = io.EOF
	}

	tee := writer.tee
	tee.mu.Lock()
	if tee.writerDone {
		tee.mu.Unlock()
		return io.ErrClosedPipe
	}
	tee.writerDone = true
	tee.writerErr = reason
	tee.nodata.Broadcast()
	tee.mu.Unlock()

	return tee.buffer.Close()
}
