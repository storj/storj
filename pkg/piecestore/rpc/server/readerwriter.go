// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"storj.io/storj/pkg/utils"
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

// StreamReader is a struct for Retrieving data from server
type StreamReader struct {
	src *utils.ReaderSource
}

// NewStreamReader returns a new StreamReader
func NewStreamReader(stream pb.PieceStoreRoutes_StoreServer) *StreamReader {
	return &StreamReader{
		src: utils.NewReaderSource(func() ([]byte, error) {
			msg, err := stream.Recv()
			if err != nil {
				return nil, err
			}
			return msg.Content, nil
		}),
	}
}

// Read -- Read method for piece download from stream
func (s *StreamReader) Read(b []byte) (int, error) {
	return s.src.Read(b)
}
