// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"log"

	pb "storj.io/storj/protos/piecestore"
)

// StreamWriter -- Struct for writing piece to server upload stream
type StreamWriter struct {
	stream pb.PieceStoreRoutes_StoreClient
}

// Write -- Write method for piece upload to stream
func (s *StreamWriter) Write(b []byte) (int, error) {
	if err := s.stream.Send(&pb.PieceStore{Content: b}); err != nil {
		return 0, fmt.Errorf("%v.Send() = %v", s.stream, err)
	}

	return len(b), nil
}

// Close -- Close Write Stream
func (s *StreamWriter) Close() error {
	reply, err := s.stream.CloseAndRecv()
	if err != nil {
		return err
	}

	log.Printf("Route summary: %v", reply)

	return nil
}

// StreamReader -- Struct for reading piece download stream from server
type StreamReader struct {
	stream       pb.PieceStoreRoutes_RetrieveClient
	overflowData []byte
}

// Read -- Read method for piece download stream
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

// Close -- Close Read Stream
func (s *StreamReader) Close() error {
	return s.stream.CloseSend()
}
