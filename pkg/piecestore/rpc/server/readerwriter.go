// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
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
	stream       pb.PieceStoreRoutes_StoreServer
	overflowData []byte
}

// Read -- Read method for piece download from stream
func (s *StreamReader) Read(b []byte) (int, error) {

	// Use overflow data if we have it
	if len(s.overflowData) > 0 {
		n := copy(b, s.overflowData)        // Copy from overflow into buffer
		s.overflowData = s.overflowData[n:] // Overflow is set to whatever remains
		return n, nil
	}

	// Receive data from server stream
	msg, err := s.stream.Recv()
	if err != nil {
		return 0, err
	}

	// Copy data into buffer
	n := copy(b, msg.Content)

	// If left over data save it into overflow variable for next read
	if n < len(msg.Content) {
		s.overflowData = b[len(b):]
	}

	return n, nil
}
