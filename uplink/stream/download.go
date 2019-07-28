// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/storage/streams"
)

// Download implements Reader, Seeker and Closer for reading from stream.
type Download struct {
	ctx     context.Context
	stream  storj.ReadOnlyStream
	streams streams.Store
	reader  io.ReadCloser
	offset  int64
	closed  bool
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
	if download.closed {
		return 0, Error.New("already closed")
	}

	if download.reader == nil {
		err = download.resetReader(0)
		if err != nil {
			return 0, err
		}
	}

	n, err = download.reader.Read(data)

	download.offset += int64(n)

	return n, err
}

// Seek changes the offset for the next Read call.
//
// See io.Seeker for more details.
func (download *Download) Seek(offset int64, whence int) (int64, error) {
	if download.closed {
		return 0, Error.New("already closed")
	}

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

// Close closes the stream and releases the underlying resources.
func (download *Download) Close() error {
	if download.closed {
		return Error.New("already closed")
	}

	download.closed = true

	if download.reader == nil {
		return nil
	}

	return download.reader.Close()
}

func (download *Download) resetReader(offset int64) error {
	if download.reader != nil {
		err := download.reader.Close()
		if err != nil {
			return err
		}
	}

	obj := download.stream.Info()

	rr, _, err := download.streams.Get(download.ctx, storj.JoinPaths(obj.Bucket.Name, obj.Path), obj.Bucket.PathCipher)
	if err != nil {
		return err
	}

	download.reader, err = rr.Range(download.ctx, offset, obj.Size-offset)
	if err != nil {
		return err
	}

	download.offset = offset

	return nil
}
