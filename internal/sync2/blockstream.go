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

// BlockStream is a file backed multipipe
type BlockStream struct {
	file  *os.File
	pipes []blockPipe

	streamDone   int32
	streamClosed sync.WaitGroup
}

// blockPipe implements a single pipe in the BlockStream
type blockPipe struct {
	file   *os.File
	offset uint32
	size   uint32

	mu     sync.Mutex
	nodata sync.Cond
	read   uint32
	write  uint32

	writerDone bool
	writerErr  error

	streamDone   *int32
	streamClosed *sync.WaitGroup
}

// NewBlockStream returns a new BlockStream that is created in tempdir
// if tempdir == "" the fill will be created it into os.TempDir
func NewBlockStream(tempdir string, blockCount, blockSize int64) (*BlockStream, error) {
	tempfile, err := ioutil.TempFile(tempdir, "blockstream")
	if err != nil {
		return nil, err
	}

	err = tempfile.Truncate(blockCount * blockSize)
	if err != nil {
		return nil, err
	}

	stream := &BlockStream{
		file:  tempfile,
		pipes: make([]blockPipe, blockCount),
	}

	for i := range stream.pipes {
		pipe := &stream.pipes[i]

		pipe.streamDone = &stream.streamDone
		pipe.streamClosed = &stream.streamClosed

		pipe.file = tempfile
		pipe.offset = uint32(i) * uint32(blockSize)
		pipe.size = uint32(blockSize)

		pipe.nodata.L = &pipe.mu
	}

	return stream, nil
}

// Close closes all the Pipes and waits for them to complete, finally closing the file
func (stream *BlockStream) Close() error {
	atomic.StoreInt32(&stream.streamDone, 1)

	// wake up readers that are waiting for data
	for i := range stream.pipes {
		pipe := &stream.pipes[i]
		pipe.mu.Lock()
		pipe.nodata.Broadcast()
		pipe.mu.Unlock()
	}

	// wait for readers and writers to be closed
	stream.streamClosed.Wait()

	// close the file
	return stream.file.Close()
}

// WriteCloserWithError allows closing the Writer with an error
type WriteCloserWithError interface {
	io.WriteCloser
	CloseWithError(reason error) error
}

// Pipe returns the two ends of a block stream pipe
func (stream *BlockStream) Pipe(index int) (io.ReadCloser, WriteCloserWithError) {
	stream.streamClosed.Add(2)
	pipe := &stream.pipes[index]
	return blockPipeReader{pipe}, pipe
}

type blockPipeReader struct{ *blockPipe }

// Close implements io.Reader Close
func (pipe blockPipeReader) Close() error {
	pipe.mu.Lock()
	err := pipe.writerErr
	pipe.mu.Unlock()
	pipe.streamClosed.Done()
	return err
}

// Close implementation for io.WriteCloser
func (pipe *blockPipe) Close() error {
	return pipe.CloseWithError(nil)
}

// CloseWithError implementation for io.WriteCloser
func (pipe *blockPipe) CloseWithError(reason error) error {
	// set us as finished writing
	pipe.mu.Lock()
	if pipe.writerDone {
		pipe.mu.Unlock()
		return os.ErrClosed
	}
	pipe.writerDone = true
	pipe.writerErr = reason
	pipe.nodata.Broadcast()
	pipe.mu.Unlock()

	pipe.streamClosed.Done()
	return nil
}

// Write writes to the pipe returning io.EOF when blockSize is reached
func (pipe *blockPipe) Write(data []byte) (n int, err error) {
	pipe.mu.Lock()
	// have we closed already
	if pipe.writerDone {
		pipe.mu.Unlock()
		return 0, os.ErrClosed
	}
	// have we hit the write limit?
	if pipe.write > pipe.size {
		pipe.mu.Unlock()
		return 0, io.EOF
	}

	// check how much we can write
	canWrite := pipe.size - pipe.write
	// check how much do they want to write
	toWrite := uint32(len(data))
	if toWrite > canWrite {
		toWrite = canWrite
	}
	writeAt := pipe.offset + pipe.write
	pipe.mu.Unlock()

	// write data to file
	writeAmount, err := pipe.file.WriteAt(data[:toWrite], int64(writeAt))

	pipe.mu.Lock()
	// update writing head
	pipe.write += uint32(writeAmount)
	// wake up reader
	pipe.nodata.Broadcast()
	// check whether we have finished
	done := pipe.write == pipe.size
	pipe.mu.Unlock()

	// when we are at the size limit return io.EOF
	if err == nil && done {
		err = io.EOF
	}
	return writeAmount, err
}

// Read reads from the pipe returning io.EOF when writer is closed or blockSize is reached
func (pipe *blockPipe) Read(data []byte) (n int, err error) {
	pipe.mu.Lock()
	// wait until we have something to read
	for pipe.read >= pipe.write {
		// has the writer finished for some reason?
		if pipe.writerDone {
			err := pipe.writerErr
			if err == nil {
				err = io.EOF
			}
			pipe.mu.Unlock()
			return 0, err
		}

		// have we finished with the whole stream?
		if atomic.LoadInt32(pipe.streamDone) == 1 {
			pipe.mu.Unlock()
			return 0, io.EOF
		}

		// are we at the pipe size limit?
		if pipe.read == pipe.size {
			pipe.mu.Unlock()
			return 0, io.EOF
		}

		// ok, lets wait
		pipe.nodata.Wait()
	}

	// how much there's available for reading
	canRead := pipe.write - pipe.read
	// how much do they want to read?
	toRead := uint32(len(data))
	if toRead > canRead {
		toRead = canRead
	}
	readAt := pipe.offset + pipe.read
	pipe.mu.Unlock()

	// read data
	readAmount, err := pipe.file.ReadAt(data[:toRead], int64(readAt))

	pipe.mu.Lock()
	// update info on how much we have read
	pipe.read += uint32(readAmount)
	done := pipe.read == pipe.size
	pipe.mu.Unlock()

	// return io.EOF when we have finished reading
	if err == nil && done {
		err = io.EOF
	}
	return readAmount, err
}
