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
	if len(s.overflowData) > 0 {
		n := copy(b, s.overflowData)
		s.overflowData = s.overflowData[:len(s.overflowData)-n]
		return n, nil
	}

	pieceData, err := s.stream.Recv()
	if err != nil {
		return 0, err
	}

	n := copy(b, pieceData.Content)

	if n < len(pieceData.Content) {
		s.overflowData = b[len(b):]
	}

	return n, nil
}

// Close -- Close Read Stream
func (s *StreamReader) Close() error {
	return s.stream.CloseSend()
}
