// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"bytes"

	pb "storj.io/storj/protos/piecestore"
)

// StreamWriter -- Struct for writing piece to server upload stream
type StreamWriter struct {
	stream pb.PieceStoreRoutes_RetrieveServer
}

// Write -- Write method for piece upload to stream
func (s *StreamWriter) Write(b []byte) (int, error) {
	// Write the buffer to the stream we opened earlier
	if err := s.stream.Send(&pb.PieceRetrievalStream{Size: int64(len(b)), Content: b}); err != nil {
		return 0, err
	}

	return len(b), nil
}

// StreamReader -- Struct for Retrieving data from server
type StreamReader struct {
	stream pb.PieceStoreRoutes_StoreServer
}

// Read -- Read method for piece download from stream
func (s *StreamReader) Read(b []byte) (int, error) {
	storeMsg, err := s.stream.Recv()
	if err != nil {
		return 0, err
	}

	// Write chunk received to disk
	return bytes.NewReader(storeMsg.Content).Read(b)
}
