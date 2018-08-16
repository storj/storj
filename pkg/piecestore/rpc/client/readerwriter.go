// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"log"

	"github.com/gogo/protobuf/proto"
	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// StreamWriter creates a StreamWriter for writing data to the piece store server
type StreamWriter struct {
	stream       pb.PieceStoreRoutes_StoreClient
	signer       *Client // We need this for signing
	totalWritten int64
	pba          *pb.PayerBandwidthAllocation
}

// Write Piece data to a piece store server upload stream
func (s *StreamWriter) Write(b []byte) (int, error) {
	updatedAllocation := s.totalWritten + int64(len(b))
	allocationData := &pb.RenterBandwidthAllocation_Data{
		PayerAllocation: s.pba,
		Total:           updatedAllocation,
	}

	serializedAllocation, err := proto.Marshal(allocationData)
	if err != nil {
		return 0, err
	}

	sig, err := s.signer.sign(serializedAllocation)
	if err != nil {
		return 0, err
	}

	msg := &pb.PieceStore{
		Piecedata: &pb.PieceStore_PieceData{Content: b},
		Bandwidthallocation: &pb.RenterBandwidthAllocation{
			Data: serializedAllocation, Signature: sig,
		},
	}

	s.totalWritten = updatedAllocation

	// Second we send the actual content
	if err := s.stream.Send(msg); err != nil {
		return 0, fmt.Errorf("%v.Send() = %v", s.stream, err)
	}

	return len(b), nil
}

// Close the piece store Write Stream
func (s *StreamWriter) Close() error {
	reply, err := s.stream.CloseAndRecv()
	if err != nil {
		return err
	}

	log.Printf("Route summary: %v", reply)

	return nil
}

// StreamReader is a struct for reading piece download stream from server
type StreamReader struct {
	stream    pb.PieceStoreRoutes_RetrieveClient
	src       *utils.ReaderSource
	totalRead int64
}

// NewStreamReader creates a StreamReader for reading data from the piece store server
func NewStreamReader(signer *Client, stream pb.PieceStoreRoutes_RetrieveClient, pba *pb.PayerBandwidthAllocation) *StreamReader {
	sr := &StreamReader{
		stream: stream,
	}

	sr.src = utils.NewReaderSource(func() ([]byte, error) {
		updatedAllocation := int64(signer.bandwidthMsgSize) + sr.totalRead
		allocationData := &pb.RenterBandwidthAllocation_Data{
			PayerAllocation: pba,
			Total:           updatedAllocation,
		}

		serializedAllocation, err := proto.Marshal(allocationData)
		if err != nil {
			return nil, err
		}

		sig, err := signer.sign(serializedAllocation)
		if err != nil {
			return nil, err
		}

		msg := &pb.PieceRetrieval{
			Bandwidthallocation: &pb.RenterBandwidthAllocation{
				Signature: sig,
				Data:      serializedAllocation,
			},
		}

		sr.totalRead = updatedAllocation

		if err = stream.Send(msg); err != nil {
			return nil, err
		}

		resp, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return resp.GetContent(), nil
	})

	return sr
}

// Read Piece data from piece store server download stream
func (s *StreamReader) Read(b []byte) (int, error) {
	return s.src.Read(b)
}

// Close the piece store server Read Stream
func (s *StreamReader) Close() error {
	return s.stream.CloseSend()
}
