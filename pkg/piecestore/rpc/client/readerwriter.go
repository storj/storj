// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"log"
	"sync"

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
	max       int64
	allocated int64
	cond1     *sync.Cond
	cond2     *sync.Cond
}

// NewStreamReader creates a StreamReader for reading data from the piece store server
func NewStreamReader(signer *Client, stream pb.PieceStoreRoutes_RetrieveClient, pba *pb.PayerBandwidthAllocation, max int64) *StreamReader {
	var mtx1 sync.Mutex
	var mtx2 sync.Mutex

	sr := &StreamReader{
		stream: stream,
		max:    max,
		cond1:  sync.NewCond(&mtx1),
		cond2:  sync.NewCond(&mtx2),
	}

	go func() {
		for sr.totalRead < sr.max {
			fmt.Println("Locking ba send")
			sr.cond2.L.Lock()
			for sr.allocated-sr.totalRead >= int64(4*signer.bandwidthMsgSize) {
				sr.cond2.Wait() // Wait for the bandwidth loop to get a new bandwidth allocation
			}
			sr.cond2.L.Unlock()
			fmt.Println("Unlocking ba send")

			fmt.Println(sr.allocated, sr.totalRead, signer.bandwidthMsgSize, sr.max)

			if sr.max-sr.allocated < int64(signer.bandwidthMsgSize) {
				sr.allocated += sr.max - sr.allocated
			} else {
				sr.allocated += int64(signer.bandwidthMsgSize)
			}

			allocationData := &pb.RenterBandwidthAllocation_Data{
				PayerAllocation: pba,
				Total:           sr.allocated,
			}

			serializedAllocation, err := proto.Marshal(allocationData)
			if err != nil {
				break
			}

			sig, err := signer.sign(serializedAllocation)
			if err != nil {
				break
			}

			msg := &pb.PieceRetrieval{
				Bandwidthallocation: &pb.RenterBandwidthAllocation{
					Signature: sig,
					Data:      serializedAllocation,
				},
			}

			if err = stream.Send(msg); err != nil {
				break
			}

			if sr.allocated-sr.totalRead >= int64(4*signer.bandwidthMsgSize) || sr.max-sr.allocated < int64(4*signer.bandwidthMsgSize) {
				sr.cond1.Signal() // Signal the data send loop that it has available bandwidth
			}
		}
	}()

	sr.src = utils.NewReaderSource(func() ([]byte, error) {
		sr.cond1.L.Lock()
		fmt.Println("Locking retrieve")
		for sr.totalRead <= sr.allocated {
			sr.cond1.Wait() // Wait for the bandwidth loop to get a new bandwidth allocation
		}
		sr.cond1.L.Unlock()
		fmt.Println("Unlocking retrieve")

		resp, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		sr.totalRead += int64(len(resp.GetContent()))

		sr.cond2.Signal()

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
