// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"

	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

// Upload implements Writer and Closer for writing to stream.
type Upload struct {
	ctx        context.Context
	stream     storj.MutableStream
	streams    streams.Store
	pathCipher storj.Cipher
	writer     io.WriteCloser
	closed     bool
	done       chan struct{}
}

// NewUpload creates new stream upload.
func NewUpload(ctx context.Context, stream storj.MutableStream, streams streams.Store, pathCipher storj.Cipher) *Upload {
	reader, writer := io.Pipe()
	done := make(chan struct{})

	go func() {
		defer close(done)

		obj := stream.Info()
		_, err := streams.Put(ctx, storj.JoinPaths(obj.Bucket, obj.Path), pathCipher, reader, obj.Metadata, obj.Expires)
		if err != nil {
			_ = reader.CloseWithError(err)
		}
	}()

	return &Upload{
		ctx:        ctx,
		stream:     stream,
		streams:    streams,
		pathCipher: pathCipher,
		writer:     writer,
		done:       done,
	}
}

// Write writes len(data) bytes from data to the underlying data stream.
//
// See io.Writer for more details.
func (upload *Upload) Write(data []byte) (n int, err error) {
	if upload.closed {
		return 0, Error.New("already closed")
	}

	return upload.writer.Write(data)
}

// Close closes the stream and releases the underlying resources.
func (upload *Upload) Close() error {
	if upload.closed {
		return Error.New("already closed")
	}

	upload.closed = true

	err := upload.writer.Close()

	// Wait for streams.Put to commit the upload to the PointerDB
	<-upload.done

	return err
}
