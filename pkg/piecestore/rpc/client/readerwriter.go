// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"log"

	pb "storj.io/storj/protos/piecestore"
)

// PieceStreamWriter -- Struct for writing piece to server upload stream
type PieceStreamWriter struct {
	stream pb.PieceStoreRoutes_StoreClient
}

// Write -- Write method for piece upload to stream
func (s *PieceStreamWriter) Write(b []byte) (int, error) {
	if err := s.stream.Send(&pb.PieceStore{Content: b}); err != nil {
		return 0, fmt.Errorf("%v.Send() = %v", s.stream, err)
	}

	return len(b), nil
}

// Close -- Close Write Stream
func (s *PieceStreamWriter) Close() error {
	reply, err := s.stream.CloseAndRecv()
	if err != nil {
		return err
	}

	log.Printf("Route summary: %v", reply)

	return nil
}

// PieceStreamReader -- Struct for reading piece download stream from server
type PieceStreamReader struct {
	stream       pb.PieceStoreRoutes_RetrieveClient
	overflowData []byte
}

// Read -- Read method for piece download stream
func (s *PieceStreamReader) Read(b []byte) (int, error) {
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

	if cap(b) < len(pieceData.Content) {
		s.overflowData = b[cap(b):]
	}

	return n, nil
}

// Close -- Close Read Stream
func (s *PieceStreamReader) Close() error {
	return s.stream.CloseSend()
}
