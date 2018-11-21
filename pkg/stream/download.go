// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"

	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
)

// Download implements ReadCloser, Seeker and ReaderAt for reading from stream.
type Download struct {
	ctx     context.Context
	stream  storj.ReadOnlyStream
	streams streams.Store
	reader  io.ReadCloser
	offset  int64
}

// NewDownload creates new stream download.
func NewDownload(ctx context.Context, stream storj.ReadOnlyStream, streams streams.Store) *Download {
	return &Download{
		ctx:     ctx,
		stream:  stream,
		streams: streams,
	}
}

// Read reads up to len(data) bytes into data.
//
// If this is the first call it will read from the beginning of the stream.
// Use Seek to change the current offset for the next Read call.
//
// See io.Reader for more details.
func (download *Download) Read(data []byte) (n int, err error) {
	if download.reader == nil {
		download.resetReader(0)
	}

	n, err = download.reader.Read(data)

	download.offset += int64(n)

	return n, err
}

// ReadAt reads len(data) bytes into data starting at offset in the underlying input source.
//
// See io.ReaderAt for more details.
func (download *Download) ReadAt(data []byte, offset int64) (n int, err error) {
	reader, err := download.newReader(offset)
	if err != nil {
		return 0, err
	}
	defer utils.LogClose(reader)

	return io.ReadFull(reader, data)
}

// Seek changes the offset for the next Read call.
//
// See io.Seeker for more details.
func (download *Download) Seek(offset int64, whence int) (int64, error) {
	var off int64
	switch whence {
	case io.SeekStart:
		off = offset
	case io.SeekEnd:
		off = download.stream.Info().Size - offset
	case io.SeekCurrent:
		off += offset
	}

	err := download.resetReader(off)
	if err != nil {
		return off, err
	}

	return download.offset, nil
}

// Close closes the underlying reader and resets the offset to the beginning of the stream.
//
// The next Read call will start from the beginning of the stream.
func (download *Download) Close() error {
	if download.reader == nil {
		return nil
	}

	defer func() {
		download.reader = nil
	}()

	return download.reader.Close()
}

func (download *Download) resetReader(offset int64) error {
	err := download.Close()
	if err != nil {
		return err
	}

	download.reader, err = download.newReader(offset)
	if err != nil {
		return err
	}

	download.offset = offset

	return nil
}

func (download *Download) newReader(offset int64) (io.ReadCloser, error) {
	obj := download.stream.Info()

	rr, _, err := download.streams.Get(download.ctx, obj.Path, obj.Cipher)
	if err != nil {
		return nil, err
	}

	return rr.Range(download.ctx, offset, obj.Size-offset)
}
