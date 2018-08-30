// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"log"
	"sync"
	"time"

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
	stream        pb.PieceStoreRoutes_RetrieveClient
	src           *utils.ReaderSource
	totalRead     int64
	max           int64
	allocated     int64
	dataSendCond  *sync.Cond
	allocSendCond *sync.Cond
	c             chan int
}

// NewStreamReader creates a StreamReader for reading data from the piece store server
func NewStreamReader(signer *Client, stream pb.PieceStoreRoutes_RetrieveClient, pba *pb.PayerBandwidthAllocation, max int64) *StreamReader {
	var mtx sync.Mutex

	sr := &StreamReader{
		stream:        stream,
		max:           max,
		dataSendCond:  sync.NewCond(&mtx),
		allocSendCond: sync.NewCond(&mtx),
		c:             make(chan int),
	}

	go func() {
		<-sr.c
		for sr.totalRead < sr.max {
			sr.allocSendCond.L.Lock()
			fmt.Println("Locking ba send")
			for sr.allocated-sr.totalRead >= int64(4*signer.bandwidthMsgSize) {
				sr.allocSendCond.Wait() // Wait for the bandwidth loop to get a new bandwidth allocation
			}
			fmt.Println("Unlocking ba send")
			sr.allocSendCond.L.Unlock()

			fmt.Println(sr.allocated, sr.totalRead)

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
				fmt.Println("Sending Signal to dataSendCond")
				time.Sleep(time.Second * 10)
				sr.dataSendCond.Signal() // Signal the data send loop that it has available bandwidth
				fmt.Println("Sent Signal to dataSendCond")
			}
		}
	}()

	sr.src = utils.NewReaderSource(func() ([]byte, error) {
		sr.dataSendCond.L.Lock()
		fmt.Println("Locking data send")
		for sr.totalRead <= sr.allocated {
			sr.c <- 1
			fmt.Println("dataSendCond Wait")
			sr.dataSendCond.Wait() // Wait for the bandwidth loop to send a new bandwidth allocation
		}
		fmt.Println("Unlocking data send")
		sr.dataSendCond.L.Unlock()

		resp, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		sr.totalRead += int64(len(resp.GetContent()))

		sr.allocSendCond.Signal()

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
