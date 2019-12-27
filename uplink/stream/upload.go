// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stream

import (
	"context"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/uplink/metainfo/kvmetainfo"
	"storj.io/storj/uplink/storage/streams"
)

// Upload implements Writer and Closer for writing to stream.
type Upload struct {
	ctx      context.Context
	stream   kvmetainfo.MutableStream
	streams  streams.Store
	writer   io.WriteCloser
	closed   bool
	errgroup errgroup.Group
}

// NewUpload creates new stream upload.
func NewUpload(ctx context.Context, stream kvmetainfo.MutableStream, streams streams.Store) *Upload {
	reader, writer := io.Pipe()

	upload := Upload{
		ctx:     ctx,
		stream:  stream,
		streams: streams,
		writer:  writer,
	}

	upload.errgroup.Go(func() error {
		obj := stream.Info()

		serMetaInfo := pb.SerializableMeta{
			ContentType: obj.ContentType,
			UserDefined: obj.Metadata,
		}
		metadata, err := proto.Marshal(&serMetaInfo)
		if err != nil {
			return errs.Combine(err, reader.CloseWithError(err))
		}

		_, err = streams.Put(ctx, storj.JoinPaths(obj.Bucket.Name, obj.Path), obj.Bucket.PathCipher, reader, metadata, obj.Expires)
		if err != nil {
			return errs.Combine(err, reader.CloseWithError(err))
		}

		return nil
	})

	return &upload
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
	return errs.Combine(err, upload.errgroup.Wait())
}
