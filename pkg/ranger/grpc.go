// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"io"
	"io/ioutil"

	"storj.io/storj/examples/piecestore/rpc/client/api"
	"storj.io/storj/internal/pkg/readcloser"

	pb "storj.io/storj/examples/piecestore/rpc/protobuf"
)

type grpcRanger struct {
	client pb.RouteGuideClient
	hash   string
	size   int64
}

// GRPCRanger turns a gRPC connection into a Ranger
func GRPCRanger(client pb.RouteGuideClient, hash string) (Ranger, error) {
	shard, err := api.ShardMetaRequest(client, hash)
	if err != nil {
		return nil, err
	}
	return &grpcRanger{client, hash, shard.Size}, nil
}

// GRPCRangerSize creates a GRPCRanger with known size.
// Use it if you know the piece size. This will safe the extra request for
// retrieving the piece size from the piece storage.
func GRPCRangerSize(client pb.RouteGuideClient, hash string, size int64) Ranger {
	return &grpcRanger{client, hash, size}
}

// Size implements Ranger.Size
func (r *grpcRanger) Size() int64 {
	return r.size
}

// Range implements Ranger.Range
func (r *grpcRanger) Range(offset, length int64) io.ReadCloser {
	if offset < 0 {
		return readcloser.FatalReadCloser(Error.New("negative offset"))
	}
	if length < 0 {
		return readcloser.FatalReadCloser(Error.New("negative length"))
	}
	if offset+length > r.size {
		return readcloser.FatalReadCloser(Error.New("range beyond end"))
	}
	if length == 0 {
		return ioutil.NopCloser(bytes.NewReader([]byte{}))
	}
	reader, err := api.RetrieveShardRequest(r.client, r.hash, length, offset)
	if err != nil {
		return readcloser.FatalReadCloser(Error.Wrap(err))
	}
	return ioutil.NopCloser(reader)
}
