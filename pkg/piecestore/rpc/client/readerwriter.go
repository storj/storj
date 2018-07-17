// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"log"

	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// StreamWriter creates a StreamWriter for writing data to the piece store server
type StreamWriter struct {
	stream pb.PieceStoreRoutes_StoreClient
	signer *Client // We need this for signing
}

// Write Piece data to a piece store server upload stream
func (s *StreamWriter) Write(b []byte) (int, error) {

	// TODO: What does the message look like?
	sig, err := s.signer.sign([]byte{'m', 's', 'g'})
	if err != nil {
		return 0, err
	}

	bandwidthAllocationMSG := &pb.PieceStore{
		Bandwidthallocation: &pb.BandwidthAllocation{
			Data: &pb.BandwidthAllocation_Data{
				Payer: s.signer.payerID, Renter: s.signer.renterID, Size: int64(len(b)),
			},
			Signature: sig,
		},
	}

	// First we send the bandwidth allocation
	if err := s.stream.Send(bandwidthAllocationMSG); err != nil {
		return 0, fmt.Errorf("%v.Send() = %v", s.stream, err)
	}

	contentMSG := &pb.PieceStore{Piecedata: &pb.PieceStore_PieceData{Content: b}}

	// Second we send the actual content
	if err := s.stream.Send(contentMSG); err != nil {
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
	stream pb.PieceStoreRoutes_RetrieveClient
	src    *utils.ReaderSource
}

// NewStreamReader creates a StreamReader for reading data from the piece store server
func (signer *Client) NewStreamReader(stream pb.PieceStoreRoutes_RetrieveClient) *StreamReader {
	return &StreamReader{
		stream: stream,
		src: utils.NewReaderSource(func() ([]byte, error) {

			// TODO: What does the message look like?
			sig, err := signer.sign([]byte{'m', 's', 'g'})
			if err != nil {
				return nil, err
			}

			msg := &pb.PieceRetrieval{
				Bandwidthallocation: &pb.BandwidthAllocation{
					Signature: sig,
					Data: &pb.BandwidthAllocation_Data{
						Payer: signer.payerID, Renter: signer.renterID, Size: 32 * 1024,
					},
				},
			}

			if err = stream.Send(msg); err != nil {
				return nil, err
			}

			resp, err := stream.Recv()
			if err != nil {
				return nil, err
			}
			return resp.Content, nil
		}),
	}
}

// Read Piece data from piece store server download stream
func (s *StreamReader) Read(b []byte) (int, error) {
	return s.src.Read(b)
}

// Close the piece store server Read Stream
func (s *StreamReader) Close() error {
	return s.stream.CloseSend()
}
